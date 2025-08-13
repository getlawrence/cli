package dependency

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/logger"
)

// JavaInjector implements DependencyHandler for Java projects (Maven/Gradle)
type JavaInjector struct {
	logger logger.Logger
}

// NewJavaInjector creates a new Java dependency handler
func NewJavaInjector(logger logger.Logger) *JavaInjector {
	return &JavaInjector{logger: logger}
}

// GetLanguage returns the language this handler supports
func (h *JavaInjector) GetLanguage() string { return "java" }

// AddDependencies adds the specified dependencies to the Java project
// Best-effort implementation:
// - Maven: use `mvn dependency:get` to fetch artifacts (won't modify pom.xml)
// - Gradle: run `gradle dependencies` as a no-op to trigger resolution (won't modify build files)
// For now, we print clear next steps and do not auto-edit build files.
func (h *JavaInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	// Resolve and pin versions using Maven Central where possible
	resolved, rerr := h.resolveLatestVersions(ctx, projectPath, dependencies)
	if rerr != nil {
		return rerr
	}

	hasPom := h.hasPom(projectPath)
	hasGradle := h.hasGradle(projectPath)

	if !hasPom && !hasGradle {
		return fmt.Errorf("no Maven or Gradle build file found in %s", projectPath)
	}

	if dryRun {
		h.logger.Logf("Java dependencies required (add to pom.xml or build.gradle):\n")
		for _, dep := range resolved {
			h.logger.Logf("  - %s\n", h.formatCoordinate(dep))
		}
		return nil
	}

	// Best effort: try to fetch with mvn if available and pom exists
	if hasPom && commandExists("mvn") {
		for _, dep := range resolved {
			coord := h.formatCoordinate(dep)
			args := []string{"dependency:get", fmt.Sprintf("-Dartifact=%s", coord)}
			cmd := exec.CommandContext(ctx, "mvn", args...)
			cmd.Dir = projectPath
			if out, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("mvn dependency:get failed for %s: %w\nOutput: %s", coord, err, string(out))
			}
		}
		return nil
	}

	// If Gradle project, just inform user to add to build file
	if hasGradle {
		h.logger.Log("Please add the following dependencies to your Gradle build file (dependencies block):")
		for _, dep := range dependencies {
			h.logger.Logf("  implementation '%s'\n", h.formatCoordinate(dep))
		}
		return nil
	}

	return nil
}

// GetCoreDependencies returns the core OpenTelemetry dependencies for Java
func (h *JavaInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry API", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-api", Category: "core", Required: true},
		{Name: "OpenTelemetry SDK", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-sdk", Category: "core", Required: true},
		{Name: "OTLP Exporter", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-exporter-otlp", Category: "exporter", Required: true},
	}
}

// GetInstrumentationDependency returns the dependency for a specific instrumentation
func (h *JavaInjector) GetInstrumentationDependency(instrumentation string) *Dependency {
	m := map[string]Dependency{
		"http":   {Name: "HTTP Instrumentation", Language: "java", ImportPath: "io.opentelemetry.instrumentation:opentelemetry-instrumentation-servlet", Category: "instrumentation"},
		"spring": {Name: "Spring Boot Starter", Language: "java", ImportPath: "io.opentelemetry.instrumentation:opentelemetry-spring-boot-starter", Category: "instrumentation"},
		"grpc":   {Name: "gRPC Instrumentation", Language: "java", ImportPath: "io.opentelemetry.instrumentation:opentelemetry-grpc-1.6", Category: "instrumentation"},
		"jdbc":   {Name: "JDBC Instrumentation", Language: "java", ImportPath: "io.opentelemetry.instrumentation:opentelemetry-jdbc", Category: "instrumentation"},
	}
	if dep, ok := m[instrumentation]; ok {
		return &dep
	}
	return nil
}

// GetComponentDependency returns exporter/propagator components if needed
func (h *JavaInjector) GetComponentDependency(componentType, component string) *Dependency {
	components := map[string]map[string]Dependency{
		"exporter": {
			"jaeger": {Name: "Jaeger Exporter", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-exporter-jaeger", Category: "exporter"},
			"zipkin": {Name: "Zipkin Exporter", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-exporter-zipkin", Category: "exporter"},
		},
		"propagator": {
			"b3": {Name: "B3 Propagator", Language: "java", ImportPath: "io.opentelemetry:opentelemetry-extension-trace-propagators", Category: "propagator"},
		},
	}
	if m, ok := components[componentType]; ok {
		if dep, ok := m[component]; ok {
			return &dep
		}
	}
	return nil
}

// ResolveInstrumentationPrerequisites for Java currently returns the list unchanged.
func (h *JavaInjector) ResolveInstrumentationPrerequisites(instrumentations []string) []string {
	return instrumentations
}

// ValidateProjectStructure checks if the project has required dependency management files
func (h *JavaInjector) ValidateProjectStructure(projectPath string) error {
	if !h.hasPom(projectPath) && !h.hasGradle(projectPath) {
		return fmt.Errorf("no pom.xml or build.gradle found in %s", projectPath)
	}
	return nil
}

// GetDependencyFiles returns the paths to dependency management files
func (h *JavaInjector) GetDependencyFiles(projectPath string) []string {
	files := []string{}
	if h.hasPom(projectPath) {
		files = append(files, filepath.Join(projectPath, "pom.xml"))
	}
	if h.hasGradle(projectPath) {
		files = append(files, filepath.Join(projectPath, "build.gradle"))
	}
	return files
}

// Helpers
func (h *JavaInjector) hasPom(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "pom.xml"))
	return err == nil
}
func (h *JavaInjector) hasGradle(projectPath string) bool {
	if _, err := os.Stat(filepath.Join(projectPath, "build.gradle")); err == nil {
		return true
	}
	if _, err := os.Stat(filepath.Join(projectPath, "build.gradle.kts")); err == nil {
		return true
	}
	return false
}

func (h *JavaInjector) formatCoordinate(dep Dependency) string {
	// If ImportPath already contains group:artifact[:version], use it; otherwise derive from Name
	coord := dep.ImportPath
	if !strings.Contains(coord, ":") && strings.Contains(dep.Name, ":") {
		coord = dep.Name
	}
	if dep.Version != "" && !hasVersionSuffix(coord) {
		coord = coord + ":" + dep.Version
	}
	return coord
}

// resolveLatestVersions attempts to find latest release version for Maven coordinates missing version
func (h *JavaInjector) resolveLatestVersions(ctx context.Context, projectPath string, deps []Dependency) ([]Dependency, error) {
	resolved := make([]Dependency, 0, len(deps))
	for _, d := range deps {
		if strings.TrimSpace(d.Version) != "" || hasVersionSuffix(d.ImportPath) || strings.Count(d.Name, ":") >= 2 {
			resolved = append(resolved, d)
			continue
		}
		coord := d.ImportPath
		if !strings.Contains(coord, ":") && strings.Contains(d.Name, ":") {
			coord = d.Name
		}
		// Query Maven using mvn help:evaluate -Dexpression=project.version with temporary pom
		// Simpler: use `mvn -q -Dexec.executable=echo -Dexec.args='${project.version}' --non-recursive` requires a project.
		// Fallback: try `mvn dependency:get -Dartifact=group:artifact:LATEST -Dtransitive=false` which resolves LATEST.
		if commandExists("mvn") {
			args := []string{"dependency:get", fmt.Sprintf("-Dartifact=%s:LATEST", coord), "-Dtransitive=false"}
			cmd := exec.CommandContext(ctx, "mvn", args...)
			cmd.Dir = projectPath
			if _, err := cmd.CombinedOutput(); err == nil {
				d.Version = "LATEST"
				resolved = append(resolved, d)
				continue
			}
		}
		// As a last resort, leave unresolved and error
		return nil, fmt.Errorf("failed to resolve latest version for %s", coord)
	}
	return resolved, nil
}

func hasVersionSuffix(coord string) bool {
	// naive: group:artifact:version (two colons)
	return strings.Count(coord, ":") >= 2
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

package dependency

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/logger"
)

// GoInjector implements DependencyHandler for Go projects
type GoInjector struct {
	logger logger.Logger
}

// NewGoInjector creates a new Go dependency handler
func NewGoInjector(logger logger.Logger) *GoInjector {
	return &GoInjector{logger: logger}
}

// GetLanguage returns the language this handler supports
func (h *GoInjector) GetLanguage() string {
	return "go"
}

// AddDependencies adds the specified dependencies to the Go project
func (h *GoInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	goModPath := filepath.Join(projectPath, "go.mod")

	// Check if go.mod exists
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in %s", projectPath)
	}

	// Filter dependencies that are not already present
	neededDeps, err := h.filterExistingDependencies(goModPath, dependencies)
	if err != nil {
		return fmt.Errorf("failed to check existing dependencies: %w", err)
	}

	if len(neededDeps) == 0 {
		h.logger.Log("All required dependencies are already present")
		return nil
	}

	if dryRun {
		h.logger.Logf("Would add the following Go dependencies to %s:\n", goModPath)
		for _, dep := range neededDeps {
			if dep.Version != "" {
				h.logger.Logf("  - %s@%s\n", dep.ImportPath, dep.Version)
			} else {
				h.logger.Logf("  - %s\n", dep.ImportPath)
			}
		}
		return nil
	}

	// Resolve explicit versions for all dependencies (use latest if missing)
	resolved, rerr := h.resolveLatestVersions(ctx, projectPath, neededDeps)
	if rerr != nil {
		return rerr
	}
	// Add dependencies using go get
	return h.addDependenciesWithGoGet(ctx, projectPath, resolved)
}

// GetCoreDependencies returns the core OpenTelemetry dependencies for Go
func (h *GoInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{
			Name:        "OpenTelemetry API",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel",
			Category:    "core",
			Description: "OpenTelemetry API for Go",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry SDK",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/sdk",
			Category:    "core",
			Description: "OpenTelemetry SDK for Go",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry Trace",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/trace",
			Category:    "core",
			Description: "OpenTelemetry tracing API",
			Required:    true,
		},
		{
			Name:        "OTLP HTTP Exporter",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
			Category:    "exporter",
			Description: "OTLP HTTP trace exporter",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry Resource",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/sdk/resource",
			Category:    "core",
			Description: "OpenTelemetry resource SDK",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry Propagation",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/propagation",
			Category:    "core",
			Description: "OpenTelemetry context propagation",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry Semantic Conventions",
			Version:     "",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/otel/semconv/v1.34.0",
			Category:    "core",
			Description: "OpenTelemetry semantic conventions",
			Required:    true,
		},
	}
}

// GetInstrumentationDependency returns the dependency for a specific instrumentation
func (h *GoInjector) GetInstrumentationDependency(instrumentation string) *Dependency {
	instrumentations := map[string]Dependency{
		"otelhttp": {
			Name:        "HTTP Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
			Category:    "instrumentation",
			Description: "HTTP client and server instrumentation",
		},
		"otelgin": {
			Name:        "Gin Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin",
			Category:    "instrumentation",
			Description: "Gin web framework instrumentation",
		},
		"otelmux": {
			Name:        "Gorilla Mux Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux",
			Category:    "instrumentation",
			Description: "Gorilla Mux router instrumentation",
		},
		"otelsql": {
			Name:        "SQL Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/database/sql/otelsql",
			Category:    "instrumentation",
			Description: "SQL database instrumentation",
		},
		"otelgrpc": {
			Name:        "gRPC Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc",
			Category:    "instrumentation",
			Description: "gRPC client and server instrumentation",
		},
		"otelecho": {
			Name:        "Echo Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/github.com/labstack/echo/otelecho",
			Category:    "instrumentation",
			Description: "Echo web framework instrumentation",
		},
		"otelfiber": {
			Name:        "Fiber Instrumentation",
			Language:    "go",
			ImportPath:  "go.opentelemetry.io/contrib/instrumentation/github.com/gofiber/fiber/otelfiber",
			Category:    "instrumentation",
			Description: "Fiber web framework instrumentation",
		},
	}

	if dep, exists := instrumentations[instrumentation]; exists {
		return &dep
	}
	return nil
}

// GetComponentDependency returns the dependency for a specific component
func (h *GoInjector) GetComponentDependency(componentType, component string) *Dependency {
	components := map[string]map[string]Dependency{
		"exporter": {
			"jaeger": {
				Name:        "Jaeger Exporter",
				Language:    "go",
				ImportPath:  "go.opentelemetry.io/otel/exporters/jaeger",
				Category:    "exporter",
				Description: "Jaeger trace exporter",
			},
			"zipkin": {
				Name:        "Zipkin Exporter",
				Language:    "go",
				ImportPath:  "go.opentelemetry.io/otel/exporters/zipkin",
				Category:    "exporter",
				Description: "Zipkin trace exporter",
			},
			"prometheus": {
				Name:        "Prometheus Exporter",
				Language:    "go",
				ImportPath:  "go.opentelemetry.io/otel/exporters/prometheus",
				Category:    "exporter",
				Description: "Prometheus metrics exporter",
			},
		},
		"sampler": {
			"probabilistic": {
				Name:        "Probabilistic Sampler",
				Language:    "go",
				ImportPath:  "go.opentelemetry.io/otel/sdk/trace",
				Category:    "sampler",
				Description: "Probabilistic trace sampler (included in SDK)",
			},
		},
	}

	if typeComponents, exists := components[componentType]; exists {
		if dep, exists := typeComponents[component]; exists {
			return &dep
		}
	}
	return nil
}

// ResolveInstrumentationPrerequisites for Go currently returns the list unchanged.
func (h *GoInjector) ResolveInstrumentationPrerequisites(instrumentations []string) []string {
	return instrumentations
}

// ValidateProjectStructure checks if the project has required dependency management files
func (h *GoInjector) ValidateProjectStructure(projectPath string) error {
	goModPath := filepath.Join(projectPath, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in %s", projectPath)
	}
	return nil
}

// GetDependencyFiles returns the paths to dependency management files
func (h *GoInjector) GetDependencyFiles(projectPath string) []string {
	return []string{
		filepath.Join(projectPath, "go.mod"),
		filepath.Join(projectPath, "go.sum"),
	}
}

// filterExistingDependencies filters out dependencies that already exist in go.mod
func (h *GoInjector) filterExistingDependencies(goModPath string, dependencies []Dependency) ([]Dependency, error) {
	existingDeps, err := h.getExistingDependencies(goModPath)
	if err != nil {
		return nil, err
	}

	var neededDeps []Dependency
	for _, dep := range dependencies {
		if !existingDeps[dep.ImportPath] {
			neededDeps = append(neededDeps, dep)
		}
	}

	return neededDeps, nil
}

// getExistingDependencies reads the go.mod file and returns existing dependencies
func (h *GoInjector) getExistingDependencies(goModPath string) (map[string]bool, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	dependencies := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	// Regex for matching dependencies
	depRegex := regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)`)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle require blocks
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Handle single require line
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
		}

		if inRequireBlock || strings.HasPrefix(scanner.Text(), "require ") {
			matches := depRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				dependencies[matches[1]] = true
			}
		}
	}

	return dependencies, scanner.Err()
}

// addDependenciesWithGoGet adds dependencies using the go get command
func (h *GoInjector) addDependenciesWithGoGet(ctx context.Context, projectPath string, dependencies []Dependency) error {
	h.logger.Logf("Adding %d dependencies to Go project...\n", len(dependencies))

	for _, dep := range dependencies {
		h.logger.Logf("  Adding %s...\n", dep.ImportPath)

		args := []string{"get"}
		if strings.TrimSpace(dep.Version) != "" {
			args = append(args, dep.ImportPath+"@"+dep.Version)
		} else {
			args = append(args, dep.ImportPath)
		}

		cmd := exec.CommandContext(ctx, "go", args...)
		cmd.Dir = projectPath

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to add dependency %s: %w\nOutput: %s", dep.ImportPath, err, string(output))
		}
	}

	h.logger.Logf("Successfully added %d dependencies\n", len(dependencies))
	return nil
}

// resolveLatestVersions queries `go list -m -versions -json` to find latest tag when version missing
func (h *GoInjector) resolveLatestVersions(ctx context.Context, projectPath string, deps []Dependency) ([]Dependency, error) {
	resolved := make([]Dependency, 0, len(deps))
	for _, d := range deps {
		if strings.TrimSpace(d.Version) != "" {
			resolved = append(resolved, d)
			continue
		}
		// If the import path encodes a version in its path (e.g., semconv/v1.34.0), skip resolution
		if hasPathEncodedVersion(d.ImportPath) {
			resolved = append(resolved, d)
			continue
		}
		// Use go list to fetch module info
		cmd := exec.CommandContext(ctx, "go", "list", "-m", "-versions", "-json", d.ImportPath)
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve latest version for %s: %w\nOutput: %s", d.ImportPath, err, string(out))
		}
		// Simple parse: find last occurrence of "Versions":["v1.0.0","v1.2.3"] and take last
		text := string(out)
		start := strings.Index(text, "\"Versions\":")
		latest := ""
		if start != -1 {
			// find brackets
			lb := strings.Index(text[start:], "[")
			rb := strings.Index(text[start+lb+1:], "]")
			if lb != -1 && rb != -1 {
				list := text[start+lb+1 : start+lb+1+rb]
				parts := strings.Split(list, ",")
				if len(parts) > 0 {
					last := strings.TrimSpace(parts[len(parts)-1])
					last = strings.Trim(last, "\" ")
					latest = last
				}
			}
		}
		if latest == "" {
			// Fallback to @latest directive
			latest = "latest"
		}
		d.Version = latest
		resolved = append(resolved, d)
	}
	return resolved, nil
}

// hasPathEncodedVersion detects if the module path encodes a version in the last segment (e.g., "/v1.34.0")
func hasPathEncodedVersion(importPath string) bool {
	// Find last segment
	lastSlash := strings.LastIndex(importPath, "/")
	seg := importPath
	if lastSlash != -1 {
		seg = importPath[lastSlash+1:]
	}
	// Match vMAJOR.MINOR.PATCH
	matched, _ := regexp.MatchString(`^v\d+\.\d+\.\d+$`, seg)
	return matched
}

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

// JavaScriptInjector implements DependencyHandler for JavaScript/Node.js projects
type JavaScriptInjector struct {
	logger logger.Logger
}

// NewJavaScriptInjector creates a new JS dependency handler
func NewJavaScriptInjector(logger logger.Logger) *JavaScriptInjector {
	return &JavaScriptInjector{logger: logger}
}

// GetLanguage returns the language this handler supports
func (h *JavaScriptInjector) GetLanguage() string { return "javascript" }

// AddDependencies installs dependencies with npm (fallback yarn/pnpm not yet implemented)
func (h *JavaScriptInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	// Ensure package.json exists
	pkgJSON := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgJSON); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found in %s", projectPath)
	}

	if len(dependencies) == 0 {
		return nil
	}

	// Build install args
	var args []string
	args = append(args, "install")
	for _, dep := range dependencies {
		spec := dep.ImportPath
		if dep.Version != "" {
			spec = fmt.Sprintf("%s@%s", dep.ImportPath, dep.Version)
		}
		args = append(args, spec)
	}

	if dryRun {
		h.logger.Logf("Would run: npm %v in %s\n", args, projectPath)
		return nil
	}

	cmd := exec.CommandContext(ctx, "npm", args...)
	cmd.Dir = projectPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install dependencies with npm: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// GetCoreDependencies returns the core OTEL deps for JS
func (h *JavaScriptInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry API", Language: "javascript", ImportPath: "@opentelemetry/api", Category: "core", Required: true},
		{Name: "OpenTelemetry SDK (Node)", Language: "javascript", ImportPath: "@opentelemetry/sdk-node", Category: "core", Required: true},
		{Name: "OTLP HTTP Trace Exporter", Language: "javascript", ImportPath: "@opentelemetry/exporter-trace-otlp-http", Category: "exporter", Required: true},
	}
}

// GetInstrumentationDependency returns a specific instrumentation package
func (h *JavaScriptInjector) GetInstrumentationDependency(instrumentation string) *Dependency {
	// Map known instrumentation names to @opentelemetry/instrumentation-* packages
	m := map[string]Dependency{
		"http":    {Name: "HTTP Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-http", Category: "instrumentation"},
		"express": {Name: "Express Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-express", Category: "instrumentation"},
		"koa":     {Name: "Koa Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-koa", Category: "instrumentation"},
		"mysql":   {Name: "MySQL Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-mysql", Category: "instrumentation"},
		"pg":      {Name: "Postgres Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-pg", Category: "instrumentation"},
		"mongodb": {Name: "MongoDB Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-mongodb", Category: "instrumentation"},
		"redis":   {Name: "Redis Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-redis", Category: "instrumentation"},
	}
	if dep, ok := m[instrumentation]; ok {
		return &dep
	}
	return nil
}

// GetComponentDependency returns exporter/propagator components if needed
func (h *JavaScriptInjector) GetComponentDependency(componentType, component string) *Dependency {
	return nil
}

// ResolveInstrumentationPrerequisites expands JS instrumentation list with required prerequisites
// For example, framework instrumentations depend on HTTP being instrumented.
func (h *JavaScriptInjector) ResolveInstrumentationPrerequisites(instrumentations []string) []string {
	if len(instrumentations) == 0 {
		return instrumentations
	}
	seen := make(map[string]bool)
	for _, inst := range instrumentations {
		seen[strings.ToLower(inst)] = true
	}
	// Add http if express/koa present
	if (seen["express"] || seen["koa"]) && !seen["http"] {
		instrumentations = append(instrumentations, "http")
	}
	return instrumentations
}

// ValidateProjectStructure checks dependency files
func (h *JavaScriptInjector) ValidateProjectStructure(projectPath string) error {
	// Best-effort: ensure package.json exists
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err != nil {
		return fmt.Errorf("package.json not found in %s", projectPath)
	}
	return nil
}

// GetDependencyFiles returns dependency file paths
func (h *JavaScriptInjector) GetDependencyFiles(projectPath string) []string {
	return []string{filepath.Join(projectPath, "package.json"), filepath.Join(projectPath, "package-lock.json")}
}

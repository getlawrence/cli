package dependency

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// JavaScriptHandler implements DependencyHandler for JavaScript/Node.js projects
type JavaScriptHandler struct{}

// NewJavaScriptHandler creates a new JS dependency handler
func NewJavaScriptHandler() *JavaScriptHandler { return &JavaScriptHandler{} }

// GetLanguage returns the language this handler supports
func (h *JavaScriptHandler) GetLanguage() string { return "javascript" }

// AddDependencies installs dependencies with npm (fallback yarn/pnpm not yet implemented)
func (h *JavaScriptHandler) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
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
		fmt.Printf("Would run: npm %v in %s\n", args, projectPath)
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
func (h *JavaScriptHandler) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry API", Language: "javascript", ImportPath: "@opentelemetry/api", Category: "core", Required: true},
		{Name: "OpenTelemetry SDK (Node)", Language: "javascript", ImportPath: "@opentelemetry/sdk-node", Category: "core", Required: true},
		{Name: "OTLP HTTP Trace Exporter", Language: "javascript", ImportPath: "@opentelemetry/exporter-trace-otlp-http", Category: "exporter", Required: true},
	}
}

// GetInstrumentationDependency returns a specific instrumentation package
func (h *JavaScriptHandler) GetInstrumentationDependency(instrumentation string) *Dependency {
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
func (h *JavaScriptHandler) GetComponentDependency(componentType, component string) *Dependency {
	return nil
}

// ValidateProjectStructure checks dependency files
func (h *JavaScriptHandler) ValidateProjectStructure(projectPath string) error {
	// Best-effort: ensure package.json exists
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err != nil {
		return fmt.Errorf("package.json not found in %s", projectPath)
	}
	return nil
}

// GetDependencyFiles returns dependency file paths
func (h *JavaScriptHandler) GetDependencyFiles(projectPath string) []string {
	return []string{filepath.Join(projectPath, "package.json"), filepath.Join(projectPath, "package-lock.json")}
}

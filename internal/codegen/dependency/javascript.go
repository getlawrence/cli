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
		// Auto instrumentation bundle from opentelemetry-js-contrib
		"auto":        {Name: "Node Auto Instrumentations", Language: "javascript", ImportPath: "@opentelemetry/auto-instrumentations-node", Category: "instrumentation"},
		"http":        {Name: "HTTP Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-http", Category: "instrumentation"},
		"express":     {Name: "Express Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-express", Category: "instrumentation"},
		"fastify":     {Name: "Fastify Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-fastify", Category: "instrumentation"},
		"hapi":        {Name: "Hapi Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-hapi", Category: "instrumentation"},
		"restify":     {Name: "Restify Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-restify", Category: "instrumentation"},
		"nestjs-core": {Name: "NestJS Core Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-nestjs-core", Category: "instrumentation"},
		"next":        {Name: "Next.js Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-next", Category: "instrumentation"},
		"socket.io":   {Name: "Socket.io Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-socket.io", Category: "instrumentation"},
		"koa":         {Name: "Koa Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-koa", Category: "instrumentation"},
		"mysql":       {Name: "MySQL Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-mysql", Category: "instrumentation"},
		"mysql2":      {Name: "MySQL2 Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-mysql2", Category: "instrumentation"},
		"pg":          {Name: "Postgres Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-pg", Category: "instrumentation"},
		"mongodb":     {Name: "MongoDB Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-mongodb", Category: "instrumentation"},
		"redis":       {Name: "Redis Instrumentation", Language: "javascript", ImportPath: "@opentelemetry/instrumentation-redis", Category: "instrumentation"},
	}
	if dep, ok := m[instrumentation]; ok {
		return &dep
	}
	return nil
}

// GetComponentDependency returns exporter/propagator components if needed
func (h *JavaScriptInjector) GetComponentDependency(componentType, component string) *Dependency {
	switch componentType {
	case "propagator":
		switch strings.ToLower(component) {
		case "b3", "b3multi":
			return &Dependency{Name: "B3 Propagator", Language: "javascript", ImportPath: "@opentelemetry/propagator-b3", Category: "propagator"}
		}
	case "exporter":
		switch strings.ToLower(component) {
		case "otlphttp", "otlp":
			return &Dependency{Name: "OTLP HTTP Trace Exporter", Language: "javascript", ImportPath: "@opentelemetry/exporter-trace-otlp-http", Category: "exporter"}
		case "otlpgrpc":
			return &Dependency{Name: "OTLP gRPC Trace Exporter", Language: "javascript", ImportPath: "@opentelemetry/exporter-trace-otlp-grpc", Category: "exporter"}
		}
	}
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
	// If using the auto bundle, prerequisites are handled internally
	if seen["auto"] {
		return instrumentations
	}

	// Add http if any web framework instrumentation is present
	if (seen["express"] || seen["koa"] || seen["fastify"] || seen["hapi"] || seen["restify"] || seen["nestjs-core"] || seen["next"] || seen["socket.io"]) && !seen["http"] {
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

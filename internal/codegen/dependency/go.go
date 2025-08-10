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
)

// GoInjector implements DependencyHandler for Go projects
type GoInjector struct{}

// NewGoInjector creates a new Go dependency handler
func NewGoInjector() *GoInjector {
	return &GoInjector{}
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
		fmt.Println("All required dependencies are already present")
		return nil
	}

	if dryRun {
		fmt.Printf("Would add the following Go dependencies to %s:\n", goModPath)
		for _, dep := range neededDeps {
			if dep.Version != "" {
				fmt.Printf("  - %s@%s\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - %s\n", dep.ImportPath)
			}
		}
		return nil
	}

	// Add dependencies using go get
	return h.addDependenciesWithGoGet(ctx, projectPath, neededDeps)
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
	fmt.Printf("Adding %d dependencies to Go project...\n", len(dependencies))

	for _, dep := range dependencies {
		fmt.Printf("  Adding %s...\n", dep.ImportPath)

		args := []string{"get"}
		if dep.Version != "" {
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

	fmt.Printf("Successfully added %d dependencies\n", len(dependencies))
	return nil
}

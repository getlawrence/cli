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

// PythonHandler implements DependencyHandler for Python projects
type PythonHandler struct{}

// NewPythonHandler creates a new Python dependency handler
func NewPythonHandler() *PythonHandler {
	return &PythonHandler{}
}

// GetLanguage returns the language this handler supports
func (h *PythonHandler) GetLanguage() string {
	return "python"
}

// AddDependencies adds the specified dependencies to the Python project
func (h *PythonHandler) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	// Try different dependency management approaches
	if h.hasRequirementsTxt(projectPath) {
		return h.addToRequirementsTxt(projectPath, dependencies, dryRun)
	}

	if h.hasPyprojectToml(projectPath) {
		return h.addToPyprojectToml(projectPath, dependencies, dryRun)
	}

	if h.hasSetupPy(projectPath) {
		return h.addWithPip(ctx, projectPath, dependencies, dryRun)
	}

	// Default: create requirements.txt
	return h.createRequirementsTxt(projectPath, dependencies, dryRun)
}

// GetCoreDependencies returns the core OpenTelemetry dependencies for Python
func (h *PythonHandler) GetCoreDependencies() []Dependency {
	return []Dependency{
		{
			Name:        "OpenTelemetry API",
			Version:     "",
			Language:    "python",
			ImportPath:  "opentelemetry-api",
			Category:    "core",
			Description: "OpenTelemetry API for Python",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry SDK",
			Version:     "",
			Language:    "python",
			ImportPath:  "opentelemetry-sdk",
			Category:    "core",
			Description: "OpenTelemetry SDK for Python",
			Required:    true,
		},
		{
			Name:        "OTLP Exporter",
			Version:     "",
			Language:    "python",
			ImportPath:  "opentelemetry-exporter-otlp",
			Category:    "exporter",
			Description: "OTLP exporter for traces and metrics",
			Required:    true,
		},
		{
			Name:        "OpenTelemetry Instrumentation",
			Version:     "",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation",
			Category:    "core",
			Description: "Base instrumentation package",
			Required:    true,
		},
	}
}

// GetInstrumentationDependency returns the dependency for a specific instrumentation
func (h *PythonHandler) GetInstrumentationDependency(instrumentation string) *Dependency {
	instrumentations := map[string]Dependency{
		"requests": {
			Name:        "Requests Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-requests",
			Category:    "instrumentation",
			Description: "HTTP requests library instrumentation",
		},
		"flask": {
			Name:        "Flask Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-flask",
			Category:    "instrumentation",
			Description: "Flask web framework instrumentation",
		},
		"django": {
			Name:        "Django Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-django",
			Category:    "instrumentation",
			Description: "Django web framework instrumentation",
		},
		"fastapi": {
			Name:        "FastAPI Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-fastapi",
			Category:    "instrumentation",
			Description: "FastAPI web framework instrumentation",
		},
		"sqlalchemy": {
			Name:        "SQLAlchemy Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-sqlalchemy",
			Category:    "instrumentation",
			Description: "SQLAlchemy ORM instrumentation",
		},
		"psycopg2": {
			Name:        "psycopg2 Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-psycopg2",
			Category:    "instrumentation",
			Description: "PostgreSQL psycopg2 driver instrumentation",
		},
		"pymongo": {
			Name:        "PyMongo Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-pymongo",
			Category:    "instrumentation",
			Description: "MongoDB PyMongo driver instrumentation",
		},
		"redis": {
			Name:        "Redis Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-redis",
			Category:    "instrumentation",
			Description: "Redis client instrumentation",
		},
		"celery": {
			Name:        "Celery Instrumentation",
			Language:    "python",
			ImportPath:  "opentelemetry-instrumentation-celery",
			Category:    "instrumentation",
			Description: "Celery task queue instrumentation",
		},
	}

	if dep, exists := instrumentations[instrumentation]; exists {
		return &dep
	}
	return nil
}

// GetComponentDependency returns the dependency for a specific component
func (h *PythonHandler) GetComponentDependency(componentType, component string) *Dependency {
	components := map[string]map[string]Dependency{
		"exporter": {
			"jaeger": {
				Name:        "Jaeger Exporter",
				Language:    "python",
				ImportPath:  "opentelemetry-exporter-jaeger",
				Category:    "exporter",
				Description: "Jaeger trace exporter",
			},
			"zipkin": {
				Name:        "Zipkin Exporter",
				Language:    "python",
				ImportPath:  "opentelemetry-exporter-zipkin",
				Category:    "exporter",
				Description: "Zipkin trace exporter",
			},
			"prometheus": {
				Name:        "Prometheus Exporter",
				Language:    "python",
				ImportPath:  "opentelemetry-exporter-prometheus",
				Category:    "exporter",
				Description: "Prometheus metrics exporter",
			},
		},
		"instrumentation": {
			"auto": {
				Name:        "Auto Instrumentation",
				Language:    "python",
				ImportPath:  "opentelemetry-distro[otlp]",
				Category:    "instrumentation",
				Description: "Auto-instrumentation distribution",
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
func (h *PythonHandler) ValidateProjectStructure(projectPath string) error {
	hasDepFile := h.hasRequirementsTxt(projectPath) ||
		h.hasPyprojectToml(projectPath) ||
		h.hasSetupPy(projectPath)

	if !hasDepFile {
		fmt.Printf("No Python dependency file found in %s, will create requirements.txt\n", projectPath)
	}

	return nil
}

// GetDependencyFiles returns the paths to dependency management files
func (h *PythonHandler) GetDependencyFiles(projectPath string) []string {
	files := []string{}

	if h.hasRequirementsTxt(projectPath) {
		files = append(files, filepath.Join(projectPath, "requirements.txt"))
	}

	if h.hasPyprojectToml(projectPath) {
		files = append(files, filepath.Join(projectPath, "pyproject.toml"))
	}

	if h.hasSetupPy(projectPath) {
		files = append(files, filepath.Join(projectPath, "setup.py"))
	}

	return files
}

// Helper methods for checking project structure
func (h *PythonHandler) hasRequirementsTxt(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "requirements.txt"))
	return err == nil
}

func (h *PythonHandler) hasPyprojectToml(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "pyproject.toml"))
	return err == nil
}

func (h *PythonHandler) hasSetupPy(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "setup.py"))
	return err == nil
}

// addToRequirementsTxt adds dependencies to requirements.txt
func (h *PythonHandler) addToRequirementsTxt(projectPath string, dependencies []Dependency, dryRun bool) error {
	reqPath := filepath.Join(projectPath, "requirements.txt")

	// Filter dependencies that are not already present
	neededDeps, err := h.filterExistingDependenciesFromRequirements(reqPath, dependencies)
	if err != nil {
		return fmt.Errorf("failed to check existing dependencies: %w", err)
	}

	if len(neededDeps) == 0 {
		fmt.Println("All required dependencies are already present in requirements.txt")
		return nil
	}

	if dryRun {
		fmt.Printf("Would add the following Python dependencies to %s:\n", reqPath)
		for _, dep := range neededDeps {
			if dep.Version != "" {
				fmt.Printf("  - %s==%s\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - %s\n", dep.ImportPath)
			}
		}
		return nil
	}

	// Append to requirements.txt
	file, err := os.OpenFile(reqPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open requirements.txt: %w", err)
	}
	defer file.Close()

	fmt.Printf("Adding %d dependencies to requirements.txt...\n", len(neededDeps))

	for _, dep := range neededDeps {
		var line string
		if dep.Version != "" {
			line = fmt.Sprintf("%s==%s", dep.ImportPath, dep.Version)
		} else {
			line = dep.ImportPath
		}

		if _, err := file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write dependency %s: %w", dep.ImportPath, err)
		}

		fmt.Printf("  Added %s\n", dep.ImportPath)
	}

	fmt.Printf("Successfully added %d dependencies to requirements.txt\n", len(neededDeps))
	return nil
}

// addToPyprojectToml adds dependencies to pyproject.toml
func (h *PythonHandler) addToPyprojectToml(projectPath string, dependencies []Dependency, dryRun bool) error {
	if dryRun {
		fmt.Printf("Would add the following Python dependencies to pyproject.toml:\n")
		for _, dep := range dependencies {
			if dep.Version != "" {
				fmt.Printf("  - %s = \"%s\"\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - %s = \"*\"\n", dep.ImportPath)
			}
		}
		return nil
	}

	// For now, we'll fall back to pip install for pyproject.toml
	return fmt.Errorf("automatic pyproject.toml editing not yet implemented, please add dependencies manually or use pip")
}

// addWithPip installs dependencies using pip
func (h *PythonHandler) addWithPip(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if dryRun {
		fmt.Printf("Would install the following Python dependencies with pip:\n")
		for _, dep := range dependencies {
			if dep.Version != "" {
				fmt.Printf("  - %s==%s\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - %s\n", dep.ImportPath)
			}
		}
		return nil
	}

	fmt.Printf("Installing %d dependencies with pip...\n", len(dependencies))

	for _, dep := range dependencies {
		fmt.Printf("  Installing %s...\n", dep.ImportPath)

		args := []string{"install"}
		if dep.Version != "" {
			args = append(args, dep.ImportPath+"=="+dep.Version)
		} else {
			args = append(args, dep.ImportPath)
		}

		cmd := exec.CommandContext(ctx, "pip", args...)
		cmd.Dir = projectPath

		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("failed to install dependency %s: %w\nOutput: %s", dep.ImportPath, err, string(output))
		}
	}

	fmt.Printf("Successfully installed %d dependencies\n", len(dependencies))
	return nil
}

// createRequirementsTxt creates a new requirements.txt file with dependencies
func (h *PythonHandler) createRequirementsTxt(projectPath string, dependencies []Dependency, dryRun bool) error {
	reqPath := filepath.Join(projectPath, "requirements.txt")

	if dryRun {
		fmt.Printf("Would create %s with the following dependencies:\n", reqPath)
		for _, dep := range dependencies {
			if dep.Version != "" {
				fmt.Printf("  - %s==%s\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - %s\n", dep.ImportPath)
			}
		}
		return nil
	}

	file, err := os.Create(reqPath)
	if err != nil {
		return fmt.Errorf("failed to create requirements.txt: %w", err)
	}
	defer file.Close()

	fmt.Printf("Creating requirements.txt with %d dependencies...\n", len(dependencies))

	for _, dep := range dependencies {
		var line string
		if dep.Version != "" {
			line = fmt.Sprintf("%s==%s", dep.ImportPath, dep.Version)
		} else {
			line = dep.ImportPath
		}

		if _, err := file.WriteString(line + "\n"); err != nil {
			return fmt.Errorf("failed to write dependency %s: %w", dep.ImportPath, err)
		}

		fmt.Printf("  Added %s\n", dep.ImportPath)
	}

	fmt.Printf("Successfully created requirements.txt with %d dependencies\n", len(dependencies))
	return nil
}

// filterExistingDependenciesFromRequirements filters out dependencies already in requirements.txt
func (h *PythonHandler) filterExistingDependenciesFromRequirements(reqPath string, dependencies []Dependency) ([]Dependency, error) {
	existingDeps, err := h.getExistingDependenciesFromRequirements(reqPath)
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

// getExistingDependenciesFromRequirements reads requirements.txt and returns existing dependencies
func (h *PythonHandler) getExistingDependenciesFromRequirements(reqPath string) (map[string]bool, error) {
	file, err := os.Open(reqPath)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]bool), nil
		}
		return nil, err
	}
	defer file.Close()

	dependencies := make(map[string]bool)
	scanner := bufio.NewScanner(file)

	// Regex for matching package requirements
	packageRegex := regexp.MustCompile(`^([a-zA-Z0-9\-\_\.]+)(?:[>=<!\s]+([^\s#]+))?`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := packageRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			dependencies[matches[1]] = true
		}
	}

	return dependencies, scanner.Err()
}

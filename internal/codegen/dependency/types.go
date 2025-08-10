package dependency

import (
	"context"
)

// Dependency represents a package dependency that needs to be added
type Dependency struct {
	Name        string `json:"name"`        // Package name
	Version     string `json:"version"`     // Version constraint (optional)
	Language    string `json:"language"`    // Programming language
	ImportPath  string `json:"import_path"` // Import path/module name
	Category    string `json:"category"`    // Category: core, instrumentation, exporter, etc.
	Description string `json:"description"` // Human-readable description
	Required    bool   `json:"required"`    // Whether this dependency is required or optional
}

// DependencyHandler defines the interface for language-specific dependency management
type DependencyHandler interface {
	// AddDependencies adds the specified dependencies to the project
	AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error

	// GetCoreDependencies returns the core OpenTelemetry dependencies for this language
	GetCoreDependencies() []Dependency

	// GetInstrumentationDependency returns the dependency for a specific instrumentation
	GetInstrumentationDependency(instrumentation string) *Dependency

	// GetComponentDependency returns the dependency for a specific component
	GetComponentDependency(componentType, component string) *Dependency

	// ValidateProjectStructure checks if the project has required dependency management files
	ValidateProjectStructure(projectPath string) error

	// GetDependencyFiles returns the paths to dependency management files (go.mod, package.json, etc.)
	GetDependencyFiles(projectPath string) []string

	// GetLanguage returns the language this handler supports
	GetLanguage() string

	// ResolveInstrumentationPrerequisites allows a language handler to expand
	// a set of requested instrumentations with any required prerequisites
	// (e.g., "express" -> also include "http"). Implementations may return
	// the same list if no changes are needed.
	ResolveInstrumentationPrerequisites(instrumentations []string) []string
}

// DependencyResult represents the result of adding dependencies
type DependencyResult struct {
	AddedDependencies   []Dependency      `json:"added_dependencies"`
	SkippedDependencies []Dependency      `json:"skipped_dependencies"` // Already exists
	ErroredDependencies []DependencyError `json:"errored_dependencies"`
	ModifiedFiles       []string          `json:"modified_files"`
}

// DependencyError represents an error that occurred while adding a dependency
type DependencyError struct {
	Dependency Dependency `json:"dependency"`
	Error      string     `json:"error"`
}

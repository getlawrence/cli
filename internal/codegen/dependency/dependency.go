package dependency

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/internal/codegen/types"
)

// DependencyWriter handles adding dependencies to projects
type DependencyWriter struct {
	handlers map[string]DependencyHandler
}

// NewDependencyWriter creates a new dependency manager with all supported handlers
func NewDependencyWriter() *DependencyWriter {
	return &DependencyWriter{
		handlers: map[string]DependencyHandler{
			"go":         NewGoHandler(),
			"javascript": NewJavaScriptHandler(),
			"python":     NewPythonHandler(),
			"java":       NewJavaHandler(),
			"c#":         NewDotNetHandler(),
			"dotnet":     NewDotNetHandler(),
			"ruby":       NewRubyHandler(),
			"php":        NewPHPHandler(),
		},
	}
}

// AddDependencies adds required dependencies to the project
func (dm *DependencyWriter) AddDependencies(
	ctx context.Context,
	projectPath string,
	language string,
	operationsData *types.OperationsData,
	req types.GenerationRequest,
) error {
	handler, exists := dm.handlers[language]
	if !exists {
		return fmt.Errorf("unsupported language for dependency management: %s", language)
	}

	// Collect all required dependencies
	dependencies := dm.collectRequiredDependencies(operationsData, handler)

	if len(dependencies) == 0 {
		return nil // No dependencies to add
	}

	return handler.AddDependencies(ctx, projectPath, dependencies, req.Config.DryRun)
}

// collectRequiredDependencies gathers all dependencies needed based on operations data
func (dm *DependencyWriter) collectRequiredDependencies(operationsData *types.OperationsData, handler DependencyHandler) []Dependency {
	var dependencies []Dependency

	if operationsData.InstallOTEL {
		// Add core OTEL dependencies
		dependencies = append(dependencies, handler.GetCoreDependencies()...)
	}

	// Add instrumentation dependencies
	for _, instrumentation := range operationsData.InstallInstrumentations {
		if dep := handler.GetInstrumentationDependency(instrumentation); dep != nil {
			dependencies = append(dependencies, *dep)
		}
	}

	// Add component dependencies
	for componentType, components := range operationsData.InstallComponents {
		for _, component := range components {
			if dep := handler.GetComponentDependency(componentType, component); dep != nil {
				dependencies = append(dependencies, *dep)
			}
		}
	}

	return dependencies
}

// GetSupportedLanguages returns all languages supported by dependency management
func (dm *DependencyWriter) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(dm.handlers))
	for lang := range dm.handlers {
		languages = append(languages, lang)
	}
	return languages
}

// ValidateProjectStructure checks if the project has the required dependency management files
func (dm *DependencyWriter) ValidateProjectStructure(projectPath, language string) error {
	handler, exists := dm.handlers[language]
	if !exists {
		return fmt.Errorf("unsupported language: %s", language)
	}

	return handler.ValidateProjectStructure(projectPath)
}

// GetRequiredDependencies returns all dependencies that would be added for the given operations
func (dm *DependencyWriter) GetRequiredDependencies(language string, operationsData *types.OperationsData) ([]Dependency, error) {
	handler, exists := dm.handlers[language]
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	return dm.collectRequiredDependencies(operationsData, handler), nil
}

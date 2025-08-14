package dependency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/orchestrator"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	generatorTypes "github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/logger"
)

// DependencyWriter handles adding dependencies to projects using the new modular system
type DependencyWriter struct {
	orchestrator *orchestrator.Orchestrator
	kb           *knowledge.KnowledgeBase
	logger       logger.Logger
}

// NewDependencyWriter creates a new dependency manager using the modular orchestrator
func NewDependencyWriter(l logger.Logger) *DependencyWriter {
	// Get repo root (walk up to find go.mod)
	repoRoot, err := findRepoRoot()
	if err != nil {
		l.Logf("Warning: could not find repo root: %v\n", err)
		repoRoot = "."
	}

	// Load knowledge base
	kb, err := knowledge.LoadFromFile(repoRoot)
	if err != nil {
		l.Logf("Warning: could not load knowledge base: %v\n", err)
		// Create empty KB as fallback
		kb = &knowledge.KnowledgeBase{
			Languages: make(map[string]knowledge.LanguagePackages),
		}
	}

	// Create registry and orchestrator
	commander := commander.NewReal()
	reg := registry.New(commander)
	orch := orchestrator.New(reg, kb)

	return &DependencyWriter{
		orchestrator: orch,
		kb:           kb,
		logger:       l,
	}
}

// AddDependencies adds required dependencies to the project
func (dm *DependencyWriter) AddDependencies(
	ctx context.Context,
	projectPath string,
	language string,
	operationsData *generatorTypes.OperationsData,
	req generatorTypes.GenerationRequest,
) error {
	dm.logger.Logf("Debug: AddDependencies called for language=%s, path=%s, installOTEL=%v\n", language, projectPath, operationsData.InstallOTEL)
	// Convert OperationsData to InstallPlan
	plan := types.InstallPlan{
		Language:                language,
		InstallOTEL:             operationsData.InstallOTEL,
		InstallInstrumentations: operationsData.InstallInstrumentations,
		InstallComponents:       operationsData.InstallComponents,
	}

	// Run orchestrator
	installed, err := dm.orchestrator.Run(ctx, projectPath, plan, req.Config.DryRun)
	if err != nil {
		return err
	}

	// Log results
	if req.Config.DryRun {
		if len(installed) > 0 {
			dm.logger.Logf("Would install the following %s dependencies:\n", language)
			for _, dep := range installed {
				dm.logger.Logf("  - %s\n", dep)
			}
		}
	} else if len(installed) > 0 {
		dm.logger.Logf("Installed %d %s dependencies\n", len(installed), language)
	}

	return nil
}

// GetRequiredDependencies returns all dependencies that would be added for the given operations
func (dm *DependencyWriter) GetRequiredDependencies(language string, operationsData *generatorTypes.OperationsData) ([]types.Dependency, error) {
	// Convert to InstallPlan
	plan := types.InstallPlan{
		Language:                language,
		InstallOTEL:             operationsData.InstallOTEL,
		InstallInstrumentations: operationsData.InstallInstrumentations,
		InstallComponents:       operationsData.InstallComponents,
	}

	// Get all required packages from KB
	var deps []types.Dependency

	if plan.InstallOTEL {
		for _, pkg := range dm.kb.GetCorePackages(language) {
			deps = append(deps, types.Dependency{
				Name:       pkg,
				Language:   language,
				ImportPath: pkg,
				Category:   "core",
				Required:   true,
			})
		}
	}

	for _, inst := range plan.InstallInstrumentations {
		if pkg := dm.kb.GetInstrumentationPackage(language, inst); pkg != "" {
			deps = append(deps, types.Dependency{
				Name:       inst,
				Language:   language,
				ImportPath: pkg,
				Category:   "instrumentation",
			})
		}
	}

	for compType, components := range plan.InstallComponents {
		for _, comp := range components {
			if pkg := dm.kb.GetComponentPackage(language, compType, comp); pkg != "" {
				deps = append(deps, types.Dependency{
					Name:       comp,
					Language:   language,
					ImportPath: pkg,
					Category:   compType,
				})
			}
		}
	}

	return deps, nil
}

// ValidateProjectStructure checks if the project has the required dependency management files
func (dm *DependencyWriter) ValidateProjectStructure(projectPath, language string) error {
	// Create a temporary registry to get scanner
	commander := commander.NewReal()
	reg := registry.New(commander)
	
	scanner, err := reg.GetScanner(language)
	if err != nil {
		return err
	}

	if !scanner.Detect(projectPath) {
		return fmt.Errorf("no dependency management file found for %s project", language)
	}

	return nil
}

// GetSupportedLanguages returns all languages supported by dependency management
func (dm *DependencyWriter) GetSupportedLanguages() []string {
	return []string{"go", "javascript", "python", "ruby", "php", "java", "csharp", "dotnet"}
}

// findRepoRoot walks up directory tree to find go.mod
func findRepoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found")
		}
		dir = parent
	}
}

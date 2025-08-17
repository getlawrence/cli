package dependency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/orchestrator"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	generatorTypes "github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/logger"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
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

	// Load knowledge base using the new system
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

// GetEnhancedDependencies returns enhanced dependency information using the new knowledge system
func (dm *DependencyWriter) GetEnhancedDependencies(language string, operationsData *generatorTypes.OperationsData) ([]EnhancedDependency, error) {
	// Get the underlying new knowledge base for enhanced information
	newKB := dm.kb.GetNewKnowledgeBase()
	if newKB == nil {
		// Fallback to basic dependencies if new KB not available
		basicDeps, err := dm.GetRequiredDependencies(language, operationsData)
		if err != nil {
			return nil, err
		}

		// Convert to enhanced format
		var enhancedDeps []EnhancedDependency
		for _, dep := range basicDeps {
			enhancedDeps = append(enhancedDeps, EnhancedDependency{
				Dependency: dep,
				Metadata:   nil,
			})
		}
		return enhancedDeps, nil
	}

	// Use the new knowledge base for enhanced information
	var enhancedDeps []EnhancedDependency

	// Get core packages with enhanced metadata
	if operationsData.InstallOTEL {
		for _, component := range newKB.Components {
			if string(component.Language) == language &&
				(component.Type == "SDK" || component.Type == "API" || component.Category == "CORE") {
				// Skip components that only have "unknown" versions (exist in registry but not in package manager)
				if hasOnlyUnknownVersions(component) {
					continue
				}
				enhancedDeps = append(enhancedDeps, EnhancedDependency{
					Dependency: types.Dependency{
						Name:       component.Name,
						Language:   language,
						ImportPath: component.Name,
						Category:   "core",
						Required:   true,
					},
					Metadata: &DependencyMetadata{
						Description:     component.Description,
						Repository:      component.Repository,
						License:         component.License,
						Status:          string(component.Status),
						SupportLevel:    string(component.SupportLevel),
						Documentation:   component.DocumentationURL,
						LatestVersion:   getLatestVersion(component),
						BreakingChanges: getBreakingChanges(component),
					},
				})
			}
		}
	}

	// Get instrumentations with enhanced metadata
	for _, inst := range operationsData.InstallInstrumentations {
		for _, component := range newKB.Components {
			if string(component.Language) == language &&
				component.Type == "Instrumentation" &&
				isInstrumentationFor(component, inst) {
				enhancedDeps = append(enhancedDeps, EnhancedDependency{
					Dependency: types.Dependency{
						Name:       inst,
						Language:   language,
						ImportPath: component.Name,
						Category:   "instrumentation",
					},
					Metadata: &DependencyMetadata{
						Description:     component.Description,
						Repository:      component.Repository,
						License:         component.License,
						Status:          string(component.Status),
						SupportLevel:    string(component.SupportLevel),
						Documentation:   component.DocumentationURL,
						LatestVersion:   getLatestVersion(component),
						BreakingChanges: getBreakingChanges(component),
					},
				})
				break
			}
		}
	}

	return enhancedDeps, nil
}

// hasOnlyUnknownVersions checks if a component only has "unknown" versions
func hasOnlyUnknownVersions(component kbtypes.Component) bool {
	if len(component.Versions) == 0 {
		return false
	}

	// Check if all versions are "unknown"
	for _, version := range component.Versions {
		if version.Name != "unknown" {
			return false
		}
	}

	// All versions are "unknown" - this package exists in registry but not in package manager
	return true
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

// EnhancedDependency represents a dependency with additional metadata
type EnhancedDependency struct {
	Dependency types.Dependency
	Metadata   *DependencyMetadata
}

// DependencyMetadata contains additional information about a dependency
type DependencyMetadata struct {
	Description     string   `json:"description,omitempty"`
	Repository      string   `json:"repository,omitempty"`
	License         string   `json:"license,omitempty"`
	Status          string   `json:"status,omitempty"`
	SupportLevel    string   `json:"support_level,omitempty"`
	Documentation   string   `json:"documentation,omitempty"`
	LatestVersion   string   `json:"latest_version,omitempty"`
	BreakingChanges []string `json:"breaking_changes,omitempty"`
}

// Helper functions for enhanced dependency information
func getLatestVersion(component kbtypes.Component) string {
	for _, version := range component.Versions {
		if version.Status == "latest" && !version.Deprecated {
			return version.Name
		}
	}
	return ""
}

func getBreakingChanges(component kbtypes.Component) []string {
	var changes []string
	for _, version := range component.Versions {
		for _, breaking := range version.BreakingChanges {
			changes = append(changes, fmt.Sprintf("%s: %s", breaking.Version, breaking.Description))
		}
	}
	return changes
}

func isInstrumentationFor(component kbtypes.Component, target string) bool {
	// Check if this instrumentation targets the specified framework/library
	name := strings.ToLower(component.Name)
	target = strings.ToLower(target)

	// Simple pattern matching - could be enhanced with more sophisticated logic
	return strings.Contains(name, target) ||
		strings.Contains(name, "instrumentation-"+target) ||
		strings.Contains(name, "@opentelemetry/instrumentation-"+target)
}

package dependency

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
	"github.com/getlawrence/cli/internal/codegen/dependency/orchestrator"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	generatorTypes "github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/client"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

// DependencyWriter handles adding dependencies to projects using the new modular system
type DependencyWriter struct {
	orchestrator *orchestrator.Orchestrator
	kb           *client.KnowledgeClient
	logger       logger.Logger
}

// NewDependencyWriter creates a new dependency manager using the modular orchestrator
func NewDependencyWriter(l logger.Logger) *DependencyWriter {
	// Load knowledge base using the unified client
	kb, err := client.NewKnowledgeClient("knowledge.db")
	if err != nil {
		l.Logf("Warning: could not load knowledge base: %v\n", err)
		// This will cause a panic, but it's better to fail fast than have broken functionality
		// In production, you might want to create a mock client instead
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
		corePackages, err := dm.kb.GetCorePackages(language)
		if err != nil {
			return nil, fmt.Errorf("failed to get core packages: %w", err)
		}
		for _, pkg := range corePackages {
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
		pkg, err := dm.kb.GetInstrumentationPackage(language, inst)
		if err != nil {
			return nil, fmt.Errorf("failed to get instrumentation package for %s: %w", inst, err)
		}
		if pkg != "" {
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
			pkg, err := dm.kb.GetComponentPackage(language, compType, comp)
			if err != nil {
				return nil, fmt.Errorf("failed to get component package for %s: %w", comp, err)
			}
			if pkg != "" {
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

// GetEnhancedDependencies returns enhanced dependency information using the unified client
func (dm *DependencyWriter) GetEnhancedDependencies(language string, operationsData *generatorTypes.OperationsData) ([]EnhancedDependency, error) {
	var enhancedDeps []EnhancedDependency

	// Get core packages with enhanced metadata
	if operationsData.InstallOTEL {
		coreResult, err := dm.kb.GetComponentsByLanguage(language, 0, 0) // Get all core components
		if err != nil {
			return nil, fmt.Errorf("failed to get core components: %w", err)
		}

		for _, component := range coreResult.Components {
			if component.Type == kbtypes.ComponentTypeSDK || component.Type == kbtypes.ComponentTypeAPI {
				// Skip components that only have "unknown" versions
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
		pkg, err := dm.kb.GetInstrumentationPackage(language, inst)
		if err != nil {
			continue // Skip on error
		}
		if pkg == "" {
			continue // Skip if not found
		}

		// Get the component details
		component, err := dm.kb.GetComponentByName(pkg)
		if err != nil || component == nil {
			// Fallback to basic dependency if component details not found
			enhancedDeps = append(enhancedDeps, EnhancedDependency{
				Dependency: types.Dependency{
					Name:       inst,
					Language:   language,
					ImportPath: pkg,
					Category:   "instrumentation",
				},
				Metadata: nil,
			})
			continue
		}

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
				LatestVersion:   getLatestVersion(*component),
				BreakingChanges: getBreakingChanges(*component),
			},
		})
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

func isInstrumentationFor(componentName, target string) bool {
	// Check if this instrumentation targets the specified framework/library
	name := strings.ToLower(componentName)
	target = strings.ToLower(target)

	// Simple pattern matching - could be enhanced with more sophisticated logic
	return strings.Contains(name, target) ||
		strings.Contains(name, "instrumentation-"+target) ||
		strings.Contains(name, "@opentelemetry/instrumentation-"+target)
}

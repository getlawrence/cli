package generator

import (
	"context"
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/pkg/knowledge/client"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeEnhancedGenerator extends the base generator with knowledge base capabilities
type KnowledgeEnhancedGenerator struct {
	*Generator
	knowledgeClient *client.KnowledgeClient
}

// NewKnowledgeEnhancedGenerator creates a new knowledge-enhanced generator
func NewKnowledgeEnhancedGenerator(baseGenerator *Generator) (*KnowledgeEnhancedGenerator, error) {
	knowledgeClient, err := client.NewKnowledgeClient("knowledge.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge client: %w", err)
	}

	return &KnowledgeEnhancedGenerator{
		Generator:       baseGenerator,
		knowledgeClient: knowledgeClient,
	}, nil
}

// Close closes the underlying resources
func (g *KnowledgeEnhancedGenerator) Close() error {
	return g.knowledgeClient.Close()
}

// GenerateWithKnowledge generates code with enhanced knowledge base recommendations
func (g *KnowledgeEnhancedGenerator) GenerateWithKnowledge(ctx context.Context, req types.GenerationRequest) error {
	// First, run the base generation
	if err := g.Generator.Generate(ctx, req); err != nil {
		return fmt.Errorf("base generation failed: %w", err)
	}

	// Enhance with knowledge base recommendations
	if err := g.enhanceWithKnowledge(ctx, req); err != nil {
		// Log warning but don't fail the generation
		g.logger.Logf("Warning: Failed to enhance with knowledge base: %v\n", err)
	}

	return nil
}

// enhanceWithKnowledge enhances the generation with knowledge base insights
func (g *KnowledgeEnhancedGenerator) enhanceWithKnowledge(ctx context.Context, req types.GenerationRequest) error {
	// Analyze the codebase to get package information
	analysis, err := g.Generator.detector.AnalyzeCodebase(ctx, req.CodebasePath)
	if err != nil {
		return fmt.Errorf("failed to analyze codebase: %w", err)
	}

	// Generate enhanced recommendations
	recommendations := g.generateEnhancedRecommendations(ctx, analysis, req)
	g.logger.Logf("Debug: Generated %d enhanced recommendations\n", len(recommendations))
	if len(recommendations) > 0 {
		g.outputEnhancedRecommendations(recommendations)
	} else {
		g.logger.Logf("Debug: No enhanced recommendations generated - checking packages...\n")
		for _, dirAnalysis := range analysis.DirectoryAnalyses {
			g.logger.Logf("Debug: Directory %s has %d packages\n", dirAnalysis.Directory, len(dirAnalysis.Packages))
			for _, pkg := range dirAnalysis.Packages {
				component, err := g.knowledgeClient.GetComponentByName(pkg.Name)
				if err != nil || component == nil {
					g.logger.Logf("Debug: Package %s not found in knowledge base\n", pkg.Name)
				} else {
					g.logger.Logf("Debug: Package %s found in knowledge base: %s\n", pkg.Name, component.Type)
				}
			}
		}
	}

	return nil
}

// EnhancedRecommendation represents an enhanced recommendation from the knowledge base
type EnhancedRecommendation struct {
	Type        string   `json:"type"`
	Package     string   `json:"package"`
	Current     string   `json:"current,omitempty"`
	Recommended string   `json:"recommended,omitempty"`
	Priority    string   `json:"priority"`
	Reason      string   `json:"reason"`
	Description string   `json:"description,omitempty"`
	Benefits    []string `json:"benefits,omitempty"`
	Options     []string `json:"options,omitempty"`
	Migration   string   `json:"migration,omitempty"`
}

// generateEnhancedRecommendations generates enhanced recommendations using the knowledge base
func (g *KnowledgeEnhancedGenerator) generateEnhancedRecommendations(ctx context.Context, analysis *detector.Analysis, req types.GenerationRequest) []EnhancedRecommendation {
	var recommendations []EnhancedRecommendation

	for _, dirAnalysis := range analysis.DirectoryAnalyses {
		for _, pkg := range dirAnalysis.Packages {
			// Find the package in the knowledge base
			component, err := g.knowledgeClient.GetComponentByName(pkg.Name)
			if err != nil || component == nil {
				continue
			}

			// Generate recommendations based on component type and status
			pkgRecommendations := g.generatePackageRecommendations(component, pkg, req)
			recommendations = append(recommendations, pkgRecommendations...)
		}
	}

	return recommendations
}

// generatePackageRecommendations generates recommendations for a specific package
func (g *KnowledgeEnhancedGenerator) generatePackageRecommendations(component *kbtypes.Component, pkg domain.Package, req types.GenerationRequest) []EnhancedRecommendation {
	var recommendations []EnhancedRecommendation

	g.logger.Logf("Debug: Generating recommendations for package %s (version: %s)\n", pkg.Name, pkg.Version)
	g.logger.Logf("Debug: Component type: %s, status: %s\n", component.Type, component.Status)

	// Check if this is an OpenTelemetry component
	if g.isOTelComponent(component) {
		g.logger.Logf("Debug: %s is an OpenTelemetry component\n", pkg.Name)
		// Recommend latest stable version if outdated
		latestVersion := g.getLatestStableVersion(component)
		g.logger.Logf("Debug: Latest version for %s: %v\n", pkg.Name, latestVersion)
		if latestVersion != nil {
			g.logger.Logf("Debug: Comparing versions - current: %s, latest: %s\n", pkg.Version, latestVersion.Name)
		}
		if latestVersion != nil && latestVersion.Name != pkg.Version {
			recommendations = append(recommendations, EnhancedRecommendation{
				Type:        "version_update",
				Package:     pkg.Name,
				Current:     pkg.Version,
				Recommended: latestVersion.Name,
				Priority:    "medium",
				Reason:      "Latest stable version available",
				Benefits:    []string{"Bug fixes", "Security updates", "New features"},
				Migration:   latestVersion.ChangelogURL, // Use ChangelogURL instead of MigrationGuideURL
			})
		}

		// Check for breaking changes
		breakingChanges := g.getBreakingChangesForVersion(component, pkg.Version)
		if len(breakingChanges) > 0 {
			for _, breaking := range breakingChanges {
				recommendations = append(recommendations, EnhancedRecommendation{
					Type:        "breaking_change",
					Package:     pkg.Name,
					Current:     pkg.Version,
					Priority:    "high",
					Reason:      fmt.Sprintf("Breaking change in version %s", breaking.Version),
					Description: breaking.Description,
					Migration:   breaking.MigrationGuideURL,
				})
			}
		}

		// Check for deprecated status
		if component.Status == kbtypes.ComponentStatusDeprecated {
			recommendations = append(recommendations, EnhancedRecommendation{
				Type:        "deprecated",
				Package:     pkg.Name,
				Current:     pkg.Version,
				Priority:    "high",
				Reason:      "Package is deprecated",
				Description: "This package has been deprecated and may not receive updates",
				Migration:   component.MigrationGuideURL,
			})
		}
	} else {
		// This is not an OpenTelemetry component, check if we have instrumentation for it
		instrumentations := g.findInstrumentationsForPackage(component, pkg)
		if len(instrumentations) > 0 {
			recommendations = append(recommendations, EnhancedRecommendation{
				Type:        "instrumentation",
				Package:     pkg.Name,
				Priority:    "medium",
				Reason:      "OpenTelemetry instrumentation available",
				Description: fmt.Sprintf("Found %d instrumentation options for this package", len(instrumentations)),
				Options:     instrumentations,
			})
		}
	}

	return recommendations
}

// isOTelComponent checks if a component is an OpenTelemetry component
func (g *KnowledgeEnhancedGenerator) isOTelComponent(component *kbtypes.Component) bool {
	if component == nil {
		return false
	}

	// Check if the name contains OpenTelemetry indicators
	name := strings.ToLower(component.Name)
	return strings.Contains(name, "opentelemetry") ||
		strings.Contains(name, "otel") ||
		strings.Contains(name, "open-telemetry")
}

// getLatestStableVersion returns the latest stable version of a component
func (g *KnowledgeEnhancedGenerator) getLatestStableVersion(component *kbtypes.Component) *kbtypes.Version {
	g.logger.Logf("Debug: Component %s has %d versions\n", component.Name, len(component.Versions))
	for i, version := range component.Versions {
		g.logger.Logf("Debug: Version %d: %s (status: %s, deprecated: %v)\n", i, version.Name, version.Status, version.Deprecated)
		if version.Status == kbtypes.VersionStatusLatest && !version.Deprecated {
			return &version
		}
	}
	// If no "latest" version found, return the first non-deprecated version
	for _, version := range component.Versions {
		if !version.Deprecated {
			g.logger.Logf("Debug: Using first non-deprecated version: %s\n", version.Name)
			return &version
		}
	}
	return nil
}

// getBreakingChangesForVersion returns breaking changes for a specific version
func (g *KnowledgeEnhancedGenerator) getBreakingChangesForVersion(component *kbtypes.Component, version string) []kbtypes.BreakingChange {
	for _, ver := range component.Versions {
		if ver.Name == version {
			return ver.BreakingChanges
		}
	}
	return nil
}

// findInstrumentationsForPackage finds available instrumentations for a package
func (g *KnowledgeEnhancedGenerator) findInstrumentationsForPackage(component *kbtypes.Component, pkg domain.Package) []string {
	var instrumentations []string

	// Query for instrumentations that target this package
	query := client.ComponentQuery{
		Language:  string(g.convertLanguage(pkg.Language)),
		Type:      string(kbtypes.ComponentTypeInstrumentation),
		Framework: pkg.Name,
	}

	result, err := g.knowledgeClient.QueryComponents(query)
	if err != nil {
		return instrumentations
	}

	for _, comp := range result.Components {
		instrumentations = append(instrumentations, comp.Name)
	}

	return instrumentations
}

// convertLanguage converts detector language to knowledge base language
func (g *KnowledgeEnhancedGenerator) convertLanguage(lang string) kbtypes.ComponentLanguage {
	switch strings.ToLower(lang) {
	case "javascript", "js", "typescript", "ts":
		return kbtypes.ComponentLanguageJavaScript
	case "python", "py":
		return kbtypes.ComponentLanguagePython
	case "go":
		return kbtypes.ComponentLanguageGo
	case "java":
		return kbtypes.ComponentLanguageJava
	case "csharp", "c#", "dotnet":
		return kbtypes.ComponentLanguageCSharp
	case "php":
		return kbtypes.ComponentLanguagePHP
	case "ruby":
		return kbtypes.ComponentLanguageRuby
	default:
		return kbtypes.ComponentLanguageJavaScript // Default fallback
	}
}

// outputEnhancedRecommendations outputs the enhanced recommendations
func (g *KnowledgeEnhancedGenerator) outputEnhancedRecommendations(recommendations []EnhancedRecommendation) {
	g.logger.Logf("\n🔍 Enhanced Recommendations from Knowledge Base:\n")
	g.logger.Logf("================================================\n\n")

	for i, rec := range recommendations {
		g.logger.Logf("%d. [%s] %s\n", i+1, strings.ToUpper(rec.Priority), rec.Type)
		g.logger.Logf("   Package: %s\n", rec.Package)
		if rec.Current != "" {
			g.logger.Logf("   Current: %s\n", rec.Current)
		}
		if rec.Recommended != "" {
			g.logger.Logf("   Recommended: %s\n", rec.Recommended)
		}
		g.logger.Logf("   Reason: %s\n", rec.Reason)
		if rec.Description != "" {
			g.logger.Logf("   Details: %s\n", rec.Description)
		}
		if len(rec.Benefits) > 0 {
			g.logger.Logf("   Benefits: %s\n", strings.Join(rec.Benefits, ", "))
		}
		if len(rec.Options) > 0 {
			g.logger.Logf("   Options: %s\n", strings.Join(rec.Options, ", "))
		}
		if rec.Migration != "" {
			g.logger.Logf("   Migration Guide: %s\n", rec.Migration)
		}
		g.logger.Logf("\n")
	}
}

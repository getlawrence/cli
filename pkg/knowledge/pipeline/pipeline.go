package pipeline

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// Pipeline represents the knowledge base update pipeline
type Pipeline struct {
	providerFactory providers.ProviderFactory
	rateLimiter     *RateLimiter
}

// NewPipeline creates a new pipeline instance
func NewPipeline() *Pipeline {
	return &Pipeline{
		providerFactory: providers.NewProviderFactory(),
		rateLimiter:     NewRateLimiter(100, time.Second), // 100 requests per second
	}
}

// NewPipelineWithProviderFactory creates a new pipeline with a custom provider factory
func NewPipelineWithProviderFactory(providerFactory providers.ProviderFactory) *Pipeline {
	return &Pipeline{
		providerFactory: providerFactory,
		rateLimiter:     NewRateLimiter(100, time.Second),
	}
}

// UpdateKnowledgeBase updates the knowledge base with fresh data for the specified language
func (p *Pipeline) UpdateKnowledgeBase(language types.ComponentLanguage) (*types.KnowledgeBase, error) {
	log.Printf("Starting knowledge base update for language: %s", language)

	// Get the provider for the specified language (for future use)
	_, err := p.providerFactory.GetProvider(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get provider for language %s: %w", language, err)
	}

	// Get registry and package manager providers
	registryProvider, err := p.providerFactory.GetRegistryProvider(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get registry provider for language %s: %w", language, err)
	}

	packageManagerProvider, err := p.providerFactory.GetPackageManagerProvider(language)
	if err != nil {
		return nil, fmt.Errorf("failed to get package manager provider for language %s: %w", language, err)
	}

	// Step 1: Fetch components from registry
	log.Printf("Fetching components from %s registry...", registryProvider.GetName())
	registryComponents, err := registryProvider.DiscoverComponents(context.Background(), string(language))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry components: %w", err)
	}
	log.Printf("Found %d components in registry", len(registryComponents))

	// Step 2: Enrich with package manager data
	log.Printf("Enriching components with %s metadata...", packageManagerProvider.GetName())
	enrichedComponents, err := p.enrichComponentsWithPackageManager(registryComponents, packageManagerProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich components: %w", err)
	}

	// Step 3: Convert to knowledge base format
	log.Printf("Converting to knowledge base format...")
	components := p.convertToComponents(enrichedComponents, language)

	// Step 4: Generate statistics
	log.Printf("Generating statistics...")
	statistics := p.generateStatistics(components, language)

	// Step 5: Create knowledge base
	kb := &types.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components:    components,
		Statistics:    statistics,
		Metadata: map[string]interface{}{
			"source":           fmt.Sprintf("%s + %s", registryProvider.GetRegistryType(), packageManagerProvider.GetPackageManagerType()),
			"language":         string(language),
			"registry":         registryProvider.GetName(),
			"package_manager":  packageManagerProvider.GetName(),
			"update_timestamp": time.Now().Unix(),
		},
	}

	log.Printf("Knowledge base update completed successfully")
	return kb, nil
}

// enrichComponentsWithPackageManager enriches registry components with package manager metadata
func (p *Pipeline) enrichComponentsWithPackageManager(registryComponents []providers.RegistryComponent, packageManagerProvider providers.PackageManagerProvider) ([]EnrichedComponent, error) {
	var enriched []EnrichedComponent

	for i, rc := range registryComponents {
		log.Printf("Processing component %d/%d: %s", i+1, len(registryComponents), rc.Name)

		enrichedComponent := EnrichedComponent{
			RegistryComponent: rc,
		}

		// Try to fetch package manager data for this component
		if packageData, err := p.fetchPackageManagerData(rc.Name, packageManagerProvider); err == nil {
			enrichedComponent.PackageData = packageData
		} else {
			log.Printf("Warning: Failed to fetch package manager data for %s: %v", rc.Name, err)
		}

		enriched = append(enriched, enrichedComponent)

		// Rate limiting
		p.rateLimiter.Wait()
	}

	return enriched, nil
}

// fetchPackageManagerData fetches package metadata from the package manager
func (p *Pipeline) fetchPackageManagerData(componentName string, packageManagerProvider providers.PackageManagerProvider) (*providers.PackageMetadata, error) {
	// Extract package name from component name
	packageName := p.extractPackageName(componentName, packageManagerProvider.GetPackageManagerType())
	if packageName == "" {
		return nil, fmt.Errorf("could not extract package name from %s", componentName)
	}

	return packageManagerProvider.GetPackage(context.Background(), packageName)
}

// extractPackageName extracts the package name from a component name based on package manager type
func (p *Pipeline) extractPackageName(componentName, packageManagerType string) string {
	switch packageManagerType {
	case "npm":
		// Handle different naming patterns for npm
		if strings.HasPrefix(componentName, "@opentelemetry/") {
			return componentName
		}
		if strings.Contains(componentName, "opentelemetry") {
			return componentName
		}
	case "pypi":
		// Handle different naming patterns for PyPI
		if strings.HasPrefix(componentName, "opentelemetry-") {
			return componentName
		}
		if strings.Contains(componentName, "opentelemetry") {
			return componentName
		}
	}

	return ""
}

// convertToComponents converts enriched components to the knowledge base format
func (p *Pipeline) convertToComponents(enriched []EnrichedComponent, language types.ComponentLanguage) []types.Component {
	var components []types.Component

	for _, ec := range enriched {
		component := types.Component{
			Name:                   ec.Name,
			Type:                   p.mapComponentType(ec.Type),
			Category:               p.determineComponentCategory(ec),
			Status:                 p.determineComponentStatus(ec),
			SupportLevel:           p.determineSupportLevel(ec),
			Language:               language,
			Description:            ec.Description,
			Repository:             ec.Repository,
			RegistryURL:            ec.RegistryURL,
			Homepage:               ec.Homepage,
			Tags:                   ec.Tags,
			Maintainers:            ec.Maintainers,
			License:                ec.License,
			LastUpdated:            ec.LastUpdated,
			Versions:               p.extractVersions(ec),
			InstrumentationTargets: p.extractInstrumentationTargets(ec),
			DocumentationURL:       p.extractDocumentationURL(ec),
			ExamplesURL:            p.extractExamplesURL(ec),
			MigrationGuideURL:      p.extractMigrationGuideURL(ec),
		}

		components = append(components, component)
	}

	return components
}

// determineComponentCategory determines the category of a component
func (p *Pipeline) determineComponentCategory(ec EnrichedComponent) types.ComponentCategory {
	name := strings.ToLower(ec.Name)

	// Check for stable SDK patterns
	if strings.Contains(name, "sdk") && !strings.Contains(name, "experimental") {
		return types.ComponentCategoryStableSDK
	}

	// Check for API patterns
	if strings.Contains(name, "api") {
		return types.ComponentCategoryAPI
	}

	// Check for experimental patterns
	if strings.Contains(name, "experimental") || strings.Contains(name, "contrib") {
		return types.ComponentCategoryExperimental
	}

	// Check for core patterns
	if strings.Contains(name, "core") {
		return types.ComponentCategoryCore
	}

	// Default to contrib for instrumentations
	if ec.Type == "instrumentation" {
		return types.ComponentCategoryContrib
	}

	return types.ComponentCategoryExperimental
}

// determineComponentStatus determines the status of a component
func (p *Pipeline) determineComponentStatus(ec EnrichedComponent) types.ComponentStatus {
	name := strings.ToLower(ec.Name)

	// Check for deprecated patterns
	if strings.Contains(name, "deprecated") || strings.Contains(name, "legacy") {
		return types.ComponentStatusDeprecated
	}

	// Check for experimental patterns
	if strings.Contains(name, "experimental") || strings.Contains(name, "contrib") {
		return types.ComponentStatusExperimental
	}

	// Check for beta/alpha patterns
	if strings.Contains(name, "beta") {
		return types.ComponentStatusBeta
	}
	if strings.Contains(name, "alpha") {
		return types.ComponentStatusAlpha
	}

	// Check if it's a contrib component (which are typically experimental)
	if strings.Contains(ec.Repository, "opentelemetry-js-contrib") || strings.Contains(ec.Repository, "opentelemetry-python-contrib") {
		return types.ComponentStatusExperimental
	}

	// Default to stable for core components
	return types.ComponentStatusStable
}

// determineSupportLevel determines the support level of a component
func (p *Pipeline) determineSupportLevel(ec EnrichedComponent) types.SupportLevel {
	// Check if it's an official OpenTelemetry component
	if strings.HasPrefix(ec.Name, "@opentelemetry/") || strings.HasPrefix(ec.Name, "opentelemetry-") {
		// Contrib components are community-supported
		if strings.Contains(ec.Repository, "opentelemetry-js-contrib") || strings.Contains(ec.Repository, "opentelemetry-python-contrib") {
			return types.SupportLevelCommunity
		}
		return types.SupportLevelOfficial
	}

	// Check maintainers for community indicators
	for _, maintainer := range ec.Maintainers {
		if strings.Contains(strings.ToLower(maintainer), "opentelemetry") {
			return types.SupportLevelOfficial
		}
	}

	// Default to community
	return types.SupportLevelCommunity
}

// extractInstrumentationTargets extracts instrumentation target information
func (p *Pipeline) extractInstrumentationTargets(ec EnrichedComponent) []types.InstrumentationTarget {
	var targets []types.InstrumentationTarget

	if ec.Type != "instrumentation" {
		return targets
	}

	// Extract framework/library name from component name
	framework := p.extractFrameworkName(ec.Name)
	if framework != "" {
		targets = append(targets, types.InstrumentationTarget{
			Framework:    framework,
			VersionRange: ">=1.0.0", // Default range, could be enhanced with package data
		})
	}

	return targets
}

// extractFrameworkName extracts the framework name from an instrumentation component name
func (p *Pipeline) extractFrameworkName(componentName string) string {
	// Handle @opentelemetry/instrumentation-{framework} pattern
	if strings.Contains(componentName, "instrumentation-") {
		parts := strings.Split(componentName, "instrumentation-")
		if len(parts) > 1 {
			return strings.Title(parts[1]) // Capitalize first letter
		}
	}

	// Handle other patterns
	if strings.Contains(componentName, "otel-") {
		parts := strings.Split(componentName, "otel-")
		if len(parts) > 1 {
			return strings.Title(parts[1])
		}
	}

	return ""
}

// extractDocumentationURL extracts documentation URL from component data
func (p *Pipeline) extractDocumentationURL(ec EnrichedComponent) string {
	if ec.Homepage != "" {
		return ec.Homepage
	}

	// Generate documentation URL based on repository
	if strings.Contains(ec.Repository, "github.com") {
		return strings.Replace(ec.Repository, "github.com", "github.com", 1) + "/blob/main/README.md"
	}

	return ""
}

// extractExamplesURL extracts examples URL from component data
func (p *Pipeline) extractExamplesURL(ec EnrichedComponent) string {
	if strings.Contains(ec.Repository, "github.com") {
		return strings.Replace(ec.Repository, "github.com", "github.com", 1) + "/tree/main/examples"
	}

	return ""
}

// extractMigrationGuideURL extracts migration guide URL from component data
func (p *Pipeline) extractMigrationGuideURL(ec EnrichedComponent) string {
	// Check if there's a migration guide in the repository
	if strings.Contains(ec.Repository, "github.com") {
		return strings.Replace(ec.Repository, "github.com", "github.com", 1) + "/blob/main/MIGRATION.md"
	}

	return ""
}

// mapComponentType maps registry component type to knowledge base type
func (p *Pipeline) mapComponentType(registryType string) types.ComponentType {
	switch strings.ToLower(registryType) {
	case "api":
		return types.ComponentTypeAPI
	case "sdk":
		return types.ComponentTypeSDK
	case "instrumentation":
		return types.ComponentTypeInstrumentation
	case "exporter":
		return types.ComponentTypeExporter
	case "propagator":
		return types.ComponentTypePropagator
	case "sampler":
		return types.ComponentTypeSampler
	case "processor":
		return types.ComponentTypeProcessor
	case "resource":
		return types.ComponentTypeResource
	case "resourcedetector":
		return types.ComponentTypeResourceDetector
	default:
		return types.ComponentTypeInstrumentation // Default fallback
	}
}

// extractVersions extracts version information from enriched component data
func (p *Pipeline) extractVersions(ec EnrichedComponent) []types.Version {
	var versions []types.Version

	if ec.PackageData != nil {
		// Extract versions from package data
		for versionStr, versionData := range ec.PackageData.Versions {
			version := types.Version{
				Name:         versionStr,
				ReleaseDate:  ec.PackageData.Time[versionStr],
				Dependencies: p.convertDependencies(versionData.Dependencies),
				Status:       p.determineVersionStatus(versionStr, ec.PackageData.DistTags),
				Metadata: map[string]interface{}{
					"package_manager_url": p.generatePackageManagerURL(ec.PackageData, versionStr),
				},
				ChangelogURL:        p.extractChangelogURL(ec, versionStr),
				CoreVersion:         p.extractCoreVersion(ec, versionStr),
				ExperimentalVersion: p.extractExperimentalVersion(ec, versionStr),
				Compatible:          p.extractCompatibleComponents(ec, versionStr),
				BreakingChanges:     p.extractBreakingChanges(ec, versionStr),
			}

			// Extract runtime version requirements
			if engines, ok := versionData.Engines["node"]; ok {
				version.MinRuntimeVersion = engines
			}
			if engines, ok := versionData.Engines["python"]; ok {
				version.MinRuntimeVersion = engines
			}

			versions = append(versions, version)
		}
	}

	// If no versions from package manager, create a basic one
	if len(versions) == 0 {
		versions = append(versions, types.Version{
			Name:        "unknown",
			ReleaseDate: time.Now(),
			Status:      types.VersionStatusStable,
		})
	}

	return versions
}

// generatePackageManagerURL generates the URL for a package in the package manager
func (p *Pipeline) generatePackageManagerURL(packageData *providers.PackageMetadata, version string) string {
	if packageData == nil {
		return ""
	}

	switch packageData.Name {
	case "npm":
		return fmt.Sprintf("https://www.npmjs.com/package/%s/v/%s", packageData.Name, version)
	case "pypi":
		return fmt.Sprintf("https://pypi.org/project/%s/%s/", packageData.Name, version)
	default:
		return ""
	}
}

// extractChangelogURL extracts changelog URL for a version
func (p *Pipeline) extractChangelogURL(ec EnrichedComponent, version string) string {
	if strings.Contains(ec.Repository, "github.com") {
		return fmt.Sprintf("%s/releases/tag/v%s", ec.Repository, version)
	}
	return ""
}

// extractCoreVersion extracts the core version compatibility
func (p *Pipeline) extractCoreVersion(ec EnrichedComponent, version string) string {
	// This would typically come from package.json or other metadata
	// For now, return empty - could be enhanced with dependency analysis
	return ""
}

// extractExperimentalVersion extracts the experimental version compatibility
func (p *Pipeline) extractExperimentalVersion(ec EnrichedComponent, version string) string {
	// This would typically come from package.json or other metadata
	// For now, return empty - could be enhanced with dependency analysis
	return ""
}

// extractCompatibleComponents extracts compatible component information
func (p *Pipeline) extractCompatibleComponents(ec EnrichedComponent, version string) []types.CompatibleComponent {
	var compatible []types.CompatibleComponent

	// This would typically come from package.json dependencies or other metadata
	// For now, return empty - could be enhanced with dependency analysis

	return compatible
}

// extractBreakingChanges extracts breaking change information
func (p *Pipeline) extractBreakingChanges(ec EnrichedComponent, version string) []types.BreakingChange {
	var breakingChanges []types.BreakingChange

	// This would typically come from changelog parsing or release notes
	// For now, return empty - could be enhanced with LLM analysis or static rules

	return breakingChanges
}

// convertDependencies converts package manager dependencies to knowledge base format
func (p *Pipeline) convertDependencies(packageDeps map[string]string) map[string]types.Dependency {
	deps := make(map[string]types.Dependency)

	for name, version := range packageDeps {
		deps[name] = types.Dependency{
			Name:    name,
			Version: version,
			Type:    "runtime",
		}
	}

	return deps
}

// determineVersionStatus determines the status of a version based on dist tags
func (p *Pipeline) determineVersionStatus(version string, distTags map[string]string) types.VersionStatus {
	if latest, ok := distTags["latest"]; ok && latest == version {
		return types.VersionStatusLatest
	}

	if strings.Contains(version, "beta") || strings.Contains(version, "alpha") {
		return types.VersionStatusBeta
	}

	return types.VersionStatusStable
}

// generateStatistics generates statistics for the knowledge base
func (p *Pipeline) generateStatistics(components []types.Component, language types.ComponentLanguage) types.Statistics {
	stats := types.Statistics{
		TotalComponents: len(components),
		ByLanguage:      make(map[string]int),
		ByType:          make(map[string]int),
		ByCategory:      make(map[string]int),
		ByStatus:        make(map[string]int),
		BySupportLevel:  make(map[string]int),
		LastUpdate:      time.Now(),
		Source:          fmt.Sprintf("OpenTelemetry Registry + %s", language),
	}

	// Count by language
	stats.ByLanguage[string(language)] = len(components)

	// Count by type, category, status, and support level
	for _, component := range components {
		typeStr := string(component.Type)
		stats.ByType[typeStr]++

		if component.Category != "" {
			categoryStr := string(component.Category)
			stats.ByCategory[categoryStr]++
		}

		if component.Status != "" {
			statusStr := string(component.Status)
			stats.ByStatus[statusStr]++
		}

		if component.SupportLevel != "" {
			supportStr := string(component.SupportLevel)
			stats.BySupportLevel[supportStr]++
		}

		// Count total versions
		stats.TotalVersions += len(component.Versions)
	}

	return stats
}

// EnrichedComponent represents a component enriched with package manager data
type EnrichedComponent struct {
	providers.RegistryComponent
	PackageData *providers.PackageMetadata
}

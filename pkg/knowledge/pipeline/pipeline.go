package pipeline

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
	"github.com/getlawrence/cli/pkg/knowledge/utils"
)

/*
GitHub API Optimization

This pipeline implements an optimization to reduce GitHub API calls when fetching
changelog information for multiple components from the same repository.

Instead of making individual API calls for each component/version combination,
the pipeline:

1. Groups components by their repository URL
2. Makes one API call per unique repository to fetch all releases
3. Caches the releases in memory
4. Filters the cached releases client-side when processing individual components

For example, if 50 packages come from the same repository (like js-contrib),
this reduces GitHub API calls from 50 to 1, significantly improving performance
and reducing the risk of hitting rate limits.

The optimization is transparent to the rest of the system - if a repository
isn't cached, it falls back to individual API calls.
*/

// RepositoryReleasesCache caches GitHub releases for repositories to avoid duplicate API calls
type RepositoryReleasesCache struct {
	releases map[string][]providers.GitHubRelease
	mu       sync.RWMutex
}

// NewRepositoryReleasesCache creates a new repository releases cache
func NewRepositoryReleasesCache() *RepositoryReleasesCache {
	return &RepositoryReleasesCache{
		releases: make(map[string][]providers.GitHubRelease),
	}
}

// Get retrieves cached releases for a repository
func (c *RepositoryReleasesCache) Get(repositoryURL string) ([]providers.GitHubRelease, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	releases, exists := c.releases[repositoryURL]
	return releases, exists
}

// Set stores releases for a repository in the cache
func (c *RepositoryReleasesCache) Set(repositoryURL string, releases []providers.GitHubRelease) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.releases[repositoryURL] = releases
}

// Pipeline represents the knowledge base update pipeline
type Pipeline struct {
	providerFactory providers.ProviderFactory
	rateLimiter     *utils.RateLimiter
	githubClient    *providers.GitHubClient
	logger          logger.Logger
	releasesCache   *RepositoryReleasesCache
	storageClient   *storage.Storage
}

// NewPipelineWithLoggerAndToken creates a new pipeline with a custom logger and GitHub token
func NewPipeline(providerFactory providers.ProviderFactory, logger logger.Logger, githubToken string, storageClient *storage.Storage) *Pipeline {
	return &Pipeline{
		providerFactory: providerFactory,
		rateLimiter:     utils.NewRateLimiter(100, time.Second),
		githubClient:    providers.NewGitHubClient(githubToken),
		logger:          logger,
		releasesCache:   NewRepositoryReleasesCache(),
		storageClient:   storageClient,
	}
}

// GetCacheStats returns statistics about the repository releases cache
func (p *Pipeline) GetCacheStats() map[string]interface{} {
	p.releasesCache.mu.RLock()
	defer p.releasesCache.mu.RUnlock()

	stats := make(map[string]interface{})
	stats["cached_repositories"] = len(p.releasesCache.releases)

	totalReleases := 0
	for _, releases := range p.releasesCache.releases {
		totalReleases += len(releases)
	}
	stats["total_cached_releases"] = totalReleases

	return stats
}

// UpdateKnowledgeBase updates the knowledge base with fresh data for the specified language
func (p *Pipeline) UpdateKnowledgeBase(language types.ComponentLanguage) error {
	p.logger.Logf("Starting knowledge base update for language: %s\n", language)

	// Get registry and package manager providers
	registryProvider, err := p.providerFactory.GetRegistryProvider(language)
	if err != nil {
		return fmt.Errorf("failed to get registry provider for language %s: %w", language, err)
	}

	packageManagerProvider, err := p.providerFactory.GetPackageManagerProvider(language)
	if err != nil {
		return fmt.Errorf("failed to get package manager provider for language %s: %w", language, err)
	}

	// Step 1: Fetch components from registry
	p.logger.Logf("Fetching components from %s registry...\n", registryProvider.GetName())
	registryComponents, err := registryProvider.DiscoverComponents(context.Background(), string(language))
	if err != nil {
		return fmt.Errorf("failed to fetch registry components: %w", err)
	}
	p.logger.Logf("Found %d components in registry\n", len(registryComponents))

	// Step 2: Enrich with package manager data
	p.logger.Logf("Enriching components with %s metadata...\n", packageManagerProvider.GetName())
	enrichedComponents, err := p.enrichComponentsWithPackageManager(registryComponents, packageManagerProvider)
	if err != nil {
		return fmt.Errorf("failed to enrich components: %w", err)
	}

	// Step 3: Convert to knowledge base format
	p.logger.Log("Converting to knowledge base format...")
	components := p.convertToComponents(enrichedComponents, language)

	p.storageClient.SaveComponents(components, "knowledge.db")

	p.logger.Log("Knowledge base update completed successfully")
	return nil
}

// enrichComponentsWithPackageManager enriches registry components with package manager metadata
func (p *Pipeline) enrichComponentsWithPackageManager(registryComponents []providers.RegistryComponent, packageManagerProvider providers.PackageManagerProvider) ([]EnrichedComponent, error) {
	var enriched []EnrichedComponent

	// Step 1: Group components by repository to optimize GitHub API calls
	p.logger.Log("Grouping components by repository for efficient GitHub API usage...")
	repositoryGroups := p.groupComponentsByRepository(registryComponents)

	// Step 2: Fetch GitHub releases for each unique repository
	p.logger.Log("Fetching GitHub releases for repositories...")
	if err := p.fetchRepositoryReleases(repositoryGroups); err != nil {
		p.logger.Logf("Warning: Failed to fetch some repository releases: %v\n", err)
	}

	// Step 3: Process each component with package manager data and cached GitHub data
	for i, rc := range registryComponents {
		// Skip components with empty names
		if rc.Name == "" || strings.TrimSpace(rc.Name) == "" {
			p.logger.Logf("Skipping component %d/%d: empty name\n", i+1, len(registryComponents))
			continue
		}

		p.logger.Logf("Processing component %d/%d: %s\n", i+1, len(registryComponents), rc.Name)

		enrichedComponent := EnrichedComponent{
			RegistryComponent: rc,
		}

		// Try to fetch package manager data for this component
		if packageData, err := p.fetchPackageManagerData(rc.Name, packageManagerProvider); err == nil {
			enrichedComponent.PackageData = packageData
		} else {
			p.logger.Logf("Warning: Failed to fetch package manager data for %s: %v\n", rc.Name, err)
		}

		enriched = append(enriched, enrichedComponent)

		// Rate limiting
		p.rateLimiter.Wait()
	}

	return enriched, nil
}

// groupComponentsByRepository groups components by their repository URL
func (p *Pipeline) groupComponentsByRepository(components []providers.RegistryComponent) map[string][]providers.RegistryComponent {
	groups := make(map[string][]providers.RegistryComponent)

	for _, component := range components {
		if component.Repository != "" && strings.Contains(component.Repository, "github.com") {
			baseRepo := extractBaseRepository(component.Repository)
			p.logger.Logf("Grouping component %s by repository %s\n", component.Name, baseRepo)
			groups[baseRepo] = append(groups[baseRepo], component)
		}
	}

	return groups
}

// extractBaseRepository extracts the base repository URL from a GitHub URL
// Input: https://github.com/Azure/azure-sdk-for-js/tree/main/sdk/monitor/monitor-opentelemetry-exporter
// Output: github.com/Azure/azure-sdk-for-js
func extractBaseRepository(fullURL string) string {
	// Remove protocol if present
	url := strings.TrimPrefix(fullURL, "https://")
	url = strings.TrimPrefix(url, "http://")

	// Split by "/" and take the first two parts (github.com/owner/repo)
	parts := strings.Split(url, "/")
	if len(parts) >= 3 && parts[0] == "github.com" {
		return strings.Join(parts[:3], "/")
	}

	// Fallback to original URL if we can't parse it
	return fullURL
}

// GroupComponentsByRepository groups components by their repository URL
func (p *Pipeline) GroupComponentsByRepository(components []providers.RegistryComponent) map[string][]providers.RegistryComponent {
	groups := make(map[string][]providers.RegistryComponent)

	for _, component := range components {
		if component.Repository != "" && strings.Contains(component.Repository, "github.com") {
			groups[component.Repository] = append(groups[component.Repository], component)
		}
	}

	return groups
}

// fetchRepositoryReleases fetches all releases for each unique repository
func (p *Pipeline) fetchRepositoryReleases(repositoryGroups map[string][]providers.RegistryComponent) error {
	if p.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	ctx := context.Background()

	totalComponents := 0
	for _, components := range repositoryGroups {
		totalComponents += len(components)
	}

	p.logger.Logf("Found %d unique repositories for %d total components\n", len(repositoryGroups), totalComponents)
	p.logger.Logf("This optimization will reduce GitHub API calls from %d to %d\n", totalComponents, len(repositoryGroups))

	for repositoryURL := range repositoryGroups {
		p.logger.Logf("Fetching releases for repository: %s\n", repositoryURL)

		// Extract owner and repo from repository URL
		owner, repo, err := p.githubClient.ExtractOwnerAndRepo(repositoryURL)
		if err != nil {
			p.logger.Logf("Warning: Failed to extract owner/repo from %s: %v\n", repositoryURL, err)
			continue
		}

		// Fetch all releases for this repository
		releases, err := p.githubClient.FetchReleases(ctx, owner, repo)
		if err != nil {
			p.logger.Logf("Warning: Failed to fetch releases for %s: %v\n", repositoryURL, err)
			continue
		}

		// Cache the releases
		p.releasesCache.Set(repositoryURL, releases)
		p.logger.Logf("Cached %d releases for repository: %s\n", len(releases), repositoryURL)

		// Rate limiting between repository requests
		p.rateLimiter.Wait()
	}

	return nil
}

// fetchPackageManagerData fetches package metadata from the package manager
func (p *Pipeline) fetchPackageManagerData(componentName string, packageManagerProvider providers.PackageManagerProvider) (*providers.PackageMetadata, error) {
	// Extract package name from component name
	packageName := p.extractPackageName(componentName)
	if packageName == "" {
		return nil, fmt.Errorf("could not extract package name from '%s'", componentName)
	}

	return packageManagerProvider.GetPackage(context.Background(), packageName)
}

// extractPackageName extracts the package name from a component name based on package manager type
func (p *Pipeline) extractPackageName(componentName string) string {
	// Handle empty or invalid component names
	if componentName == "" || strings.TrimSpace(componentName) == "" {
		return ""
	}

	// Clean the component name
	componentName = strings.TrimSpace(componentName)

	// Return the component name as-is - it's already in the correct format for its package manager
	return componentName
}

// convertToComponents converts enriched components to the knowledge base format
func (p *Pipeline) convertToComponents(enriched []EnrichedComponent, language types.ComponentLanguage) []types.Component {
	var components []types.Component

	for _, ec := range enriched {
		component := types.Component{
			Name:                   ec.Name,
			Type:                   p.mapComponentType(ec.Type, ec.Name),
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

// mapComponentType maps registry component type to knowledge base type with smart name-based fallback
func (p *Pipeline) mapComponentType(registryType string, componentName string) types.ComponentType {
	// First, try to map based on the registry type
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
	}

	// If registry type doesn't match, use smart name-based detection
	return p.detectComponentTypeFromName(componentName)
}

// detectComponentTypeFromName detects component type based on the component name
func (p *Pipeline) detectComponentTypeFromName(componentName string) types.ComponentType {
	name := strings.ToLower(componentName)

	// API detection patterns
	if strings.Contains(name, "/api") || strings.HasSuffix(name, "-api") ||
		strings.HasSuffix(name, ".api") || name == "api" ||
		strings.Contains(name, "opentelemetry-api") {
		return types.ComponentTypeAPI
	}

	// SDK detection patterns
	if strings.Contains(name, "/sdk") || strings.HasSuffix(name, "-sdk") ||
		strings.HasSuffix(name, ".sdk") || name == "sdk" ||
		strings.Contains(name, "opentelemetry-sdk") ||
		strings.Contains(name, "sdk-") {
		return types.ComponentTypeSDK
	}

	// Exporter detection patterns
	if strings.Contains(name, "exporter") || strings.Contains(name, "-exporter-") {
		return types.ComponentTypeExporter
	}

	// Propagator detection patterns
	if strings.Contains(name, "propagator") || strings.Contains(name, "-propagator-") {
		return types.ComponentTypePropagator
	}

	// Sampler detection patterns
	if strings.Contains(name, "sampler") || strings.Contains(name, "-sampler-") {
		return types.ComponentTypeSampler
	}

	// Processor detection patterns
	if strings.Contains(name, "processor") || strings.Contains(name, "-processor-") {
		return types.ComponentTypeProcessor
	}

	// Resource detection patterns
	if strings.Contains(name, "resource") && !strings.Contains(name, "detector") {
		return types.ComponentTypeResource
	}

	// Resource detector detection patterns
	if strings.Contains(name, "resource") && strings.Contains(name, "detector") {
		return types.ComponentTypeResourceDetector
	}

	// Default fallback to instrumentation for everything else
	return types.ComponentTypeInstrumentation
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

			p.logger.Logf("Fetching changelog for %s version %s\n", ec.Name, versionStr)
			// Fetch changelog from GitHub if repository is available
			if ec.Repository != "" && strings.Contains(ec.Repository, "github.com") {
				if changelog, err := p.fetchChangelogFromGitHub(ec.Repository, versionStr); err == nil && changelog.Found {
					version.Changelog = changelog.Notes
					// Update release date if GitHub has more accurate information
					if !changelog.ReleaseDate.IsZero() {
						version.ReleaseDate = changelog.ReleaseDate
					}
				}
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

// fetchChangelogFromGitHub fetches changelog information from GitHub releases using cached data
func (p *Pipeline) fetchChangelogFromGitHub(repositoryURL, version string) (*providers.ReleaseNotes, error) {
	if p.githubClient == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	baseRepo := extractBaseRepository(repositoryURL)

	// Check if we have cached releases for this repository
	cachedReleases, exists := p.releasesCache.Get(baseRepo)
	if !exists {
		// Fallback to individual API call if not cached
		p.logger.Logf("Warning: No cached releases for %s, falling back to individual API call\n", baseRepo)
		ctx := context.Background()
		return p.githubClient.GetReleaseNotes(ctx, baseRepo, version)
	}

	// Find the matching release from cached data
	var matchingRelease *providers.GitHubRelease
	for _, release := range cachedReleases {
		if p.matchesVersion(release.TagName, version) {
			matchingRelease = &release
			break
		}
	}

	if matchingRelease == nil {
		return &providers.ReleaseNotes{
			Version:     version,
			ReleaseDate: time.Time{},
			Notes:       "",
			URL:         "",
			Found:       false,
		}, nil
	}

	return &providers.ReleaseNotes{
		Version:     matchingRelease.TagName,
		ReleaseDate: matchingRelease.PublishedAt,
		Notes:       matchingRelease.Body,
		URL:         matchingRelease.HTMLURL,
		Found:       true,
	}, nil
}

// matchesVersion checks if a GitHub tag matches the requested version
func (p *Pipeline) matchesVersion(tagName, version string) bool {
	// Remove 'v' prefix if present
	tag := strings.TrimPrefix(tagName, "v")
	ver := strings.TrimPrefix(version, "v")

	// Direct match
	if tag == ver {
		return true
	}

	// Handle cases where GitHub tag might have additional prefixes/suffixes
	if strings.Contains(tag, ver) || strings.Contains(ver, tag) {
		return true
	}

	// Handle semantic versioning patterns
	// For example: tag "v1.0.0" should match version "1.0.0"
	// Also handle cases like "v1.0.0-beta.1" matching "1.0.0"

	// Split by dots and compare major.minor.patch
	tagParts := strings.Split(tag, ".")
	verParts := strings.Split(ver, ".")

	if len(tagParts) >= 3 && len(verParts) >= 3 {
		// Compare major.minor.patch
		if tagParts[0] == verParts[0] && tagParts[1] == verParts[1] && tagParts[2] == verParts[2] {
			return true
		}
	}

	// Handle pre-release versions (e.g., "1.0.0-beta" should match "1.0.0")
	if len(verParts) >= 3 {
		baseVersion := strings.Join(verParts[:3], ".")
		if strings.HasPrefix(tag, baseVersion) {
			return true
		}
	}

	return false
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

// EnrichedComponent represents a component enriched with package manager data
type EnrichedComponent struct {
	providers.RegistryComponent
	PackageData *providers.PackageMetadata
}

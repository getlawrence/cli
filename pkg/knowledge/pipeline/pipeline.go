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

// Constants for common values
const (
	// GitHub-related constants
	githubDomain = "github.com"
	githubHTTPS  = "https://"
	githubHTTP   = "http://"

	// URL path components
	readmePath    = "/blob/main/README.md"
	examplesPath  = "/tree/main/examples"
	migrationPath = "/blob/main/MIGRATION.md"
	releasesPath  = "/releases/tag/v"

	// Default values
	defaultVersionRange = ">=1.0.0"
	unknownVersion      = "unknown"

	// Rate limiting
	defaultRateLimit = 100
	rateLimitWindow  = time.Second
)

// Component patterns for type detection
var (
	componentPatterns = map[string][]string{
		"api":        {"/api", "-api", ".api", "opentelemetry-api"},
		"sdk":        {"/sdk", "-sdk", ".sdk", "opentelemetry-sdk", "sdk-"},
		"exporter":   {"exporter", "-exporter-"},
		"propagator": {"propagator", "-propagator-"},
		"sampler":    {"sampler", "-sampler-"},
		"processor":  {"processor", "-processor-"},
		"resource":   {"resource"},
		"detector":   {"resource", "detector"},
	}

	statusPatterns = map[string][]string{
		"deprecated":   {"deprecated", "legacy"},
		"experimental": {"experimental", "contrib"},
		"beta":         {"beta"},
		"alpha":        {"alpha"},
	}
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

// NewPipeline creates a new pipeline with a custom logger and GitHub token
func NewPipeline(providerFactory providers.ProviderFactory, logger logger.Logger, githubToken string, storageClient *storage.Storage) *Pipeline {
	return &Pipeline{
		providerFactory: providerFactory,
		rateLimiter:     utils.NewRateLimiter(defaultRateLimit, rateLimitWindow),
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

// UpdateKnowledgeBase updates the knowledge base with fresh data for the specified language(s)
func (p *Pipeline) UpdateKnowledgeBase(languages []types.ComponentLanguage) error {
	if err := p.validateLanguages(languages); err != nil {
		return err
	}

	if len(languages) == 1 {
		return p.updateSingleLanguage(languages[0])
	}

	return p.updateMultipleLanguages(languages)
}

// validateLanguages ensures the languages slice is valid
func (p *Pipeline) validateLanguages(languages []types.ComponentLanguage) error {
	if len(languages) == 0 {
		return fmt.Errorf("no languages specified for update")
	}
	return nil
}

// updateSingleLanguage handles single language updates
func (p *Pipeline) updateSingleLanguage(language types.ComponentLanguage) error {
	p.logger.Logf("Starting knowledge base update for language: %s\n", language)

	components, err := p.processLanguage(language)
	if err != nil {
		return fmt.Errorf("failed to process language %s: %w", language, err)
	}

	p.storageClient.SaveComponents(components)
	p.logger.Log("Knowledge base update completed successfully")
	return nil
}

// updateMultipleLanguages handles multiple language updates
func (p *Pipeline) updateMultipleLanguages(languages []types.ComponentLanguage) error {
	p.logger.Logf("Starting knowledge base update for %d languages\n", len(languages))

	allComponents, err := p.processAllLanguages(languages)
	if err != nil {
		return err
	}

	p.logger.Logf("Saving %d total components to knowledge base...\n", len(allComponents))
	p.storageClient.SaveComponents(allComponents)
	p.logger.Log("Knowledge base update for all languages completed successfully")
	return nil
}

// processAllLanguages processes all languages and returns combined components
func (p *Pipeline) processAllLanguages(languages []types.ComponentLanguage) ([]types.Component, error) {
	var allComponents []types.Component

	for _, language := range languages {
		p.logger.Logf("Processing language: %s\n", language)

		components, err := p.processLanguage(language)
		if err != nil {
			return nil, fmt.Errorf("failed to process language %s: %w", language, err)
		}

		allComponents = append(allComponents, components...)
	}

	return allComponents, nil
}

// processLanguage handles the processing of a single language and returns the components
func (p *Pipeline) processLanguage(language types.ComponentLanguage) ([]types.Component, error) {
	registryProvider, packageManagerProvider, err := p.getProviders(language)
	if err != nil {
		return nil, err
	}

	registryComponents, err := p.fetchRegistryComponents(registryProvider, language)
	if err != nil {
		return nil, err
	}

	enrichedComponents, err := p.enrichComponentsWithPackageManager(registryComponents, packageManagerProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to enrich components: %w", err)
	}

	return p.convertToComponents(enrichedComponents, language), nil
}

// getProviders retrieves registry and package manager providers
func (p *Pipeline) getProviders(language types.ComponentLanguage) (providers.RegistryProvider, providers.PackageManagerProvider, error) {
	registryProvider, err := p.providerFactory.GetRegistryProvider(language)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get registry provider for language %s: %w", language, err)
	}

	packageManagerProvider, err := p.providerFactory.GetPackageManagerProvider(language)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get package manager provider for language %s: %w", language, err)
	}

	return registryProvider, packageManagerProvider, nil
}

// fetchRegistryComponents fetches components from the registry
func (p *Pipeline) fetchRegistryComponents(registryProvider providers.RegistryProvider, language types.ComponentLanguage) ([]providers.RegistryComponent, error) {
	p.logger.Logf("Fetching components from %s registry...\n", registryProvider.GetName())

	registryComponents, err := registryProvider.DiscoverComponents(context.Background(), string(language))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry components: %w", err)
	}

	p.logger.Logf("Found %d components in registry\n", len(registryComponents))
	return registryComponents, nil
}

// enrichComponentsWithPackageManager enriches registry components with package manager metadata
func (p *Pipeline) enrichComponentsWithPackageManager(registryComponents []providers.RegistryComponent, packageManagerProvider providers.PackageManagerProvider) ([]EnrichedComponent, error) {
	if len(registryComponents) == 0 {
		return []EnrichedComponent{}, nil
	}

	repositoryGroups := p.groupComponentsByRepository(registryComponents)

	if err := p.fetchRepositoryReleases(repositoryGroups); err != nil {
		p.logger.Logf("Warning: Failed to fetch some repository releases: %v\n", err)
	}

	return p.processComponents(registryComponents, packageManagerProvider)
}

// groupComponentsByRepository groups components by their repository URL
func (p *Pipeline) groupComponentsByRepository(components []providers.RegistryComponent) map[string][]providers.RegistryComponent {
	groups := make(map[string][]providers.RegistryComponent)

	for _, component := range components {
		if p.isValidGitHubRepository(component.Repository) {
			baseRepo := extractBaseRepository(component.Repository)
			p.logger.Logf("Grouping component %s by repository %s\n", component.Name, baseRepo)
			groups[baseRepo] = append(groups[baseRepo], component)
		}
	}

	return groups
}

// isValidGitHubRepository checks if a repository URL is a valid GitHub repository
func (p *Pipeline) isValidGitHubRepository(repository string) bool {
	return repository != "" && strings.Contains(repository, githubDomain)
}

// extractBaseRepository extracts the base repository URL from a GitHub URL
func extractBaseRepository(fullURL string) string {
	url := strings.TrimPrefix(fullURL, githubHTTPS)
	url = strings.TrimPrefix(url, githubHTTP)

	parts := strings.Split(url, "/")
	if len(parts) >= 3 && parts[0] == githubDomain {
		return strings.Join(parts[:3], "/")
	}

	return fullURL
}

// processComponents processes each component and enriches it with package manager data
func (p *Pipeline) processComponents(registryComponents []providers.RegistryComponent, packageManagerProvider providers.PackageManagerProvider) ([]EnrichedComponent, error) {
	var enriched []EnrichedComponent

	for i, rc := range registryComponents {
		if !p.isValidComponent(rc) {
			p.logger.Logf("Skipping component %d/%d: empty name\n", i+1, len(registryComponents))
			continue
		}

		p.logger.Logf("Processing component %d/%d: %s\n", i+1, len(registryComponents), rc.Name)

		enrichedComponent := p.createEnrichedComponent(rc, packageManagerProvider)
		enriched = append(enriched, enrichedComponent)

		p.rateLimiter.Wait()
	}

	return enriched, nil
}

// isValidComponent checks if a component is valid for processing
func (p *Pipeline) isValidComponent(rc providers.RegistryComponent) bool {
	return rc.Name != "" && strings.TrimSpace(rc.Name) != ""
}

// createEnrichedComponent creates an enriched component with package manager data
func (p *Pipeline) createEnrichedComponent(rc providers.RegistryComponent, packageManagerProvider providers.PackageManagerProvider) EnrichedComponent {
	enrichedComponent := EnrichedComponent{
		RegistryComponent: rc,
	}

	if packageData, err := p.fetchPackageManagerData(rc.Name, packageManagerProvider); err == nil {
		enrichedComponent.PackageData = packageData
	} else {
		p.logger.Logf("Warning: Failed to fetch package manager data for %s: %v\n", rc.Name, err)
	}

	return enrichedComponent
}

// fetchRepositoryReleases fetches all releases for each unique repository
func (p *Pipeline) fetchRepositoryReleases(repositoryGroups map[string][]providers.RegistryComponent) error {
	if p.githubClient == nil {
		return fmt.Errorf("GitHub client not initialized")
	}

	totalComponents := p.countTotalComponents(repositoryGroups)
	p.logger.Logf("Found %d unique repositories for %d total components\n", len(repositoryGroups), totalComponents)
	p.logger.Logf("This optimization will reduce GitHub API calls from %d to %d\n", totalComponents, len(repositoryGroups))

	for repositoryURL := range repositoryGroups {
		if err := p.fetchAndCacheReleases(repositoryURL); err != nil {
			p.logger.Logf("Warning: Failed to fetch releases for %s: %v\n", repositoryURL, err)
			continue
		}
		p.rateLimiter.Wait()
	}

	return nil
}

// countTotalComponents counts the total number of components across all repository groups
func (p *Pipeline) countTotalComponents(repositoryGroups map[string][]providers.RegistryComponent) int {
	total := 0
	for _, components := range repositoryGroups {
		total += len(components)
	}
	return total
}

// fetchAndCacheReleases fetches and caches releases for a single repository
func (p *Pipeline) fetchAndCacheReleases(repositoryURL string) error {
	p.logger.Logf("Fetching releases for repository: %s\n", repositoryURL)

	owner, repo, err := p.githubClient.ExtractOwnerAndRepo(repositoryURL)
	if err != nil {
		return fmt.Errorf("failed to extract owner/repo from %s: %w", repositoryURL, err)
	}

	releases, err := p.githubClient.FetchReleases(context.Background(), owner, repo)
	if err != nil {
		return fmt.Errorf("failed to fetch releases for %s: %w", repositoryURL, err)
	}

	p.releasesCache.Set(repositoryURL, releases)
	p.logger.Logf("Cached %d releases for repository: %s\n", len(releases), repositoryURL)
	return nil
}

// fetchPackageManagerData fetches package metadata from the package manager
func (p *Pipeline) fetchPackageManagerData(componentName string, packageManagerProvider providers.PackageManagerProvider) (*providers.PackageMetadata, error) {
	packageName := p.extractPackageName(componentName)
	if packageName == "" {
		return nil, fmt.Errorf("could not extract package name from '%s'", componentName)
	}

	return packageManagerProvider.GetPackage(context.Background(), packageName)
}

// extractPackageName extracts the package name from a component name
func (p *Pipeline) extractPackageName(componentName string) string {
	if componentName == "" || strings.TrimSpace(componentName) == "" {
		return ""
	}
	return strings.TrimSpace(componentName)
}

// convertToComponents converts enriched components to the knowledge base format
func (p *Pipeline) convertToComponents(enriched []EnrichedComponent, language types.ComponentLanguage) []types.Component {
	components := make([]types.Component, 0, len(enriched))

	for _, ec := range enriched {
		component := p.createComponent(ec, language)
		components = append(components, component)
	}

	return components
}

// createComponent creates a single component from enriched data
func (p *Pipeline) createComponent(ec EnrichedComponent, language types.ComponentLanguage) types.Component {
	return types.Component{
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
}

// determineComponentCategory determines the category of a component
func (p *Pipeline) determineComponentCategory(ec EnrichedComponent) types.ComponentCategory {
	name := strings.ToLower(ec.Name)

	if strings.Contains(name, "sdk") && !strings.Contains(name, "experimental") {
		return types.ComponentCategoryStableSDK
	}

	if strings.Contains(name, "api") {
		return types.ComponentCategoryAPI
	}

	if strings.Contains(name, "experimental") || strings.Contains(name, "contrib") {
		return types.ComponentCategoryExperimental
	}

	if strings.Contains(name, "core") {
		return types.ComponentCategoryCore
	}

	if ec.Type == "instrumentation" {
		return types.ComponentCategoryContrib
	}

	return types.ComponentCategoryExperimental
}

// determineComponentStatus determines the status of a component
func (p *Pipeline) determineComponentStatus(ec EnrichedComponent) types.ComponentStatus {
	name := strings.ToLower(ec.Name)

	for status, patterns := range statusPatterns {
		if p.matchesAnyPattern(name, patterns) {
			switch status {
			case "deprecated":
				return types.ComponentStatusDeprecated
			case "experimental":
				return types.ComponentStatusExperimental
			case "beta":
				return types.ComponentStatusBeta
			case "alpha":
				return types.ComponentStatusAlpha
			}
		}
	}

	if p.isContribComponent(ec.Repository) {
		return types.ComponentStatusExperimental
	}

	return types.ComponentStatusStable
}

// matchesAnyPattern checks if a string matches any of the given patterns
func (p *Pipeline) matchesAnyPattern(text string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.Contains(text, pattern) {
			return true
		}
	}
	return false
}

// isContribComponent checks if a component is a contrib component
func (p *Pipeline) isContribComponent(repository string) bool {
	return strings.Contains(repository, "opentelemetry-js-contrib") ||
		strings.Contains(repository, "opentelemetry-python-contrib")
}

// determineSupportLevel determines the support level of a component
func (p *Pipeline) determineSupportLevel(ec EnrichedComponent) types.SupportLevel {
	if p.isOfficialOpenTelemetryComponent(ec.Name) {
		if p.isContribComponent(ec.Repository) {
			return types.SupportLevelCommunity
		}
		return types.SupportLevelOfficial
	}

	if p.hasOpenTelemetryMaintainer(ec.Maintainers) {
		return types.SupportLevelOfficial
	}

	return types.SupportLevelCommunity
}

// isOfficialOpenTelemetryComponent checks if a component is official OpenTelemetry
func (p *Pipeline) isOfficialOpenTelemetryComponent(name string) bool {
	return strings.HasPrefix(name, "@opentelemetry/") || strings.HasPrefix(name, "opentelemetry-")
}

// hasOpenTelemetryMaintainer checks if maintainers contain OpenTelemetry references
func (p *Pipeline) hasOpenTelemetryMaintainer(maintainers []string) bool {
	for _, maintainer := range maintainers {
		if strings.Contains(strings.ToLower(maintainer), "opentelemetry") {
			return true
		}
	}
	return false
}

// extractInstrumentationTargets extracts instrumentation target information
func (p *Pipeline) extractInstrumentationTargets(ec EnrichedComponent) []types.InstrumentationTarget {
	if ec.Type != "instrumentation" {
		return []types.InstrumentationTarget{}
	}

	framework := p.extractFrameworkName(ec.Name)
	if framework == "" {
		return []types.InstrumentationTarget{}
	}

	return []types.InstrumentationTarget{
		{
			Framework:    framework,
			VersionRange: defaultVersionRange,
		},
	}
}

// extractFrameworkName extracts the framework name from an instrumentation component name
func (p *Pipeline) extractFrameworkName(componentName string) string {
	patterns := []string{"instrumentation-", "otel-"}

	for _, pattern := range patterns {
		if strings.Contains(componentName, pattern) {
			parts := strings.Split(componentName, pattern)
			if len(parts) > 1 {
				return strings.Title(parts[1])
			}
		}
	}

	return ""
}

// extractDocumentationURL extracts documentation URL from component data
func (p *Pipeline) extractDocumentationURL(ec EnrichedComponent) string {
	if ec.Homepage != "" {
		return ec.Homepage
	}

	return p.generateGitHubURL(ec.Repository, readmePath)
}

// extractExamplesURL extracts examples URL from component data
func (p *Pipeline) extractExamplesURL(ec EnrichedComponent) string {
	return p.generateGitHubURL(ec.Repository, examplesPath)
}

// extractMigrationGuideURL extracts migration guide URL from component data
func (p *Pipeline) extractMigrationGuideURL(ec EnrichedComponent) string {
	return p.generateGitHubURL(ec.Repository, migrationPath)
}

// generateGitHubURL generates a GitHub URL with the given path
func (p *Pipeline) generateGitHubURL(repository, path string) string {
	if !p.isValidGitHubRepository(repository) {
		return ""
	}
	return repository + path
}

// mapComponentType maps registry component type to knowledge base type
func (p *Pipeline) mapComponentType(registryType string, componentName string) types.ComponentType {
	registryTypeLower := strings.ToLower(registryType)

	switch registryTypeLower {
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

	return p.detectComponentTypeFromName(componentName)
}

// detectComponentTypeFromName detects component type based on the component name
func (p *Pipeline) detectComponentTypeFromName(componentName string) types.ComponentType {
	name := strings.ToLower(componentName)

	for componentType, patterns := range componentPatterns {
		if p.matchesAnyPattern(name, patterns) {
			switch componentType {
			case "api":
				return types.ComponentTypeAPI
			case "sdk":
				return types.ComponentTypeSDK
			case "exporter":
				return types.ComponentTypeExporter
			case "propagator":
				return types.ComponentTypePropagator
			case "sampler":
				return types.ComponentTypeSampler
			case "processor":
				return types.ComponentTypeProcessor
			case "resource":
				if strings.Contains(name, "detector") {
					return types.ComponentTypeResourceDetector
				}
				return types.ComponentTypeResource
			}
		}
	}

	return types.ComponentTypeInstrumentation
}

// extractVersions extracts version information from enriched component data
func (p *Pipeline) extractVersions(ec EnrichedComponent) []types.Version {
	if ec.PackageData == nil {
		return p.createDefaultVersion()
	}

	return p.extractVersionsFromPackageData(ec)
}

// createDefaultVersion creates a default version when no package data is available
func (p *Pipeline) createDefaultVersion() []types.Version {
	return []types.Version{
		{
			Name:        unknownVersion,
			ReleaseDate: time.Now(),
			Status:      types.VersionStatusStable,
		},
	}
}

// extractVersionsFromPackageData extracts versions from package metadata
func (p *Pipeline) extractVersionsFromPackageData(ec EnrichedComponent) []types.Version {
	var versions []types.Version

	for versionStr, versionData := range ec.PackageData.Versions {
		version := p.createVersion(ec, versionStr, versionData)
		versions = append(versions, version)
	}

	return versions
}

// createVersion creates a version object from package data
func (p *Pipeline) createVersion(ec EnrichedComponent, versionStr string, versionData providers.VersionMetadata) types.Version {
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

	p.setRuntimeVersionRequirements(&version, versionData)
	p.enrichWithChangelog(&version, ec, versionStr)

	return version
}

// setRuntimeVersionRequirements sets runtime version requirements for the version
func (p *Pipeline) setRuntimeVersionRequirements(version *types.Version, versionData providers.VersionMetadata) {
	runtimeEngines := []string{"node", "python"}

	for _, engine := range runtimeEngines {
		if engines, ok := versionData.Engines[engine]; ok {
			version.MinRuntimeVersion = engines
			break
		}
	}
}

// enrichWithChangelog enriches the version with changelog information
func (p *Pipeline) enrichWithChangelog(version *types.Version, ec EnrichedComponent, versionStr string) {
	p.logger.Logf("Fetching changelog for %s version %s\n", ec.Name, versionStr)

	if !p.isValidGitHubRepository(ec.Repository) {
		return
	}

	changelog, err := p.fetchChangelogFromGitHub(ec.Repository, versionStr)
	if err != nil || !changelog.Found {
		return
	}

	version.Changelog = changelog.Notes
	if !changelog.ReleaseDate.IsZero() {
		version.ReleaseDate = changelog.ReleaseDate
	}
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
	if !p.isValidGitHubRepository(ec.Repository) {
		return ""
	}
	return ec.Repository + releasesPath + version
}

// extractCoreVersion extracts the core version compatibility
func (p *Pipeline) extractCoreVersion(ec EnrichedComponent, version string) string {
	// TODO: Implement dependency analysis for core version compatibility
	return ""
}

// extractExperimentalVersion extracts the experimental version compatibility
func (p *Pipeline) extractExperimentalVersion(ec EnrichedComponent, version string) string {
	// TODO: Implement dependency analysis for experimental version compatibility
	return ""
}

// extractCompatibleComponents extracts compatible component information
func (p *Pipeline) extractCompatibleComponents(ec EnrichedComponent, version string) []types.CompatibleComponent {
	// TODO: Implement dependency analysis for compatible components
	return []types.CompatibleComponent{}
}

// extractBreakingChanges extracts breaking change information
func (p *Pipeline) extractBreakingChanges(ec EnrichedComponent, version string) []types.BreakingChange {
	// TODO: Implement LLM analysis or static rules for breaking changes
	return []types.BreakingChange{}
}

// fetchChangelogFromGitHub fetches changelog information from GitHub releases using cached data
func (p *Pipeline) fetchChangelogFromGitHub(repositoryURL, version string) (*providers.ReleaseNotes, error) {
	if p.githubClient == nil {
		return nil, fmt.Errorf("GitHub client not initialized")
	}

	baseRepo := extractBaseRepository(repositoryURL)
	cachedReleases, exists := p.releasesCache.Get(baseRepo)

	if !exists {
		return p.fallbackToIndividualAPI(baseRepo, version)
	}

	return p.findMatchingRelease(cachedReleases, version)
}

// fallbackToIndividualAPI falls back to individual API call if not cached
func (p *Pipeline) fallbackToIndividualAPI(baseRepo, version string) (*providers.ReleaseNotes, error) {
	p.logger.Logf("Warning: No cached releases for %s, falling back to individual API call\n", baseRepo)
	ctx := context.Background()
	return p.githubClient.GetReleaseNotes(ctx, baseRepo, version)
}

// findMatchingRelease finds a matching release from cached data
func (p *Pipeline) findMatchingRelease(cachedReleases []providers.GitHubRelease, version string) (*providers.ReleaseNotes, error) {
	for _, release := range cachedReleases {
		if p.matchesVersion(release.TagName, version) {
			return &providers.ReleaseNotes{
				Version:     release.TagName,
				ReleaseDate: release.PublishedAt,
				Notes:       release.Body,
				URL:         release.HTMLURL,
				Found:       true,
			}, nil
		}
	}

	return &providers.ReleaseNotes{
		Version:     version,
		ReleaseDate: time.Time{},
		Notes:       "",
		URL:         "",
		Found:       false,
	}, nil
}

// matchesVersion checks if a GitHub tag matches the requested version
func (p *Pipeline) matchesVersion(tagName, version string) bool {
	tag := strings.TrimPrefix(tagName, "v")
	ver := strings.TrimPrefix(version, "v")

	if tag == ver {
		return true
	}

	if strings.Contains(tag, ver) || strings.Contains(ver, tag) {
		return true
	}

	return p.matchesSemanticVersion(tag, ver)
}

// matchesSemanticVersion checks if versions match using semantic versioning rules
func (p *Pipeline) matchesSemanticVersion(tag, version string) bool {
	tagParts := strings.Split(tag, ".")
	verParts := strings.Split(version, ".")

	if len(tagParts) >= 3 && len(verParts) >= 3 {
		if tagParts[0] == verParts[0] && tagParts[1] == verParts[1] && tagParts[2] == verParts[2] {
			return true
		}
	}

	if len(verParts) >= 3 {
		baseVersion := strings.Join(verParts[:3], ".")
		return strings.HasPrefix(tag, baseVersion)
	}

	return false
}

// convertDependencies converts package manager dependencies to knowledge base format
func (p *Pipeline) convertDependencies(packageDeps map[string]string) map[string]types.Dependency {
	deps := make(map[string]types.Dependency, len(packageDeps))

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

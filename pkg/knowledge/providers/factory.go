package providers

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/getlawrence/cli/pkg/knowledge/registry"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// DefaultProviderFactory implements the ProviderFactory interface
type DefaultProviderFactory struct {
	providers map[types.ComponentLanguage]Provider
	mu        sync.RWMutex
}

// NewProviderFactory creates a new provider factory with default providers
func NewProviderFactory() *DefaultProviderFactory {
	factory := &DefaultProviderFactory{
		providers: make(map[types.ComponentLanguage]Provider),
	}

	// Register default providers
	factory.registerDefaultProviders()

	return factory
}

// registerDefaultProviders registers the built-in providers
func (f *DefaultProviderFactory) registerDefaultProviders() {
	// JavaScript provider (existing functionality)
	jsProvider := NewJavaScriptProvider()
	f.RegisterProvider(jsProvider)

	// Python provider (new)
	pythonProvider := NewPythonProvider()
	f.RegisterProvider(pythonProvider)

	// Register OTEL core providers for all languages to ensure core packages are included
	allLanguages := []types.ComponentLanguage{
		types.ComponentLanguageJavaScript,
		types.ComponentLanguagePython,
		types.ComponentLanguageGo,
		types.ComponentLanguageJava,
		types.ComponentLanguageCSharp,
		types.ComponentLanguagePHP,
		types.ComponentLanguageRuby,
	}

	for _, lang := range allLanguages {
		// Create OTEL core provider for this language
		otelCoreProvider := NewOTELCoreProvider(lang)

		// Create a composite provider that combines OTEL core with existing providers
		if lang == types.ComponentLanguageJavaScript {
			// For JavaScript, combine with existing provider
			jsPackageManagerProvider := jsProvider.GetPackageManagerProvider()

			compositeProvider := NewCompositeProvider(
				fmt.Sprintf("JavaScript OTEL Core Provider"),
				lang,
				otelCoreProvider,
				jsPackageManagerProvider,
			)
			f.RegisterProvider(compositeProvider)
		} else if lang == types.ComponentLanguagePython {
			// For Python, combine with existing provider
			pythonPackageManagerProvider := pythonProvider.GetPackageManagerProvider()

			compositeProvider := NewCompositeProvider(
				fmt.Sprintf("Python OTEL Core Provider"),
				lang,
				otelCoreProvider,
				pythonPackageManagerProvider,
			)
			f.RegisterProvider(compositeProvider)
		} else {
			// For other languages, create a composite provider with OTEL core
			genericPackageManagerProvider := NewGenericPackageManagerProvider(lang)

			compositeProvider := NewCompositeProvider(
				fmt.Sprintf("%s OTEL Core Provider", strings.Title(string(lang))),
				lang,
				otelCoreProvider,
				genericPackageManagerProvider,
			)
			f.RegisterProvider(compositeProvider)
		}
	}
}

// GetProvider returns a provider for the specified language
func (f *DefaultProviderFactory) GetProvider(language types.ComponentLanguage) (Provider, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	provider, exists := f.providers[language]
	if !exists {
		return nil, fmt.Errorf("no provider found for language: %s", language)
	}

	return provider, nil
}

// GetRegistryProvider returns a registry provider for the specified language
func (f *DefaultProviderFactory) GetRegistryProvider(language types.ComponentLanguage) (RegistryProvider, error) {
	provider, err := f.GetProvider(language)
	if err != nil {
		return nil, err
	}

	// Try to get a composite provider
	if compositeProvider, ok := provider.(*CompositeProvider); ok {
		return compositeProvider.registryProvider, nil
	}

	// Try to get registry provider from provider methods
	if providerWithRegistry, ok := provider.(interface{ GetRegistryProvider() RegistryProvider }); ok {
		return providerWithRegistry.GetRegistryProvider(), nil
	}

	return nil, fmt.Errorf("provider for language %s does not implement RegistryProvider", language)
}

// GetPackageManagerProvider returns a package manager provider for the specified language
func (f *DefaultProviderFactory) GetPackageManagerProvider(language types.ComponentLanguage) (PackageManagerProvider, error) {
	provider, err := f.GetProvider(language)
	if err != nil {
		return nil, err
	}

	// Try to get a composite provider
	if compositeProvider, ok := provider.(*CompositeProvider); ok {
		return compositeProvider.packageManagerProvider, nil
	}

	// Try to get package manager provider from provider methods
	if providerWithPackageManager, ok := provider.(interface{ GetPackageManagerProvider() PackageManagerProvider }); ok {
		return providerWithPackageManager.GetPackageManagerProvider(), nil
	}

	return nil, fmt.Errorf("provider for language %s does not implement PackageManagerProvider", language)
}

// ListSupportedLanguages returns all supported languages
func (f *DefaultProviderFactory) ListSupportedLanguages() []types.ComponentLanguage {
	f.mu.RLock()
	defer f.mu.RUnlock()

	languages := make([]types.ComponentLanguage, 0, len(f.providers))
	for language := range f.providers {
		languages = append(languages, language)
	}

	return languages
}

// RegisterProvider registers a custom provider
func (f *DefaultProviderFactory) RegisterProvider(provider Provider) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	language := provider.GetLanguage()
	f.providers[language] = provider

	return nil
}

// CompositeProvider combines a registry provider and package manager provider
type CompositeProvider struct {
	name                   string
	language               types.ComponentLanguage
	registryProvider       RegistryProvider
	packageManagerProvider PackageManagerProvider
}

// NewCompositeProvider creates a new composite provider
func NewCompositeProvider(name string, language types.ComponentLanguage, registry RegistryProvider, packageManager PackageManagerProvider) *CompositeProvider {
	return &CompositeProvider{
		name:                   name,
		language:               language,
		registryProvider:       registry,
		packageManagerProvider: packageManager,
	}
}

// GetName returns the provider name
func (p *CompositeProvider) GetName() string {
	return p.name
}

// GetLanguage returns the language this provider supports
func (p *CompositeProvider) GetLanguage() types.ComponentLanguage {
	return p.language
}

// GetRegistryType returns the type of registry
func (p *CompositeProvider) GetRegistryType() string {
	return p.registryProvider.GetRegistryType()
}

// GetPackageManagerType returns the type of package manager
func (p *CompositeProvider) GetPackageManagerType() string {
	return p.packageManagerProvider.GetPackageManagerType()
}

// DiscoverComponents implements the Provider interface
func (p *CompositeProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// Get components from registry provider
	registryComponents, err := p.registryProvider.DiscoverComponents(ctx, string(p.language))
	if err != nil {
		return nil, err
	}

	// Convert RegistryComponent to types.Component
	var components []types.Component
	for _, rc := range registryComponents {
		component := types.Component{
			Name:                   rc.Name,
			Type:                   p.mapComponentType(rc.Type),
			Category:               p.determineComponentCategory(rc),
			Status:                 p.determineComponentStatus(rc),
			SupportLevel:           p.determineSupportLevel(rc),
			Language:               p.language,
			Description:            rc.Description,
			Repository:             rc.Repository,
			RegistryURL:            rc.RegistryURL,
			Homepage:               rc.Homepage,
			Tags:                   rc.Tags,
			Maintainers:            rc.Maintainers,
			License:                rc.License,
			LastUpdated:            rc.LastUpdated,
			Versions:               []types.Version{}, // Will be populated by package manager
			InstrumentationTargets: []types.InstrumentationTarget{},
			DocumentationURL:       rc.Homepage,
			ExamplesURL:            "",
			MigrationGuideURL:      "",
		}
		components = append(components, component)
	}

	return components, nil
}

// GetComponentMetadata implements the Provider interface
func (p *CompositeProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	registryComponent, err := p.registryProvider.GetComponentByName(ctx, name)
	if err != nil {
		return nil, err
	}
	if registryComponent == nil {
		return nil, nil
	}

	// Convert to types.Component
	component := types.Component{
		Name:                   registryComponent.Name,
		Type:                   p.mapComponentType(registryComponent.Type),
		Category:               p.determineComponentCategory(*registryComponent),
		Status:                 p.determineComponentStatus(*registryComponent),
		SupportLevel:           p.determineSupportLevel(*registryComponent),
		Language:               p.language,
		Description:            registryComponent.Description,
		Repository:             registryComponent.Repository,
		RegistryURL:            registryComponent.RegistryURL,
		Homepage:               registryComponent.Homepage,
		Tags:                   registryComponent.Tags,
		Maintainers:            registryComponent.Maintainers,
		License:                registryComponent.License,
		LastUpdated:            registryComponent.LastUpdated,
		Versions:               []types.Version{}, // Will be populated by package manager
		InstrumentationTargets: []types.InstrumentationTarget{},
		DocumentationURL:       registryComponent.Homepage,
		ExamplesURL:            "",
		MigrationGuideURL:      "",
	}

	return &component, nil
}

// GetComponentVersions implements the Provider interface
func (p *CompositeProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// For now, return empty versions - this will be populated by package manager
	return []types.Version{}, nil
}

// Helper methods for converting component types and determining metadata
func (p *CompositeProvider) mapComponentType(registryType string) types.ComponentType {
	switch registryType {
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
	default:
		return types.ComponentTypeInstrumentation
	}
}

func (p *CompositeProvider) determineComponentCategory(rc RegistryComponent) types.ComponentCategory {
	name := strings.ToLower(rc.Name)

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

	return types.ComponentCategoryExperimental
}

func (p *CompositeProvider) determineComponentStatus(rc RegistryComponent) types.ComponentStatus {
	name := strings.ToLower(rc.Name)

	if strings.Contains(name, "deprecated") || strings.Contains(name, "legacy") {
		return types.ComponentStatusExperimental
	}
	if strings.Contains(name, "experimental") || strings.Contains(name, "contrib") {
		return types.ComponentStatusExperimental
	}
	if strings.Contains(name, "beta") {
		return types.ComponentStatusBeta
	}
	if strings.Contains(name, "alpha") {
		return types.ComponentStatusAlpha
	}

	return types.ComponentStatusStable
}

func (p *CompositeProvider) determineSupportLevel(rc RegistryComponent) types.SupportLevel {
	if strings.HasPrefix(rc.Name, "@opentelemetry/") || strings.HasPrefix(rc.Name, "opentelemetry-") {
		return types.SupportLevelOfficial
	}

	for _, maintainer := range rc.Maintainers {
		if strings.Contains(strings.ToLower(maintainer), "opentelemetry") {
			return types.SupportLevelOfficial
		}
	}

	return types.SupportLevelCommunity
}

// IsHealthy checks if the provider is healthy
func (p *CompositeProvider) IsHealthy(ctx context.Context) bool {
	return p.registryProvider.IsHealthy(ctx) && p.packageManagerProvider.IsHealthy(ctx)
}

// GenericLanguageProvider implements Provider for any language using the registry client
type GenericLanguageProvider struct {
	language               types.ComponentLanguage
	registryProvider       RegistryProvider
	packageManagerProvider PackageManagerProvider
}

// NewGenericLanguageProvider creates a new generic language provider
func NewGenericLanguageProvider(language types.ComponentLanguage) *GenericLanguageProvider {
	return &GenericLanguageProvider{
		language:               language,
		registryProvider:       NewGenericRegistryProvider(language),
		packageManagerProvider: NewGenericPackageManagerProvider(language),
	}
}

// GetName returns the provider name
func (p *GenericLanguageProvider) GetName() string {
	return fmt.Sprintf("%s Provider", strings.Title(string(p.language)))
}

// GetLanguage returns the language this provider supports
func (p *GenericLanguageProvider) GetLanguage() types.ComponentLanguage {
	return p.language
}

// GetRegistryType returns the type of registry
func (p *GenericLanguageProvider) GetRegistryType() string {
	return "opentelemetry"
}

// GetPackageManagerType returns the type of package manager
func (p *GenericLanguageProvider) GetPackageManagerType() string {
	// Map language to package manager
	packageManagers := map[types.ComponentLanguage]string{
		types.ComponentLanguageGo:     "go",
		types.ComponentLanguageJava:   "maven",
		types.ComponentLanguageCSharp: "nuget",
		types.ComponentLanguagePHP:    "composer",
		types.ComponentLanguageRuby:   "gem",
	}
	return packageManagers[p.language]
}

// DiscoverComponents discovers all components for the language
func (p *GenericLanguageProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Component{}, nil
}

// GetComponentMetadata gets metadata for a specific component
func (p *GenericLanguageProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return nil
	return nil, nil
}

// GetComponentVersions gets versions for a specific component
func (p *GenericLanguageProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Version{}, nil
}

// IsHealthy checks if the provider is healthy
func (p *GenericLanguageProvider) IsHealthy(ctx context.Context) bool {
	return p.registryProvider.IsHealthy(ctx) && p.packageManagerProvider.IsHealthy(ctx)
}

// GetRegistryProvider returns the registry provider
func (p *GenericLanguageProvider) GetRegistryProvider() RegistryProvider {
	return p.registryProvider
}

// GetPackageManagerProvider returns the package manager provider
func (p *GenericLanguageProvider) GetPackageManagerProvider() PackageManagerProvider {
	return p.packageManagerProvider
}

// GenericRegistryProvider implements RegistryProvider for any language
type GenericRegistryProvider struct {
	language types.ComponentLanguage
	client   *registry.Client
}

// NewGenericRegistryProvider creates a new generic registry provider
func NewGenericRegistryProvider(language types.ComponentLanguage) *GenericRegistryProvider {
	return &GenericRegistryProvider{
		language: language,
		client:   registry.NewClient(),
	}
}

// GetName returns the provider name
func (p *GenericRegistryProvider) GetName() string {
	return fmt.Sprintf("%s Registry Provider", strings.Title(string(p.language)))
}

// GetLanguage returns the language this registry supports
func (p *GenericRegistryProvider) GetLanguage() types.ComponentLanguage {
	return p.language
}

// GetRegistryType returns the type of registry
func (p *GenericRegistryProvider) GetRegistryType() string {
	return "opentelemetry"
}

// DiscoverComponents discovers all components for the language
func (p *GenericRegistryProvider) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	registryComponents, err := p.client.GetComponentsByLanguage(string(p.language))
	if err != nil {
		return nil, err
	}

	// Convert registry.RegistryComponent to providers.RegistryComponent
	var components []RegistryComponent
	for _, rc := range registryComponents {
		components = append(components, RegistryComponent{
			Name:        rc.Name,
			Type:        rc.Type,
			Language:    rc.Language,
			Description: rc.Description,
			Repository:  rc.Repository,
			RegistryURL: rc.RegistryURL,
			Homepage:    rc.Homepage,
			Tags:        rc.Tags,
			Maintainers: rc.Maintainers,
			License:     rc.License,
			LastUpdated: rc.LastUpdated,
			Metadata:    rc.Metadata,
		})
	}

	return components, nil
}

// GetComponentByName gets a specific component by name
func (p *GenericRegistryProvider) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
	rc, err := p.client.GetComponentByName(name)
	if err != nil {
		return nil, err
	}
	if rc == nil {
		return nil, nil
	}

	// Convert registry.RegistryComponent to providers.RegistryComponent
	component := &RegistryComponent{
		Name:        rc.Name,
		Type:        rc.Type,
		Language:    rc.Language,
		Description: rc.Description,
		Repository:  rc.Repository,
		RegistryURL: rc.RegistryURL,
		Homepage:    rc.Homepage,
		Tags:        rc.Tags,
		Maintainers: rc.Maintainers,
		License:     rc.License,
		LastUpdated: rc.LastUpdated,
		Metadata:    rc.Metadata,
	}

	return component, nil
}

// IsHealthy checks if the registry is accessible
func (p *GenericRegistryProvider) IsHealthy(ctx context.Context) bool {
	return true // Simple health check
}

// GenericPackageManagerProvider implements PackageManagerProvider for any language
type GenericPackageManagerProvider struct {
	language types.ComponentLanguage
}

// NewGenericPackageManagerProvider creates a new generic package manager provider
func NewGenericPackageManagerProvider(language types.ComponentLanguage) *GenericPackageManagerProvider {
	return &GenericPackageManagerProvider{
		language: language,
	}
}

// GetName returns the provider name
func (p *GenericPackageManagerProvider) GetName() string {
	return fmt.Sprintf("%s Package Manager Provider", strings.Title(string(p.language)))
}

// GetLanguage returns the language this package manager supports
func (p *GenericPackageManagerProvider) GetLanguage() types.ComponentLanguage {
	return p.language
}

// GetPackageManagerType returns the type of package manager
func (p *GenericPackageManagerProvider) GetPackageManagerType() string {
	// Map language to package manager
	packageManagers := map[types.ComponentLanguage]string{
		types.ComponentLanguageGo:     "go",
		types.ComponentLanguageJava:   "maven",
		types.ComponentLanguageCSharp: "nuget",
		types.ComponentLanguagePHP:    "composer",
		types.ComponentLanguageRuby:   "gem",
	}
	return packageManagers[p.language]
}

// GetPackage gets package metadata by name
func (p *GenericPackageManagerProvider) GetPackage(ctx context.Context, name string) (*PackageMetadata, error) {
	// This will be implemented to use the appropriate package manager
	// For now, return nil to avoid errors
	return nil, nil
}

// GetPackageVersion gets specific version metadata
func (p *GenericPackageManagerProvider) GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error) {
	// This will be implemented to use the appropriate package manager
	// For now, return nil to avoid errors
	return nil, nil
}

// GetLatestVersion gets the latest version of a package
func (p *GenericPackageManagerProvider) GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error) {
	// This will be implemented to use the appropriate package manager
	// For now, return nil to avoid errors
	return nil, nil
}

// IsHealthy checks if the package manager is accessible
func (p *GenericPackageManagerProvider) IsHealthy(ctx context.Context) bool {
	return true // Simple health check
}

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

	// Register generic providers for other languages
	languages := []types.ComponentLanguage{
		types.ComponentLanguageGo,
		types.ComponentLanguageJava,
		types.ComponentLanguageCSharp,
		types.ComponentLanguagePHP,
		types.ComponentLanguageRuby,
	}

	for _, lang := range languages {
		genericProvider := NewGenericLanguageProvider(lang)
		f.RegisterProvider(genericProvider)
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

	// Check if the provider implements RegistryProvider directly
	if registryProvider, ok := provider.(RegistryProvider); ok {
		return registryProvider, nil
	}

	// If not, try to get a composite provider
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

	// Check if the provider implements PackageManagerProvider directly
	if packageManagerProvider, ok := provider.(PackageManagerProvider); ok {
		return packageManagerProvider, nil
	}

	// If not, try to get a composite provider
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

// DiscoverComponents discovers all components for the language
func (p *CompositeProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// This would implement the logic to combine registry and package manager data
	// For now, return empty slice - will be implemented in the pipeline
	return []types.Component{}, nil
}

// GetComponentMetadata gets metadata for a specific component
func (p *CompositeProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	// This would implement the logic to combine registry and package manager data
	// For now, return nil - will be implemented in the pipeline
	return nil, nil
}

// GetComponentVersions gets versions for a specific component
func (p *CompositeProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// This would implement the logic to get versions from package manager
	// For now, return empty slice - will be implemented in the pipeline
	return []types.Version{}, nil
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

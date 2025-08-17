package providers

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/pkg/knowledge/registry"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// JavaScriptProvider implements the Provider interface for JavaScript
type JavaScriptProvider struct {
	registryClient *registry.Client
}

// NewJavaScriptProvider creates a new JavaScript provider
func NewJavaScriptProvider() *JavaScriptProvider {
	return &JavaScriptProvider{
		registryClient: registry.NewClient(),
	}
}

// GetName returns the provider name
func (p *JavaScriptProvider) GetName() string {
	return "JavaScript Provider"
}

// GetLanguage returns the language this provider supports
func (p *JavaScriptProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageJavaScript
}

// GetRegistryType returns the type of registry
func (p *JavaScriptProvider) GetRegistryType() string {
	return "opentelemetry"
}

// GetPackageManagerType returns the type of package manager
func (p *JavaScriptProvider) GetPackageManagerType() string {
	return "npm"
}

// DiscoverComponents discovers all JavaScript components
func (p *JavaScriptProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Component{}, nil
}

// GetComponentMetadata gets metadata for a specific JavaScript component
func (p *JavaScriptProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return nil
	return nil, nil
}

// GetComponentVersions gets versions for a specific JavaScript component
func (p *JavaScriptProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Version{}, nil
}

// IsHealthy checks if the provider is healthy
func (p *JavaScriptProvider) IsHealthy(ctx context.Context) bool {
	// Simple health check - could be enhanced
	return true
}

// GetRegistryProvider returns the registry provider
func (p *JavaScriptProvider) GetRegistryProvider() RegistryProvider {
	return &JavaScriptRegistryProvider{client: p.registryClient}
}

// GetPackageManagerProvider returns the package manager provider
func (p *JavaScriptProvider) GetPackageManagerProvider() PackageManagerProvider {
	return &JavaScriptPackageManagerProvider{} // No longer using npm client
}

// JavaScriptRegistryProvider implements RegistryProvider for JavaScript
type JavaScriptRegistryProvider struct {
	client *registry.Client
}

// GetName returns the provider name
func (p *JavaScriptRegistryProvider) GetName() string {
	return "JavaScript Registry Provider"
}

// GetLanguage returns the language this registry supports
func (p *JavaScriptRegistryProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageJavaScript
}

// GetRegistryType returns the type of registry
func (p *JavaScriptRegistryProvider) GetRegistryType() string {
	return "opentelemetry"
}

// DiscoverComponents discovers all components for JavaScript
func (p *JavaScriptRegistryProvider) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	components, err := p.client.GetComponentsByLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to discover components: %w", err)
	}

	// Convert to generic RegistryComponent
	var result []RegistryComponent
	for _, comp := range components {
		result = append(result, RegistryComponent{
			Name:        comp.Name,
			Type:        comp.Type,
			Language:    comp.Language,
			Description: comp.Description,
			Repository:  comp.Repository,
			RegistryURL: comp.RegistryURL,
			Homepage:    comp.Homepage,
			Tags:        comp.Tags,
			Maintainers: comp.Maintainers,
			License:     comp.License,
			LastUpdated: comp.LastUpdated,
			Metadata:    comp.Metadata,
		})
	}

	return result, nil
}

// GetComponentByName gets a specific component by name
func (p *JavaScriptRegistryProvider) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
	comp, err := p.client.GetComponentByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get component: %w", err)
	}

	if comp == nil {
		return nil, nil
	}

	result := &RegistryComponent{
		Name:        comp.Name,
		Type:        comp.Type,
		Language:    comp.Language,
		Description: comp.Description,
		Repository:  comp.Repository,
		RegistryURL: comp.RegistryURL,
		Homepage:    comp.Homepage,
		Tags:        comp.Tags,
		Maintainers: comp.Maintainers,
		License:     comp.License,
		LastUpdated: comp.LastUpdated,
		Metadata:    comp.Metadata,
	}

	return result, nil
}

// IsHealthy checks if the registry is accessible
func (p *JavaScriptRegistryProvider) IsHealthy(ctx context.Context) bool {
	// Simple health check - could be enhanced
	return true
}

// JavaScriptPackageManagerProvider implements PackageManagerProvider for JavaScript
type JavaScriptPackageManagerProvider struct {
	// No client needed for now since npm functionality is disabled
}

// GetName returns the provider name
func (p *JavaScriptPackageManagerProvider) GetName() string {
	return "JavaScript Package Manager Provider"
}

// GetLanguage returns the language this package manager supports
func (p *JavaScriptPackageManagerProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageJavaScript
}

// GetPackageManagerType returns the type of package manager
func (p *JavaScriptPackageManagerProvider) GetPackageManagerType() string {
	return "npm"
}

// GetPackage gets package metadata by name
func (p *JavaScriptPackageManagerProvider) GetPackage(ctx context.Context, name string) (*PackageMetadata, error) {
	// This function is no longer directly related to npm, but kept for potential future use or if the interface requires it.
	// For now, it will return nil as there's no npm client.
	return nil, fmt.Errorf("package manager functionality not available")
}

// GetPackageVersion gets specific version metadata
func (p *JavaScriptPackageManagerProvider) GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error) {
	// This function is no longer directly related to npm, but kept for potential future use or if the interface requires it.
	// For now, it will return nil as there's no npm client.
	return nil, fmt.Errorf("package manager functionality not available")
}

// GetLatestVersion gets the latest version of a package
func (p *JavaScriptPackageManagerProvider) GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error) {
	// This function is no longer directly related to npm, but kept for potential future use or if the interface requires it.
	// For now, it will return nil as there's no npm client.
	return nil, fmt.Errorf("package manager functionality not available")
}

// IsHealthy checks if the package manager is accessible
func (p *JavaScriptPackageManagerProvider) IsHealthy(ctx context.Context) bool {
	// Simple health check - could be enhanced
	return true
}

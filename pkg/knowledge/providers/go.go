package providers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/registry"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// GoProvider implements the Provider interface for Go
type GoProvider struct {
	registryClient       RegistryProvider
	packageManagerClient *GoModuleProxyClient
}

// NewGoProvider creates a new Go provider
func NewGoProvider() *GoProvider {
	return &GoProvider{
		registryClient:       NewGoRegistryProvider(),
		packageManagerClient: NewGoModuleProxyClient(),
	}
}

// GetName returns the provider name
func (p *GoProvider) GetName() string {
	return "Go Provider"
}

// GetLanguage returns the language this provider supports
func (p *GoProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageGo
}

// GetRegistryType returns the type of registry
func (p *GoProvider) GetRegistryType() string {
	return "opentelemetry"
}

// GetPackageManagerType returns the type of package manager
func (p *GoProvider) GetPackageManagerType() string {
	return "go"
}

// DiscoverComponents discovers all Go components
func (p *GoProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Component{}, nil
}

// GetComponentMetadata gets metadata for a specific Go component
func (p *GoProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return nil
	return nil, nil
}

// GetComponentVersions gets versions for a specific Go component
func (p *GoProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Version{}, nil
}

// IsHealthy checks if the provider is healthy
func (p *GoProvider) IsHealthy(ctx context.Context) bool {
	return p.registryClient.IsHealthy(ctx) && p.packageManagerClient.IsHealthy(ctx)
}

// GetPackageManagerProvider returns the package manager provider
func (p *GoProvider) GetPackageManagerProvider() PackageManagerProvider {
	return NewGoPackageManagerProvider()
}

// GoRegistryProvider implements RegistryProvider for Go OpenTelemetry registry
type GoRegistryProvider struct {
	registryClient *registry.Client
}

// NewGoRegistryProvider creates a new Go registry provider
func NewGoRegistryProvider() *GoRegistryProvider {
	return &GoRegistryProvider{
		registryClient: registry.NewClient(),
	}
}

// GetName returns the provider name
func (p *GoRegistryProvider) GetName() string {
	return "Go OpenTelemetry Registry"
}

// GetLanguage returns the language this registry supports
func (p *GoRegistryProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageGo
}

// GetRegistryType returns the type of registry
func (p *GoRegistryProvider) GetRegistryType() string {
	return "opentelemetry"
}

// DiscoverComponents discovers all Go components from the registry
func (p *GoRegistryProvider) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	components, err := p.registryClient.GetComponentsByLanguage(language)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Go components: %w", err)
	}

	var registryComponents []RegistryComponent
	for _, comp := range components {
		registryComponents = append(registryComponents, RegistryComponent{
			Name:        comp.Name,
			Description: comp.Description,
			Type:        comp.Type,
			Language:    comp.Language,
			Repository:  comp.Repository,
			License:     comp.License,
		})
	}

	return registryComponents, nil
}

// GetComponentByName gets a specific Go component by name
func (p *GoRegistryProvider) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
	component, err := p.registryClient.GetComponentByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch Go component %s: %w", name, err)
	}

	if component == nil {
		return nil, nil
	}

	return &RegistryComponent{
		Name:        component.Name,
		Description: component.Description,
		Type:        component.Type,
		Language:    component.Language,
		Repository:  component.Repository,
		License:     component.License,
	}, nil
}

// IsHealthy checks if the registry is accessible
func (p *GoRegistryProvider) IsHealthy(ctx context.Context) bool {
	// Test by fetching Go components
	_, err := p.registryClient.GetComponentsByLanguage("go")
	return err == nil
}

// GoPackageManagerProvider implements PackageManagerProvider for Go modules
type GoPackageManagerProvider struct {
	httpClient *http.Client
}

// NewGoPackageManagerProvider creates a new Go package manager provider
func NewGoPackageManagerProvider() *GoPackageManagerProvider {
	return &GoPackageManagerProvider{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *GoPackageManagerProvider) GetName() string {
	return "Go Module Proxy Provider"
}

// GetLanguage returns the language this package manager supports
func (p *GoPackageManagerProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguageGo
}

// GetPackageManagerType returns the type of package manager
func (p *GoPackageManagerProvider) GetPackageManagerType() string {
	return "go"
}

// GetPackage gets package metadata by name
func (p *GoPackageManagerProvider) GetPackage(ctx context.Context, name string) (*PackageMetadata, error) {
	return p.fetchGoModuleMetadata(ctx, name)
}

// GetPackageVersion gets specific version metadata
func (p *GoPackageManagerProvider) GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error) {
	packageData, err := p.fetchGoModuleMetadata(ctx, name)
	if err != nil {
		return nil, err
	}

	// Find the specific version
	for _, v := range packageData.Versions {
		if v.Version == version {
			return &v, nil
		}
	}

	return nil, fmt.Errorf("version %s not found for module %s", version, name)
}

// GetLatestVersion gets the latest version of a package
func (p *GoPackageManagerProvider) GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error) {
	packageData, err := p.fetchGoModuleMetadata(ctx, name)
	if err != nil {
		return nil, err
	}

	if len(packageData.Versions) == 0 {
		return nil, fmt.Errorf("no versions found for module %s", name)
	}

	// Get the first version from the map (latest)
	for _, version := range packageData.Versions {
		return &version, nil
	}

	return nil, fmt.Errorf("no versions found for module %s", name)
}

// IsHealthy checks if the package manager is accessible
func (p *GoPackageManagerProvider) IsHealthy(ctx context.Context) bool {
	// Test connection to Go module proxy
	req, err := http.NewRequestWithContext(ctx, "GET", "https://proxy.golang.org/", nil)
	if err != nil {
		return false
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

// fetchGoModuleMetadata fetches metadata for a Go module from the module proxy
func (p *GoPackageManagerProvider) fetchGoModuleMetadata(ctx context.Context, modulePath string) (*PackageMetadata, error) {
	// Escape module path for URL
	escapedPath := strings.ReplaceAll(modulePath, "/", "%2F")
	escapedPath = strings.ToLower(escapedPath)

	// Get list of versions
	versionsURL := fmt.Sprintf("https://proxy.golang.org/%s/@v/list", escapedPath)
	req, err := http.NewRequestWithContext(ctx, "GET", versionsURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request for %s: %w", modulePath, err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch versions for %s: %w", modulePath, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch versions for %s: HTTP %d", modulePath, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response for %s: %w", modulePath, err)
	}

	// Parse versions (one per line)
	versionLines := strings.Split(strings.TrimSpace(string(body)), "\n")
	if len(versionLines) == 0 || (len(versionLines) == 1 && versionLines[0] == "") {
		return nil, fmt.Errorf("no versions found for module %s", modulePath)
	}

	// Convert to PackageMetadata
	packageData := &PackageMetadata{
		Name:        modulePath,
		Description: fmt.Sprintf("Go module %s", modulePath),
		License:     "",
		Repository:  fmt.Sprintf("https://%s", modulePath),
		Homepage:    fmt.Sprintf("https://%s", modulePath),
		Versions:    make(map[string]VersionMetadata),
	}

	// Get metadata for each version (limit to avoid too many requests)
	maxVersions := 50
	if len(versionLines) > maxVersions {
		versionLines = versionLines[:maxVersions]
	}

	for _, version := range versionLines {
		if version == "" {
			continue
		}

		versionData := VersionMetadata{
			Version:      version,
			Dependencies: make(map[string]string),
		}

		packageData.Versions[version] = versionData
	}

	return packageData, nil
}

// GoModuleProxyClient provides access to Go module proxy
type GoModuleProxyClient struct {
	httpClient *http.Client
}

// NewGoModuleProxyClient creates a new Go module proxy client
func NewGoModuleProxyClient() *GoModuleProxyClient {
	return &GoModuleProxyClient{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// IsHealthy checks if the Go module proxy is accessible
func (c *GoModuleProxyClient) IsHealthy(ctx context.Context) bool {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://proxy.golang.org/", nil)
	if err != nil {
		return false
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK
}

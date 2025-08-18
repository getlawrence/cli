package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/getlawrence/cli/internal/logger"
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
		registryClient: registry.NewClient("", &logger.StdoutLogger{}, registry.RegistryBaseURL),
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
	return &JavaScriptPackageManagerProvider{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
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
	httpClient *http.Client
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
	url := fmt.Sprintf("https://registry.npmjs.org/%s", name)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package data: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("package not found: %s", name)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm registry returned status %d", resp.StatusCode)
	}

	var npmData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&npmData); err != nil {
		return nil, fmt.Errorf("failed to decode npm response: %w", err)
	}

	return p.convertNpmDataToPackageMetadata(npmData)
}

// GetPackageVersion gets specific version metadata
func (p *JavaScriptPackageManagerProvider) GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error) {
	packageData, err := p.GetPackage(ctx, name)
	if err != nil {
		return nil, err
	}

	versionData, exists := packageData.Versions[version]
	if !exists {
		return nil, fmt.Errorf("version %s not found for package %s", version, name)
	}

	return &versionData, nil
}

// GetLatestVersion gets the latest version of a package
func (p *JavaScriptPackageManagerProvider) GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error) {
	packageData, err := p.GetPackage(ctx, name)
	if err != nil {
		return nil, err
	}

	latestVersion := "latest"
	if packageData.DistTags != nil {
		if latest, exists := packageData.DistTags["latest"]; exists {
			latestVersion = latest
		}
	}

	versionData, exists := packageData.Versions[latestVersion]
	if !exists {
		return nil, fmt.Errorf("latest version not found for package %s", name)
	}

	return &versionData, nil
}

// IsHealthy checks if the package manager is accessible
func (p *JavaScriptPackageManagerProvider) IsHealthy(ctx context.Context) bool {
	// Simple health check - could be enhanced
	return true
}

// convertNpmDataToPackageMetadata converts npm registry data to PackageMetadata
func (p *JavaScriptPackageManagerProvider) convertNpmDataToPackageMetadata(npmData map[string]interface{}) (*PackageMetadata, error) {
	metadata := &PackageMetadata{
		Versions: make(map[string]VersionMetadata),
		Time:     make(map[string]time.Time),
	}

	// Extract basic package info
	if name, ok := npmData["name"].(string); ok {
		metadata.Name = name
	}

	if description, ok := npmData["description"].(string); ok {
		metadata.Description = description
	}

	if homepage, ok := npmData["homepage"].(string); ok {
		metadata.Homepage = homepage
	}

	if license, ok := npmData["license"].(string); ok {
		metadata.License = license
	}

	// Extract dist-tags
	if distTags, ok := npmData["dist-tags"].(map[string]interface{}); ok {
		metadata.DistTags = make(map[string]string)
		for tag, version := range distTags {
			if versionStr, ok := version.(string); ok {
				metadata.DistTags[tag] = versionStr
			}
		}
	}

	// Extract time information
	if timeData, ok := npmData["time"].(map[string]interface{}); ok {
		for version, timeStr := range timeData {
			if timeString, ok := timeStr.(string); ok {
				if parsedTime, err := time.Parse(time.RFC3339, timeString); err == nil {
					metadata.Time[version] = parsedTime
				}
			}
		}
	}

	// Extract versions
	if versions, ok := npmData["versions"].(map[string]interface{}); ok {
		for versionStr, versionData := range versions {
			if versionInfo, ok := versionData.(map[string]interface{}); ok {
				versionMetadata := p.convertVersionData(versionInfo)
				metadata.Versions[versionStr] = versionMetadata
			}
		}
	}

	return metadata, nil
}

// convertVersionData converts npm version data to VersionMetadata
func (p *JavaScriptPackageManagerProvider) convertVersionData(versionData map[string]interface{}) VersionMetadata {
	metadata := VersionMetadata{}

	if name, ok := versionData["name"].(string); ok {
		metadata.Name = name
	}

	if version, ok := versionData["version"].(string); ok {
		metadata.Version = version
	}

	if description, ok := versionData["description"].(string); ok {
		metadata.Description = description
	}

	if main, ok := versionData["main"].(string); ok {
		metadata.Main = main
	}

	if types, ok := versionData["types"].(string); ok {
		metadata.Types = types
	}

	if homepage, ok := versionData["homepage"].(string); ok {
		metadata.Homepage = homepage
	}

	if license, ok := versionData["license"].(string); ok {
		metadata.License = license
	}

	// Extract dependencies
	if deps, ok := versionData["dependencies"].(map[string]interface{}); ok {
		metadata.Dependencies = make(map[string]string)
		for name, version := range deps {
			if versionStr, ok := version.(string); ok {
				metadata.Dependencies[name] = versionStr
			}
		}
	}

	// Extract devDependencies
	if devDeps, ok := versionData["devDependencies"].(map[string]interface{}); ok {
		metadata.DevDependencies = make(map[string]string)
		for name, version := range devDeps {
			if versionStr, ok := version.(string); ok {
				metadata.DevDependencies[name] = versionStr
			}
		}
	}

	// Extract peerDependencies
	if peerDeps, ok := versionData["peerDependencies"].(map[string]interface{}); ok {
		metadata.PeerDependencies = make(map[string]string)
		for name, version := range peerDeps {
			if versionStr, ok := version.(string); ok {
				metadata.PeerDependencies[name] = versionStr
			}
		}
	}

	// Extract engines
	if engines, ok := versionData["engines"].(map[string]interface{}); ok {
		metadata.Engines = make(map[string]string)
		for name, version := range engines {
			if versionStr, ok := version.(string); ok {
				metadata.Engines[name] = versionStr
			}
		}
	}

	// Extract keywords
	if keywords, ok := versionData["keywords"].([]interface{}); ok {
		for _, keyword := range keywords {
			if keywordStr, ok := keyword.(string); ok {
				metadata.Keywords = append(metadata.Keywords, keywordStr)
			}
		}
	}

	return metadata
}

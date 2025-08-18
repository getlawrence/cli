package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/registry"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// PythonProvider implements the Provider interface for Python
type PythonProvider struct {
	registryClient       RegistryProvider
	packageManagerClient *PyPIClient
}

// NewPythonProvider creates a new Python provider
func NewPythonProvider() *PythonProvider {
	return &PythonProvider{
		registryClient:       NewPythonRegistryProvider(),
		packageManagerClient: NewPyPIClient(),
	}
}

// GetName returns the provider name
func (p *PythonProvider) GetName() string {
	return "Python Provider"
}

// GetLanguage returns the language this provider supports
func (p *PythonProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguagePython
}

// GetRegistryType returns the type of registry
func (p *PythonProvider) GetRegistryType() string {
	return "opentelemetry"
}

// GetPackageManagerType returns the type of package manager
func (p *PythonProvider) GetPackageManagerType() string {
	return "pypi"
}

// DiscoverComponents discovers all Python components
func (p *PythonProvider) DiscoverComponents(ctx context.Context) ([]types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Component{}, nil
}

// GetComponentMetadata gets metadata for a specific Python component
func (p *PythonProvider) GetComponentMetadata(ctx context.Context, name string) (*types.Component, error) {
	// This will be implemented to use the pipeline
	// For now, return nil
	return nil, nil
}

// GetComponentVersions gets versions for a specific Python component
func (p *PythonProvider) GetComponentVersions(ctx context.Context, name string) ([]types.Version, error) {
	// This will be implemented to use the pipeline
	// For now, return empty slice
	return []types.Version{}, nil
}

// IsHealthy checks if the provider is healthy
func (p *PythonProvider) IsHealthy(ctx context.Context) bool {
	return p.registryClient.IsHealthy(ctx) && p.packageManagerClient.IsHealthy(ctx)
}

// GetRegistryProvider returns the registry provider
func (p *PythonProvider) GetRegistryProvider() RegistryProvider {
	return p.registryClient
}

// GetPackageManagerProvider returns the package manager provider
func (p *PythonProvider) GetPackageManagerProvider() PackageManagerProvider {
	return p.packageManagerClient
}

// PythonRegistryProvider implements RegistryProvider for Python using the main registry client
type PythonRegistryProvider struct {
	client *registry.Client
}

// NewPythonRegistryProvider creates a new Python registry provider
func NewPythonRegistryProvider() *PythonRegistryProvider {
	return &PythonRegistryProvider{
		client: registry.NewClient("", &logger.StdoutLogger{}, registry.RegistryBaseURL),
	}
}

// GetName returns the provider name
func (p *PythonRegistryProvider) GetName() string {
	return "Python Registry Provider"
}

// GetLanguage returns the language this registry supports
func (p *PythonRegistryProvider) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguagePython
}

// GetRegistryType returns the type of registry
func (p *PythonRegistryProvider) GetRegistryType() string {
	return "opentelemetry"
}

// DiscoverComponents discovers all components for Python
func (p *PythonRegistryProvider) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	registryComponents, err := p.client.GetComponentsByLanguage("python")
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
func (p *PythonRegistryProvider) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
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
func (p *PythonRegistryProvider) IsHealthy(ctx context.Context) bool {
	return true // Simple health check
}

// PythonRegistryClient implements RegistryProvider for Python OpenTelemetry components
type PythonRegistryClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPythonRegistryClient creates a new Python registry client
func NewPythonRegistryClient() *PythonRegistryClient {
	return &PythonRegistryClient{
		baseURL: "https://registry.opentelemetry.io",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *PythonRegistryClient) GetName() string {
	return "Python Registry Provider"
}

// GetLanguage returns the language this registry supports
func (p *PythonRegistryClient) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguagePython
}

// GetRegistryType returns the type of registry
func (p *PythonRegistryClient) GetRegistryType() string {
	return "opentelemetry"
}

// DiscoverComponents discovers all components for Python
func (p *PythonRegistryClient) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	url := fmt.Sprintf("%s/api/v1/components?language=%s&per_page=100", p.baseURL, language)

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch components: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var response struct {
		Components []struct {
			Name        string                 `json:"name"`
			Type        string                 `json:"type"`
			Language    string                 `json:"language"`
			Description string                 `json:"description"`
			Repository  string                 `json:"repository"`
			RegistryURL string                 `json:"registry_url"`
			Homepage    string                 `json:"homepage"`
			Tags        []string               `json:"tags"`
			Maintainers []string               `json:"maintainers"`
			License     string                 `json:"license"`
			LastUpdated time.Time              `json:"last_updated"`
			Metadata    map[string]interface{} `json:"metadata"`
		} `json:"components"`
	}

	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	var components []RegistryComponent
	for _, comp := range response.Components {
		components = append(components, RegistryComponent{
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

	return components, nil
}

// GetComponentByName gets a specific component by name
func (p *PythonRegistryClient) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
	url := fmt.Sprintf("%s/api/v1/components/%s", p.baseURL, name)

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch component: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("registry returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var comp struct {
		Name        string                 `json:"name"`
		Type        string                 `json:"type"`
		Language    string                 `json:"language"`
		Description string                 `json:"description"`
		Repository  string                 `json:"repository"`
		RegistryURL string                 `json:"registry_url"`
		Homepage    string                 `json:"homepage"`
		Tags        []string               `json:"tags"`
		Maintainers []string               `json:"maintainers"`
		License     string                 `json:"license"`
		LastUpdated time.Time              `json:"last_updated"`
		Metadata    map[string]interface{} `json:"metadata"`
	}

	if err := json.Unmarshal(body, &comp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
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
func (p *PythonRegistryClient) IsHealthy(ctx context.Context) bool {
	resp, err := p.httpClient.Get(p.baseURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// PyPIClient implements PackageManagerProvider for PyPI
type PyPIClient struct {
	baseURL    string
	httpClient *http.Client
}

// NewPyPIClient creates a new PyPI client
func NewPyPIClient() *PyPIClient {
	return &PyPIClient{
		baseURL: "https://pypi.org/pypi",
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// GetName returns the provider name
func (p *PyPIClient) GetName() string {
	return "Python Package Manager Provider"
}

// GetLanguage returns the language this package manager supports
func (p *PyPIClient) GetLanguage() types.ComponentLanguage {
	return types.ComponentLanguagePython
}

// GetPackageManagerType returns the type of package manager
func (p *PyPIClient) GetPackageManagerType() string {
	return "pypi"
}

// GetPackage gets package metadata by name
func (p *PyPIClient) GetPackage(ctx context.Context, name string) (*PackageMetadata, error) {
	url := fmt.Sprintf("%s/%s/json", p.baseURL, name)

	resp, err := p.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PyPI returned status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var pypiPackage struct {
		Info struct {
			Name           string   `json:"name"`
			Version        string   `json:"version"`
			Summary        string   `json:"summary"`
			HomePage       string   `json:"home_page"`
			Author         string   `json:"author"`
			AuthorEmail    string   `json:"author_email"`
			License        string   `json:"license"`
			Keywords       []string `json:"keywords"`
			Platform       []string `json:"platform"`
			RequiresDist   []string `json:"requires_dist"`
			RequiresPython string   `json:"requires_python"`
		} `json:"info"`
		Releases map[string][]struct {
			URL           string `json:"url"`
			Size          int    `json:"size"`
			UploadTime    string `json:"upload_time"`
			PythonVersion string `json:"python_version"`
		} `json:"releases"`
		URLs []struct {
			URL           string `json:"url"`
			Size          int    `json:"size"`
			UploadTime    string `json:"upload_time"`
			PythonVersion string `json:"python_version"`
		} `json:"urls"`
	}

	if err := json.Unmarshal(body, &pypiPackage); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// Convert PyPI format to PackageMetadata
	result := &PackageMetadata{
		Name:        pypiPackage.Info.Name,
		Description: pypiPackage.Info.Summary,
		Version:     pypiPackage.Info.Version,
		Homepage:    pypiPackage.Info.HomePage,
		Author:      pypiPackage.Info.Author,
		License:     pypiPackage.Info.License,
		Keywords:    pypiPackage.Info.Keywords,
		OS:          pypiPackage.Info.Platform,
	}

	// Convert dependencies
	if len(pypiPackage.Info.RequiresDist) > 0 {
		result.Dependencies = make(map[string]string)
		for _, req := range pypiPackage.Info.RequiresDist {
			// Parse requirement string (e.g., "opentelemetry-api>=1.0.0")
			if strings.Contains(req, ">=") || strings.Contains(req, "==") || strings.Contains(req, "~=") {
				parts := strings.SplitN(req, ">=", 2)
				if len(parts) == 2 {
					result.Dependencies[parts[0]] = ">=" + parts[1]
				} else {
					parts = strings.SplitN(req, "==", 2)
					if len(parts) == 2 {
						result.Dependencies[parts[0]] = "==" + parts[1]
					} else {
						parts = strings.SplitN(req, "~=", 2)
						if len(parts) == 2 {
							result.Dependencies[parts[0]] = "~=" + parts[1]
						} else {
							result.Dependencies[req] = "*"
						}
					}
				}
			} else {
				result.Dependencies[req] = "*"
			}
		}
	}

	// Convert runtime requirements
	if pypiPackage.Info.RequiresPython != "" {
		result.Engines = map[string]string{
			"python": pypiPackage.Info.RequiresPython,
		}
	}

	// Convert versions
	if pypiPackage.Releases != nil {
		result.Versions = make(map[string]VersionMetadata)
		result.Time = make(map[string]time.Time)

		for versionStr, releases := range pypiPackage.Releases {
			if len(releases) > 0 {
				release := releases[0] // Use first release for metadata

				uploadTime, _ := time.Parse("2006-01-02T15:04:05", release.UploadTime)

				result.Versions[versionStr] = VersionMetadata{
					Name:        pypiPackage.Info.Name,
					Version:     versionStr,
					Description: pypiPackage.Info.Summary,
					Repository:  pypiPackage.Info.HomePage,
					Homepage:    pypiPackage.Info.HomePage,
					License:     pypiPackage.Info.License,
					Keywords:    pypiPackage.Info.Keywords,
					Author:      pypiPackage.Info.Author,
				}

				result.Time[versionStr] = uploadTime
			}
		}
	}

	return result, nil
}

// GetPackageVersion gets specific version metadata
func (p *PyPIClient) GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error) {
	// For PyPI, we get all versions in GetPackage, so we can extract the specific one
	packageData, err := p.GetPackage(ctx, name)
	if err != nil {
		return nil, err
	}

	if packageData == nil {
		return nil, nil
	}

	versionData, exists := packageData.Versions[version]
	if !exists {
		return nil, nil
	}

	return &versionData, nil
}

// GetLatestVersion gets the latest version of a package
func (p *PyPIClient) GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error) {
	packageData, err := p.GetPackage(ctx, name)
	if err != nil {
		return nil, err
	}

	if packageData == nil {
		return nil, nil
	}

	// Find the latest version by comparing timestamps
	var latestVersion string
	var latestTime time.Time

	for versionStr, uploadTime := range packageData.Time {
		if uploadTime.After(latestTime) {
			latestTime = uploadTime
			latestVersion = versionStr
		}
	}

	if latestVersion == "" {
		return nil, nil
	}

	versionData, exists := packageData.Versions[latestVersion]
	if !exists {
		return nil, nil
	}

	return &versionData, nil
}

// IsHealthy checks if the package manager is accessible
func (p *PyPIClient) IsHealthy(ctx context.Context) bool {
	resp, err := p.httpClient.Get("https://pypi.org/")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

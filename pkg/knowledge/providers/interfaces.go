package providers

import (
	"context"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// Provider represents a language/registry provider for OpenTelemetry components
type Provider interface {
	// Metadata about the provider
	GetName() string
	GetLanguage() types.ComponentLanguage
	GetRegistryType() string
	GetPackageManagerType() string

	// Core functionality
	DiscoverComponents(ctx context.Context) ([]types.Component, error)
	GetComponentMetadata(ctx context.Context, name string) (*types.Component, error)
	GetComponentVersions(ctx context.Context, name string) ([]types.Version, error)

	// Health check
	IsHealthy(ctx context.Context) bool
}

// RegistryProvider represents a registry for discovering OpenTelemetry components
type RegistryProvider interface {
	// GetName returns the provider name
	GetName() string

	// GetLanguage returns the language this registry supports
	GetLanguage() types.ComponentLanguage

	// GetRegistryType returns the type of registry (e.g., "opentelemetry", "community")
	GetRegistryType() string

	// DiscoverComponents discovers all components for the given language
	DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error)

	// GetComponentByName gets a specific component by name
	GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error)

	// IsHealthy checks if the registry is accessible
	IsHealthy(ctx context.Context) bool
}

// PackageManagerProvider represents a package manager for enriching component metadata
type PackageManagerProvider interface {
	// GetName returns the provider name
	GetName() string

	// GetLanguage returns the language this package manager supports
	GetLanguage() types.ComponentLanguage

	// GetPackageManagerType returns the type of package manager (e.g., "npm", "pypi", "maven")
	GetPackageManagerType() string

	// GetPackage gets package metadata by name
	GetPackage(ctx context.Context, name string) (*PackageMetadata, error)

	// GetPackageVersion gets specific version metadata
	GetPackageVersion(ctx context.Context, name, version string) (*VersionMetadata, error)

	// GetLatestVersion gets the latest version of a package
	GetLatestVersion(ctx context.Context, name string) (*VersionMetadata, error)

	// IsHealthy checks if the package manager is accessible
	IsHealthy(ctx context.Context) bool
}

// ProviderFactory creates and manages providers
type ProviderFactory interface {
	// GetProvider returns a provider for the specified language
	GetProvider(language types.ComponentLanguage) (Provider, error)

	// GetRegistryProvider returns a registry provider for the specified language
	GetRegistryProvider(language types.ComponentLanguage) (RegistryProvider, error)

	// GetPackageManagerProvider returns a package manager provider for the specified language
	GetPackageManagerProvider(language types.ComponentLanguage) (PackageManagerProvider, error)

	// ListSupportedLanguages returns all supported languages
	ListSupportedLanguages() []types.ComponentLanguage

	// RegisterProvider registers a custom provider
	RegisterProvider(provider Provider) error
}

// RegistryComponent represents a component from any registry
type RegistryComponent struct {
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

// PackageMetadata represents package metadata from any package manager
type PackageMetadata struct {
	Name                 string                     `json:"name"`
	Description          string                     `json:"description"`
	Version              string                     `json:"version"`
	Homepage             string                     `json:"homepage"`
	Repository           string                     `json:"repository"`
	Author               interface{}                `json:"author"`
	License              string                     `json:"license"`
	Keywords             []string                   `json:"keywords"`
	Main                 string                     `json:"main,omitempty"`
	Types                string                     `json:"types,omitempty"`
	Scripts              map[string]string          `json:"scripts,omitempty"`
	Dependencies         map[string]string          `json:"dependencies,omitempty"`
	DevDependencies      map[string]string          `json:"devDependencies,omitempty"`
	PeerDependencies     map[string]string          `json:"peerDependencies,omitempty"`
	OptionalDependencies map[string]string          `json:"optionalDependencies,omitempty"`
	Engines              map[string]string          `json:"engines,omitempty"`
	OS                   []string                   `json:"os,omitempty"`
	CPU                  []string                   `json:"cpu,omitempty"`
	DistTags             map[string]string          `json:"dist-tags,omitempty"`
	Time                 map[string]time.Time       `json:"time,omitempty"`
	Versions             map[string]VersionMetadata `json:"versions,omitempty"`
	Maintainers          []string                   `json:"maintainers,omitempty"`
	Contributors         []string                   `json:"contributors,omitempty"`
	Bugs                 string                     `json:"bugs,omitempty"`
	Readme               string                     `json:"readme,omitempty"`
	ID                   string                     `json:"_id,omitempty"`
	Rev                  string                     `json:"_rev,omitempty"`
}

// VersionMetadata represents version metadata from any package manager
type VersionMetadata struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Description          string            `json:"description"`
	Main                 string            `json:"main,omitempty"`
	Types                string            `json:"types,omitempty"`
	Scripts              map[string]string `json:"scripts,omitempty"`
	Dependencies         map[string]string `json:"dependencies,omitempty"`
	DevDependencies      map[string]string `json:"devDependencies,omitempty"`
	PeerDependencies     map[string]string `json:"peerDependencies,omitempty"`
	OptionalDependencies map[string]string `json:"optionalDependencies,omitempty"`
	Engines              map[string]string `json:"engines,omitempty"`
	OS                   []string          `json:"os,omitempty"`
	CPU                  []string          `json:"cpu,omitempty"`
	Repository           string            `json:"repository,omitempty"`
	Homepage             string            `json:"homepage,omitempty"`
	License              string            `json:"license,omitempty"`
	Keywords             []string          `json:"keywords,omitempty"`
	Author               interface{}       `json:"author,omitempty"`
	Maintainers          []string          `json:"maintainers,omitempty"`
	Contributors         []string          `json:"contributors,omitempty"`
	Bugs                 string            `json:"bugs,omitempty"`
	Readme               string            `json:"readme,omitempty"`
	ID                   string            `json:"_id,omitempty"`
	Rev                  string            `json:"_rev,omitempty"`
}

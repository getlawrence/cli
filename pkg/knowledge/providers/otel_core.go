package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// OTELCoreProvider implements both Provider and RegistryProvider to include both registry components and core packages
type OTELCoreProvider struct {
	language       types.ComponentLanguage
	registryClient *http.Client
	logger         logger.Logger
}

// NewOTELCoreProvider creates a new OTEL core provider
func NewOTELCoreProvider(language types.ComponentLanguage) *OTELCoreProvider {
	return &OTELCoreProvider{
		language:       language,
		registryClient: &http.Client{Timeout: 30 * time.Second},
		logger:         &logger.StdoutLogger{}, // Default logger
	}
}

// NewOTELCoreProviderWithLogger creates a new OTEL core provider with a custom logger
func NewOTELCoreProviderWithLogger(language types.ComponentLanguage, l logger.Logger) *OTELCoreProvider {
	return &OTELCoreProvider{
		language:       language,
		registryClient: &http.Client{Timeout: 30 * time.Second},
		logger:         l,
	}
}

// GetName returns the provider name
func (p *OTELCoreProvider) GetName() string {
	return fmt.Sprintf("OTEL Core Provider for %s", strings.Title(string(p.language)))
}

// GetLanguage returns the language this registry supports
func (p *OTELCoreProvider) GetLanguage() types.ComponentLanguage {
	return p.language
}

// GetRegistryType returns the type of registry
func (p *OTELCoreProvider) GetRegistryType() string {
	return "opentelemetry_core"
}

// DiscoverComponentsForRegistry discovers all components including core packages for the given language
func (p *OTELCoreProvider) DiscoverComponentsForRegistry(ctx context.Context, language string) ([]RegistryComponent, error) {
	var allComponents []RegistryComponent

	// Step 1: Get core packages dynamically
	coreComponents, err := p.discoverCorePackages()
	if err != nil {
		return nil, fmt.Errorf("failed to discover core packages: %w", err)
	}
	allComponents = append(allComponents, coreComponents...)

	// Step 2: Get registry components (if any)
	registryComponents, err := p.discoverRegistryComponents()
	if err != nil {
		// Log warning but continue with core packages
		p.logger.Logf("Warning: failed to discover registry components: %v\n", err)
	} else {
		allComponents = append(allComponents, registryComponents...)
	}

	return allComponents, nil
}

// DiscoverComponentsForRegistryInterface implements the RegistryProvider interface
func (p *OTELCoreProvider) DiscoverComponents(ctx context.Context, language string) ([]RegistryComponent, error) {
	return p.DiscoverComponentsForRegistry(ctx, language)
}

// discoverCorePackages discovers core OTEL packages for the language dynamically
func (p *OTELCoreProvider) discoverCorePackages() ([]RegistryComponent, error) {
	langStr := string(p.language)
	var components []RegistryComponent

	// Define core packages for each language
	corePackages := p.getCorePackagesForLanguage(langStr)

	// Discover each core package
	for _, pkg := range corePackages {
		component, err := p.discoverPackage(pkg.Name, pkg.Type)
		if err != nil {
			// Log warning but continue with other packages
			p.logger.Logf("Warning: failed to discover package %s: %v\n", pkg.Name, err)
			continue
		}
		components = append(components, component)
	}

	return components, nil
}

// CorePackage represents a core OTEL package
type CorePackage struct {
	Name            string
	Type            string
	MinVersion      string
	MaxVersion      string
	Stability       string // stable, experimental, deprecated
	Lifecycle       string // alpha, beta, stable, deprecated
	SpecCompliance  string // v1.0, v1.1, v1.2, etc.
	BreakingChanges []string
}

// PackageVersionInfo represents detailed version information
type PackageVersionInfo struct {
	Version         string
	ReleaseDate     time.Time
	Stability       string
	Lifecycle       string
	SpecCompliance  string
	BreakingChanges []string
	Deprecated      bool
	MinCompatible   map[string]string // e.g., {"api": "1.0.0", "sdk": "1.0.0"}
}

// getCorePackagesForLanguage returns the core packages that should be discovered for a language
func (p *OTELCoreProvider) getCorePackagesForLanguage(language string) []CorePackage {
	switch language {
	case "javascript":
		return []CorePackage{
			{Name: "@opentelemetry/api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/sdk-node", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/sdk-web", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/exporter-trace-otlp-http", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/exporter-trace-otlp-grpc", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/exporter-trace-otlp-proto", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/propagator-b3", Type: "propagator", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/propagator-jaeger", Type: "propagator", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "@opentelemetry/propagator-w3c", Type: "propagator", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "python":
		return []CorePackage{
			{Name: "opentelemetry-api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-exporter-otlp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-exporter-jaeger", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-exporter-zipkin", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "go":
		return []CorePackage{
			{Name: "go.opentelemetry.io/otel", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/trace", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/metrics", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/logs", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/propagation", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/semconv/v1.34.0", Type: "semconv", MinVersion: "1.34.0", MaxVersion: "1.34.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.34.0", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/jaeger", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/zipkin", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/exporters/prometheus", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/sdk/resource", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/sdk/trace", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/sdk/metrics", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "go.opentelemetry.io/otel/sdk/logs", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "java":
		return []CorePackage{
			{Name: "io.opentelemetry:opentelemetry-api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-sdk-extension-autoconfigure", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-exporter-otlp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-exporter-jaeger", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-exporter-zipkin", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-extension-trace-propagators", Type: "propagator", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "io.opentelemetry:opentelemetry-instrumentation-api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "csharp", "dotnet":
		return []CorePackage{
			{Name: "OpenTelemetry", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "OpenTelemetry.Api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "OpenTelemetry.Sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "OpenTelemetry.Extensions.Hosting", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "php":
		return []CorePackage{
			{Name: "open-telemetry/api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "open-telemetry/sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "open-telemetry/exporter-otlp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	case "ruby":
		return []CorePackage{
			{Name: "opentelemetry-api", Type: "api", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-sdk", Type: "sdk", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
			{Name: "opentelemetry-exporter-otlp", Type: "exporter", MinVersion: "1.0.0", MaxVersion: "2.0.0", Stability: "stable", Lifecycle: "stable", SpecCompliance: "v1.0+", BreakingChanges: []string{}},
		}
	default:
		return []CorePackage{}
	}
}

// discoverPackage discovers information about a specific package
func (p *OTELCoreProvider) discoverPackage(packageName, componentType string) (RegistryComponent, error) {
	langStr := string(p.language)

	// Try to fetch package metadata from the appropriate package registry
	packageData, err := p.fetchPackageMetadata(packageName)

	// Get core package information
	corePackages := p.getCorePackagesForLanguage(langStr)
	var corePackage *CorePackage
	for _, cp := range corePackages {
		if cp.Name == packageName {
			corePackage = &cp
			break
		}
	}

	component := RegistryComponent{
		Name:        packageName,
		Type:        p.mapComponentType(componentType),
		Language:    langStr,
		Description: p.generateDescription(packageName, componentType),
		Repository:  p.determineRepository(packageName),
		RegistryURL: p.generateRegistryURL(packageName),
		Homepage:    p.determineRepository(packageName),
		Tags:        p.generateTags(componentType),
		Maintainers: p.generateMaintainers(),
		License:     "Apache-2.0", // OpenTelemetry uses Apache 2.0
		LastUpdated: time.Now(),
		Metadata: map[string]interface{}{
			"isCorePackage": true,
			"packageName":   packageName,
			"componentType": componentType,
			"source":        "dynamic_discovery",
		},
	}

	// Add version and lifecycle information if available
	if corePackage != nil {
		component.Metadata["minVersion"] = corePackage.MinVersion
		component.Metadata["maxVersion"] = corePackage.MaxVersion
		component.Metadata["stability"] = corePackage.Stability
		component.Metadata["lifecycle"] = corePackage.Lifecycle
		component.Metadata["specCompliance"] = corePackage.SpecCompliance
		component.Metadata["breakingChanges"] = corePackage.BreakingChanges
	}

	// If we successfully fetched package data, enrich the component
	if err == nil && packageData != nil {
		component.Description = packageData.Description
		component.Homepage = packageData.Homepage
		component.License = packageData.License
		component.Maintainers = packageData.Maintainers
		component.Metadata["packageData"] = packageData
	}

	return component, nil
}

// fetchPackageMetadata fetches package metadata from the appropriate package registry
func (p *OTELCoreProvider) fetchPackageMetadata(packageName string) (*PackageMetadata, error) {
	langStr := string(p.language)

	switch langStr {
	case "javascript":
		return p.fetchNpmPackageMetadata(packageName)
	case "python":
		return p.fetchPyPIPackageMetadata(packageName)
	case "go":
		return p.fetchGoPackageMetadata(packageName)
	case "java":
		return p.fetchMavenPackageMetadata(packageName)
	case "csharp", "dotnet":
		return p.fetchNuGetPackageMetadata(packageName)
	case "php":
		return p.fetchComposerPackageMetadata(packageName)
	case "ruby":
		return p.fetchRubyGemsPackageMetadata(packageName)
	default:
		return nil, fmt.Errorf("unsupported language for package metadata: %s", langStr)
	}
}

// fetchNpmPackageMetadata fetches package metadata from npm
func (p *OTELCoreProvider) fetchNpmPackageMetadata(packageName string) (*PackageMetadata, error) {
	url := fmt.Sprintf("https://registry.npmjs.org/%s", packageName)
	resp, err := p.registryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("npm registry returned status %d", resp.StatusCode)
	}

	var npmData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&npmData); err != nil {
		return nil, err
	}

	// Extract latest version data
	latestVersion := "latest"
	if distTags, ok := npmData["dist-tags"].(map[string]interface{}); ok {
		if latest, ok := distTags["latest"].(string); ok {
			latestVersion = latest
		}
	}

	latestData, ok := npmData["versions"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("no versions found")
	}

	versionData, ok := latestData[latestVersion].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("latest version data not found")
	}

	// Convert to PackageMetadata
	metadata := &PackageMetadata{
		Name:        packageName,
		Description: p.extractString(versionData, "description"),
		Version:     latestVersion,
		Homepage:    p.extractString(versionData, "homepage"),
		Repository:  p.extractString(versionData, "repository"),
		License:     p.extractString(versionData, "license"),
		Maintainers: p.extractStringSlice(versionData, "maintainers"),
	}

	return metadata, nil
}

// fetchPyPIPackageMetadata fetches package metadata from PyPI
func (p *OTELCoreProvider) fetchPyPIPackageMetadata(packageName string) (*PackageMetadata, error) {
	url := fmt.Sprintf("https://pypi.org/pypi/%s/json", packageName)
	resp, err := p.registryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("PyPI returned status %d", resp.StatusCode)
	}

	var pypiData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&pypiData); err != nil {
		return nil, err
	}

	info, ok := pypiData["info"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid PyPI response structure")
	}

	metadata := &PackageMetadata{
		Name:        packageName,
		Description: p.extractString(info, "summary"),
		Version:     p.extractString(info, "version"),
		Homepage:    p.extractString(info, "home_page"),
		License:     p.extractString(info, "license"),
		Maintainers: []string{}, // PyPI doesn't have maintainers in the same way
	}

	return metadata, nil
}

// fetchGoPackageMetadata fetches package metadata from Go modules
func (p *OTELCoreProvider) fetchGoPackageMetadata(packageName string) (*PackageMetadata, error) {
	// Go modules don't have a centralized registry like npm/PyPI
	// We'll create basic metadata based on the package name
	metadata := &PackageMetadata{
		Name:        packageName,
		Description: fmt.Sprintf("Go OpenTelemetry package: %s", packageName),
		Version:     "latest",
		Homepage:    p.determineRepository(packageName),
		License:     "Apache-2.0",
		Maintainers: []string{"OpenTelemetry", "CNCF"},
	}

	return metadata, nil
}

// fetchMavenPackageMetadata fetches package metadata from Maven Central
func (p *OTELCoreProvider) fetchMavenPackageMetadata(packageName string) (*PackageMetadata, error) {
	// Maven Central has a REST API but it's more complex
	// For now, return basic metadata
	metadata := &PackageMetadata{
		Name:        packageName,
		Description: fmt.Sprintf("Java OpenTelemetry package: %s", packageName),
		Version:     "latest",
		Homepage:    p.determineRepository(packageName),
		License:     "Apache-2.0",
		Maintainers: []string{"OpenTelemetry", "CNCF"},
	}

	return metadata, nil
}

// fetchNuGetPackageMetadata fetches package metadata from NuGet
func (p *OTELCoreProvider) fetchNuGetPackageMetadata(packageName string) (*PackageMetadata, error) {
	url := fmt.Sprintf("https://api.nuget.org/v3/registration5-semver1/%s/index.json", packageName)
	resp, err := p.registryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("NuGet returned status %d", resp.StatusCode)
	}

	var nugetData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&nugetData); err != nil {
		return nil, err
	}

	// Extract basic information
	metadata := &PackageMetadata{
		Name:        packageName,
		Description: fmt.Sprintf(".NET OpenTelemetry package: %s", packageName),
		Version:     "latest",
		Homepage:    p.determineRepository(packageName),
		License:     "Apache-2.0",
		Maintainers: []string{"OpenTelemetry", "CNCF"},
	}

	return metadata, nil
}

// fetchComposerPackageMetadata fetches package metadata from Composer
func (p *OTELCoreProvider) fetchComposerPackageMetadata(packageName string) (*PackageMetadata, error) {
	url := fmt.Sprintf("https://packagist.org/packages/%s.json", packageName)
	resp, err := p.registryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Composer returned status %d", resp.StatusCode)
	}

	var composerData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&composerData); err != nil {
		return nil, err
	}

	packageInfo, ok := composerData["package"].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid Composer response structure")
	}

	metadata := &PackageMetadata{
		Name:        packageName,
		Description: p.extractString(packageInfo, "description"),
		Version:     "latest",
		Homepage:    p.extractString(packageInfo, "homepage"),
		License:     p.extractString(packageInfo, "license"),
		Maintainers: []string{}, // Composer doesn't have maintainers in the same way
	}

	return metadata, nil
}

// fetchRubyGemsPackageMetadata fetches package metadata from RubyGems
func (p *OTELCoreProvider) fetchRubyGemsPackageMetadata(packageName string) (*PackageMetadata, error) {
	url := fmt.Sprintf("https://rubygems.org/api/v1/gems/%s.json", packageName)
	resp, err := p.registryClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("RubyGems returned status %d", resp.StatusCode)
	}

	var gemData map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&gemData); err != nil {
		return nil, err
	}

	metadata := &PackageMetadata{
		Name:        packageName,
		Description: p.extractString(gemData, "info"),
		Version:     p.extractString(gemData, "version"),
		Homepage:    p.extractString(gemData, "homepage_uri"),
		License:     p.extractString(gemData, "license"),
		Maintainers: []string{}, // RubyGems doesn't have maintainers in the same way
	}

	return metadata, nil
}

// Helper functions for extracting data from JSON responses
func (p *OTELCoreProvider) extractString(data map[string]interface{}, key string) string {
	if value, ok := data[key].(string); ok {
		return value
	}
	return ""
}

func (p *OTELCoreProvider) extractStringSlice(data map[string]interface{}, key string) []string {
	if value, ok := data[key].([]interface{}); ok {
		var result []string
		for _, item := range value {
			if str, ok := item.(string); ok {
				result = append(result, str)
			}
		}
		return result
	}
	return []string{}
}

// createCoreComponent creates a RegistryComponent for a core package
func (p *OTELCoreProvider) createCoreComponent(packageName, componentType string) RegistryComponent {
	// Determine the repository based on package name and language
	repository := p.determineRepository(packageName)

	// Determine the component type
	compType := p.mapComponentType(componentType)

	return RegistryComponent{
		Name:        packageName,
		Type:        compType,
		Language:    string(p.language),
		Description: p.generateDescription(packageName, componentType),
		Repository:  repository,
		RegistryURL: p.generateRegistryURL(packageName),
		Homepage:    repository,
		Tags:        p.generateTags(componentType),
		Maintainers: p.generateMaintainers(),
		License:     "Apache-2.0", // OpenTelemetry uses Apache 2.0
		LastUpdated: time.Now(),
		Metadata: map[string]interface{}{
			"isCorePackage": true,
			"packageName":   packageName,
			"componentType": componentType,
			"source":        "core_packages",
		},
	}
}

// determineRepository determines the repository URL for a package
func (p *OTELCoreProvider) determineRepository(packageName string) string {
	langStr := string(p.language)

	// Map common patterns to repositories
	switch langStr {
	case "javascript":
		if strings.HasPrefix(packageName, "@opentelemetry/") {
			return fmt.Sprintf("https://github.com/open-telemetry/opentelemetry-js/tree/main/packages/%s",
				strings.TrimPrefix(packageName, "@opentelemetry/"))
		}
	case "python":
		if strings.HasPrefix(packageName, "opentelemetry-") {
			return fmt.Sprintf("https://github.com/open-telemetry/opentelemetry-python/tree/main/%s", packageName)
		}
	case "go":
		if strings.HasPrefix(packageName, "go.opentelemetry.io/") {
			return "https://github.com/open-telemetry/opentelemetry-go"
		}
	case "java":
		if strings.Contains(packageName, "opentelemetry") {
			return "https://github.com/open-telemetry/opentelemetry-java"
		}
	case "csharp", "dotnet":
		if strings.Contains(packageName, "OpenTelemetry") {
			return "https://github.com/open-telemetry/opentelemetry-dotnet"
		}
	case "php":
		if strings.Contains(packageName, "open-telemetry") {
			return "https://github.com/open-telemetry/opentelemetry-php"
		}
	case "ruby":
		if strings.Contains(packageName, "opentelemetry") {
			return "https://github.com/open-telemetry/opentelemetry-ruby"
		}
	}

	// Default fallback
	return "https://github.com/open-telemetry"
}

// generateRegistryURL generates a registry URL for the package
func (p *OTELCoreProvider) generateRegistryURL(packageName string) string {
	langStr := string(p.language)

	switch langStr {
	case "javascript":
		return fmt.Sprintf("https://www.npmjs.com/package/%s", packageName)
	case "python":
		return fmt.Sprintf("https://pypi.org/project/%s/", packageName)
	case "go":
		return fmt.Sprintf("https://pkg.go.dev/%s", packageName)
	case "java":
		return fmt.Sprintf("https://search.maven.org/artifact/%s", packageName)
	case "csharp", "dotnet":
		return fmt.Sprintf("https://www.nuget.org/packages/%s", packageName)
	case "php":
		return fmt.Sprintf("https://packagist.org/packages/%s", packageName)
	case "ruby":
		return fmt.Sprintf("https://rubygems.org/gems/%s", packageName)
	default:
		return ""
	}
}

// generateDescription generates a description for the component
func (p *OTELCoreProvider) generateDescription(packageName, componentType string) string {
	langStr := string(p.language)

	switch componentType {
	case "core":
		return fmt.Sprintf("OpenTelemetry %s core package for %s", strings.Title(langStr), packageName)
	case "exporter":
		return fmt.Sprintf("OpenTelemetry %s exporter for %s", strings.Title(langStr), packageName)
	case "propagator":
		return fmt.Sprintf("OpenTelemetry %s propagator for %s", strings.Title(langStr), packageName)
	case "sampler":
		return fmt.Sprintf("OpenTelemetry %s sampler for %s", strings.Title(langStr), packageName)
	default:
		return fmt.Sprintf("OpenTelemetry %s %s package", strings.Title(langStr), componentType)
	}
}

// generateTags generates tags for the component
func (p *OTELCoreProvider) generateTags(componentType string) []string {
	tags := []string{"opentelemetry", "observability", "telemetry"}

	switch componentType {
	case "core":
		tags = append(tags, "core", "sdk", "api")
	case "exporter":
		tags = append(tags, "exporter", "backend")
	case "propagator":
		tags = append(tags, "propagator", "context")
	case "sampler":
		tags = append(tags, "sampler", "sampling")
	}

	return tags
}

// generateMaintainers generates maintainer information
func (p *OTELCoreProvider) generateMaintainers() []string {
	return []string{"OpenTelemetry", "CNCF"}
}

// mapComponentType maps component type to registry component type
func (p *OTELCoreProvider) mapComponentType(componentType string) string {
	switch strings.ToLower(componentType) {
	case "core":
		return "sdk"
	case "api":
		return "api"
	case "sdk":
		return "sdk"
	case "exporter":
		return "exporter"
	case "propagator":
		return "propagator"
	case "sampler":
		return "sampler"
	case "processor":
		return "processor"
	case "resource":
		return "resource"
	case "instrumentation":
		return "instrumentation"
	default:
		return "component"
	}
}

// discoverRegistryComponents discovers components from the registry (if available)
func (p *OTELCoreProvider) discoverRegistryComponents() ([]RegistryComponent, error) {
	// For now, return empty slice as we're focusing on core packages
	// This could be enhanced to also fetch from the registry
	return []RegistryComponent{}, nil
}

// GetComponentByName gets a specific component by name
func (p *OTELCoreProvider) GetComponentByName(ctx context.Context, name string) (*RegistryComponent, error) {
	// Discover all components and find the one with matching name
	components, err := p.DiscoverComponentsForRegistry(ctx, string(p.language))
	if err != nil {
		return nil, err
	}

	for _, component := range components {
		if component.Name == name {
			return &component, nil
		}
	}

	return nil, nil
}

// IsHealthy checks if the registry is accessible
func (p *OTELCoreProvider) IsHealthy(ctx context.Context) bool {
	// Test if we can access a common package registry to verify connectivity
	// For now, test npm registry as it's commonly accessible
	testURL := "https://registry.npmjs.org/@opentelemetry/api"
	resp, err := p.registryClient.Get(testURL)
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

// VersionCompatibilityMatrix tracks compatibility between different core packages
type VersionCompatibilityMatrix struct {
	Language           string
	APIVersion         string
	SDKVersion         string
	SemConvVersion     string
	ExporterVersions   map[string]string
	PropagatorVersions map[string]string
	Compatible         bool
	Notes              string
}

// getVersionCompatibilityMatrix returns compatibility information for a language
func (p *OTELCoreProvider) getVersionCompatibilityMatrix(language string) []VersionCompatibilityMatrix {
	switch language {
	case "javascript":
		return []VersionCompatibilityMatrix{
			{
				Language:       "javascript",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp-http": "1.0.0",
					"otlp-grpc": "1.0.0",
				},
				PropagatorVersions: map[string]string{
					"b3":  "1.0.0",
					"w3c": "1.0.0",
				},
				Compatible: true,
				Notes:      "All packages in 1.x series are compatible",
			},
		}
	case "python":
		return []VersionCompatibilityMatrix{
			{
				Language:       "python",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp":   "1.0.0",
					"jaeger": "1.0.0",
					"zipkin": "1.0.0",
				},
				PropagatorVersions: map[string]string{},
				Compatible:         true,
				Notes:              "All packages in 1.x series are compatible",
			},
		}
	case "go":
		return []VersionCompatibilityMatrix{
			{
				Language:       "go",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.34.0",
				ExporterVersions: map[string]string{
					"otlp-trace-http":  "1.0.0",
					"otlp-trace-grpc":  "1.0.0",
					"otlp-metric-http": "1.0.0",
					"otlp-metric-grpc": "1.0.0",
					"otlp-log-http":    "1.0.0",
					"otlp-log-grpc":    "1.0.0",
					"jaeger":           "1.0.0",
					"zipkin":           "1.0.0",
					"prometheus":       "1.0.0",
				},
				PropagatorVersions: map[string]string{},
				Compatible:         true,
				Notes:              "All packages in 1.x series are compatible",
			},
		}
	case "java":
		return []VersionCompatibilityMatrix{
			{
				Language:       "java",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp":   "1.0.0",
					"jaeger": "1.0.0",
					"zipkin": "1.0.0",
				},
				PropagatorVersions: map[string]string{
					"trace-propagators": "1.0.0",
				},
				Compatible: true,
				Notes:      "All packages in 1.x series are compatible",
			},
		}
	case "csharp", "dotnet":
		return []VersionCompatibilityMatrix{
			{
				Language:       "csharp",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp": "1.0.0",
				},
				PropagatorVersions: map[string]string{},
				Compatible:         true,
				Notes:              "All packages in 1.x series are compatible",
			},
		}
	case "php":
		return []VersionCompatibilityMatrix{
			{
				Language:       "php",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp": "1.0.0",
				},
				PropagatorVersions: map[string]string{},
				Compatible:         true,
				Notes:              "All packages in 1.x series are compatible",
			},
		}
	case "ruby":
		return []VersionCompatibilityMatrix{
			{
				Language:       "ruby",
				APIVersion:     "1.0.0",
				SDKVersion:     "1.0.0",
				SemConvVersion: "1.0.0",
				ExporterVersions: map[string]string{
					"otlp": "1.0.0",
				},
				PropagatorVersions: map[string]string{},
				Compatible:         true,
				Notes:              "All packages in 1.x series are compatible",
			},
		}
	default:
		return []VersionCompatibilityMatrix{}
	}
}

// CheckVersionCompatibility checks if a set of package versions are compatible
func (p *OTELCoreProvider) CheckVersionCompatibility(packages map[string]string) (bool, []string) {
	langStr := string(p.language)
	compatibilityMatrix := p.getVersionCompatibilityMatrix(langStr)

	if len(compatibilityMatrix) == 0 {
		return false, []string{"No compatibility matrix available for language: " + langStr}
	}

	var issues []string

	// Check against the first compatibility matrix (assuming single matrix per language)
	matrix := compatibilityMatrix[0]

	// Check API version
	if apiVer, exists := packages["api"]; exists {
		if !p.isVersionCompatible(apiVer, matrix.APIVersion) {
			issues = append(issues, fmt.Sprintf("API version %s is not compatible with expected version %s", apiVer, matrix.APIVersion))
		}
	}

	// Check SDK version
	if sdkVer, exists := packages["sdk"]; exists {
		if !p.isVersionCompatible(sdkVer, matrix.SDKVersion) {
			issues = append(issues, fmt.Sprintf("SDK version %s is not compatible with expected version %s", sdkVer, matrix.SDKVersion))
		}
	}

	// Check semantic conventions version
	if semConvVer, exists := packages["semconv"]; exists {
		if !p.isVersionCompatible(semConvVer, matrix.SemConvVersion) {
			issues = append(issues, fmt.Sprintf("Semantic conventions version %s is not compatible with expected version %s", semConvVer, matrix.SemConvVersion))
		}
	}

	return len(issues) == 0, issues
}

// isVersionCompatible checks if two version strings are compatible
func (p *OTELCoreProvider) isVersionCompatible(actual, expected string) bool {
	// Simple compatibility check: major versions should match
	// This is a simplified check - in practice, you'd want more sophisticated version parsing
	if strings.HasPrefix(actual, "1.") && strings.HasPrefix(expected, "1.") {
		return true
	}
	if strings.HasPrefix(actual, "2.") && strings.HasPrefix(expected, "2.") {
		return true
	}
	return actual == expected
}

// SpecificationCompliance tracks compliance with OpenTelemetry specifications
type SpecificationCompliance struct {
	SpecVersion     string // e.g., "v1.0", "v1.1", "v1.2"
	Language        string
	ComplianceLevel string   // "full", "partial", "minimal"
	Features        []string // List of implemented features
	MissingFeatures []string // List of missing features
	Notes           string
}

// getSpecificationCompliance returns compliance information for a language
func (p *OTELCoreProvider) getSpecificationCompliance(language string) []SpecificationCompliance {
	switch language {
	case "javascript":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "javascript",
				ComplianceLevel: "full",
				Features: []string{
					"traces", "metrics", "logs", "context_propagation",
					"sampling", "resource_detection", "exporters", "propagators",
				},
				MissingFeatures: []string{},
				Notes:           "Full compliance with OpenTelemetry v1.0 specification",
			},
		}
	case "python":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "python",
				ComplianceLevel: "full",
				Features: []string{
					"traces", "metrics", "logs", "context_propagation",
					"sampling", "resource_detection", "exporters",
				},
				MissingFeatures: []string{},
				Notes:           "Full compliance with OpenTelemetry v1.0 specification",
			},
		}
	case "go":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "go",
				ComplianceLevel: "full",
				Features: []string{
					"traces", "metrics", "logs", "context_propagation",
					"sampling", "resource_detection", "exporters", "propagators",
					"semantic_conventions", "attribute_utilities", "status_codes",
				},
				MissingFeatures: []string{},
				Notes:           "Full compliance with OpenTelemetry v1.0 specification",
			},
		}
	case "java":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "java",
				ComplianceLevel: "full",
				Features: []string{
					"traces", "metrics", "logs", "context_propagation",
					"sampling", "resource_detection", "exporters", "propagators",
					"auto_configuration", "instrumentation_api",
				},
				MissingFeatures: []string{},
				Notes:           "Full compliance with OpenTelemetry v1.0 specification",
			},
		}
	case "csharp", "dotnet":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "csharp",
				ComplianceLevel: "full",
				Features: []string{
					"traces", "metrics", "logs", "context_propagation",
					"sampling", "resource_detection", "exporters", "hosting_extensions",
				},
				MissingFeatures: []string{},
				Notes:           "Full compliance with OpenTelemetry v1.0 specification",
			},
		}
	case "php":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "php",
				ComplianceLevel: "partial",
				Features: []string{
					"traces", "metrics", "logs", "exporters",
				},
				MissingFeatures: []string{
					"context_propagation", "propagators", "semantic_conventions",
				},
				Notes: "Partial compliance - missing some advanced features",
			},
		}
	case "ruby":
		return []SpecificationCompliance{
			{
				SpecVersion:     "v1.0",
				Language:        "ruby",
				ComplianceLevel: "partial",
				Features: []string{
					"traces", "metrics", "logs", "exporters",
				},
				MissingFeatures: []string{
					"context_propagation", "propagators", "semantic_conventions",
				},
				Notes: "Partial compliance - missing some advanced features",
			},
		}
	default:
		return []SpecificationCompliance{}
	}
}

// CheckSpecificationCompliance checks if packages comply with a specific OTEL spec version
func (p *OTELCoreProvider) CheckSpecificationCompliance(specVersion string) (*SpecificationCompliance, error) {
	langStr := string(p.language)
	complianceList := p.getSpecificationCompliance(langStr)

	if len(complianceList) == 0 {
		return nil, fmt.Errorf("no compliance information available for language: %s", langStr)
	}

	// Find the best matching spec version
	var bestMatch *SpecificationCompliance
	for _, compliance := range complianceList {
		if compliance.SpecVersion == specVersion {
			bestMatch = &compliance
			break
		}
		// If no exact match, use the first available (assuming it's the most recent)
		if bestMatch == nil {
			bestMatch = &compliance
		}
	}

	return bestMatch, nil
}

// GetComplianceReport generates a comprehensive compliance report for a language
func (p *OTELCoreProvider) GetComplianceReport() map[string]interface{} {
	langStr := string(p.language)

	report := map[string]interface{}{
		"language":                 langStr,
		"timestamp":                time.Now(),
		"core_packages":            p.getCorePackagesForLanguage(langStr),
		"version_compatibility":    p.getVersionCompatibilityMatrix(langStr),
		"specification_compliance": p.getSpecificationCompliance(langStr),
		"summary": map[string]interface{}{
			"total_core_packages":  len(p.getCorePackagesForLanguage(langStr)),
			"compatibility_status": "stable",
			"compliance_level":     "full",
		},
	}

	// Calculate summary statistics
	complianceList := p.getSpecificationCompliance(langStr)
	if len(complianceList) > 0 {
		report["summary"].(map[string]interface{})["compliance_level"] = complianceList[0].ComplianceLevel
	}

	return report
}

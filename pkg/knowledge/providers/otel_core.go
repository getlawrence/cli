package providers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// OTELCoreProvider implements both Provider and RegistryProvider to include both registry components and core packages
type OTELCoreProvider struct {
	language       types.ComponentLanguage
	registryClient *http.Client
}

// NewOTELCoreProvider creates a new OTEL core provider
func NewOTELCoreProvider(language types.ComponentLanguage) *OTELCoreProvider {
	return &OTELCoreProvider{
		language:       language,
		registryClient: &http.Client{Timeout: 30 * time.Second},
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
		fmt.Printf("Warning: failed to discover registry components: %v\n", err)
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
			fmt.Printf("Warning: failed to discover package %s: %v\n", pkg.Name, err)
			continue
		}
		components = append(components, component)
	}

	return components, nil
}

// CorePackage represents a core OTEL package
type CorePackage struct {
	Name string
	Type string
}

// getCorePackagesForLanguage returns the core packages that should be discovered for a language
func (p *OTELCoreProvider) getCorePackagesForLanguage(language string) []CorePackage {
	switch language {
	case "javascript":
		return []CorePackage{
			{Name: "@opentelemetry/api", Type: "api"},
			{Name: "@opentelemetry/sdk-node", Type: "sdk"},
			{Name: "@opentelemetry/sdk-web", Type: "sdk"},
			{Name: "@opentelemetry/exporter-trace-otlp-http", Type: "exporter"},
			{Name: "@opentelemetry/exporter-trace-otlp-grpc", Type: "exporter"},
			{Name: "@opentelemetry/exporter-trace-otlp-proto", Type: "exporter"},
			{Name: "@opentelemetry/propagator-b3", Type: "propagator"},
			{Name: "@opentelemetry/propagator-jaeger", Type: "propagator"},
			{Name: "@opentelemetry/propagator-w3c", Type: "propagator"},
		}
	case "python":
		return []CorePackage{
			{Name: "opentelemetry-api", Type: "api"},
			{Name: "opentelemetry-sdk", Type: "sdk"},
			{Name: "opentelemetry-exporter-otlp", Type: "exporter"},
			{Name: "opentelemetry-exporter-jaeger", Type: "exporter"},
			{Name: "opentelemetry-exporter-zipkin", Type: "exporter"},
		}
	case "go":
		return []CorePackage{
			{Name: "go.opentelemetry.io/otel", Type: "api"},
			{Name: "go.opentelemetry.io/otel/sdk", Type: "sdk"},
			{Name: "go.opentelemetry.io/otel/trace", Type: "api"},
			{Name: "go.opentelemetry.io/otel/metrics", Type: "api"},
			{Name: "go.opentelemetry.io/otel/logs", Type: "api"},
			{Name: "go.opentelemetry.io/otel/propagation", Type: "api"},
			{Name: "go.opentelemetry.io/otel/semconv/v1.34.0", Type: "semconv"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploggrpc", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/jaeger", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/zipkin", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/exporters/prometheus", Type: "exporter"},
			{Name: "go.opentelemetry.io/otel/sdk/resource", Type: "sdk"},
			{Name: "go.opentelemetry.io/otel/sdk/trace", Type: "sdk"},
			{Name: "go.opentelemetry.io/otel/sdk/metrics", Type: "sdk"},
			{Name: "go.opentelemetry.io/otel/sdk/logs", Type: "sdk"},
		}
	case "java":
		return []CorePackage{
			{Name: "io.opentelemetry:opentelemetry-api", Type: "api"},
			{Name: "io.opentelemetry:opentelemetry-sdk", Type: "sdk"},
			{Name: "io.opentelemetry:opentelemetry-sdk-extension-autoconfigure", Type: "sdk"},
			{Name: "io.opentelemetry:opentelemetry-exporter-otlp", Type: "exporter"},
			{Name: "io.opentelemetry:opentelemetry-exporter-jaeger", Type: "exporter"},
			{Name: "io.opentelemetry:opentelemetry-exporter-zipkin", Type: "exporter"},
			{Name: "io.opentelemetry:opentelemetry-extension-trace-propagators", Type: "propagator"},
			{Name: "io.opentelemetry:opentelemetry-instrumentation-api", Type: "api"},
		}
	case "csharp", "dotnet":
		return []CorePackage{
			{Name: "OpenTelemetry", Type: "api"},
			{Name: "OpenTelemetry.Api", Type: "api"},
			{Name: "OpenTelemetry.Sdk", Type: "sdk"},
			{Name: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Type: "exporter"},
			{Name: "OpenTelemetry.Extensions.Hosting", Type: "sdk"},
		}
	case "php":
		return []CorePackage{
			{Name: "open-telemetry/api", Type: "api"},
			{Name: "open-telemetry/sdk", Type: "sdk"},
			{Name: "open-telemetry/exporter-otlp", Type: "exporter"},
		}
	case "ruby":
		return []CorePackage{
			{Name: "opentelemetry-api", Type: "api"},
			{Name: "opentelemetry-sdk", Type: "sdk"},
			{Name: "opentelemetry-exporter-otlp", Type: "exporter"},
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
	switch componentType {
	case "core":
		return "sdk"
	case "exporter":
		return "exporter"
	case "propagator":
		return "propagator"
	case "sampler":
		return "sampler"
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

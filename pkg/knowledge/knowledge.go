package knowledge

import (
	"fmt"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeClient provides access to knowledge about OpenTelemetry components
type Knowledge struct {
	storage   storage.Storage
	providers map[string]*providers.OTELCoreProvider
	logger    logger.Logger
}

// NewKnowledgeClient creates a new knowledge client
func NewKnowledge(storage storage.Storage, logger logger.Logger) *Knowledge {
	// Initialize providers for all supported languages
	providersMap := make(map[string]*providers.OTELCoreProvider)
	languages := []string{"javascript", "python", "go", "java", "csharp", "dotnet", "php", "ruby"}

	for _, lang := range languages {
		provider := providers.NewOTELCoreProvider(types.ComponentLanguage(lang), logger)
		providersMap[lang] = provider
	}

	return &Knowledge{
		storage:   storage,
		providers: providersMap,
		logger:    logger,
	}
}

// Close closes the underlying storage connection
func (kc *Knowledge) Close() error {
	return kc.storage.Close()
}

// getProvider safely gets a provider for a language, returning nil if not found
func (kc *Knowledge) getProvider(language string) *providers.OTELCoreProvider {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	provider, exists := kc.providers[normalizedLanguage]
	if !exists {
		kc.logger.Logf("Warning: no provider found for language: %s (normalized: %s)\n", language, normalizedLanguage)
		return nil
	}
	return provider
}

// normalizeLanguage normalizes language names to handle variations and aliases
func (kc *Knowledge) normalizeLanguage(language string) string {
	switch strings.ToLower(language) {
	case "dotnet", "c#", "csharp":
		return "csharp"
	case "js", "node", "nodejs":
		return "javascript"
	case "py", "python":
		return "python"
	case "go", "golang":
		return "go"
	case "java":
		return "java"
	case "php":
		return "php"
	case "ruby":
		return "ruby"
	default:
		return strings.ToLower(language)
	}
}

// GetCorePackages returns core packages for a language
func (kc *Knowledge) GetCorePackages(language string) ([]string, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeSDK),
	}

	// Also query for API components
	apiQuery := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeAPI),
	}

	// Get SDK components
	sdkResult := kc.storage.GetComponentsLight(query)
	apiResult := kc.storage.GetComponentsLight(apiQuery)

	var corePackages []string

	// Add main SDK packages (filter out framework-specific ones)
	provider := kc.getProvider(normalizedLanguage)
	if provider != nil {
		for _, component := range sdkResult.Components {
			if provider.IsMainSDK(component.Name) {
				corePackages = append(corePackages, component.Name)
			}
		}
	} else {
		// Fallback to pattern-based detection if no provider
		for _, component := range sdkResult.Components {
			if kc.isMainSDKByPattern(component.Name) {
				corePackages = append(corePackages, component.Name)
			}
		}
	}

	// Add API packages
	for _, component := range apiResult.Components {
		corePackages = append(corePackages, component.Name)
	}

	return corePackages, nil
}

// IsMainSDK checks if a package is a main SDK package for the given language
func (kc *Knowledge) IsMainSDK(language, packageName string) bool {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	provider := kc.getProvider(normalizedLanguage)
	if provider != nil {
		return provider.IsMainSDK(packageName)
	}

	// Fallback to pattern-based detection
	return kc.isMainSDKByPattern(packageName)
}

// GetMainSDKs returns all main SDK packages for the given language
func (kc *Knowledge) GetMainSDKs(language string) ([]providers.CorePackage, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return nil, fmt.Errorf("no provider found for language: %s", language)
	}

	return provider.GetMainSDKs(), nil
}

// GetPackageType returns the type of a package for the given language
func (kc *Knowledge) GetPackageType(language, packageName string) (string, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return "", fmt.Errorf("no provider found for language: %s", language)
	}

	// Use reflection to access the private method, or make it public
	// For now, we'll implement a simpler approach
	return kc.getPackageTypeByPattern(packageName), nil
}

// getPackageTypeByPattern determines package type based on naming patterns
func (kc *Knowledge) getPackageTypeByPattern(packageName string) string {
	name := strings.ToLower(packageName)

	if strings.Contains(name, "sdk") {
		return "sdk"
	}
	if strings.Contains(name, "api") {
		return "api"
	}
	if strings.Contains(name, "exporter") {
		return "exporter"
	}
	if strings.Contains(name, "propagator") {
		return "propagator"
	}
	if strings.Contains(name, "instrumentation") {
		return "instrumentation"
	}

	return "component"
}

// IsCorePackage checks if a package is a core OpenTelemetry package
func (kc *Knowledge) IsCorePackage(language, packageName string) bool {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	provider := kc.getProvider(normalizedLanguage)
	if provider == nil {
		return false
	}

	// This would require making getCorePackagesForLanguage public in OTELCoreProvider
	// For now, we'll use a simpler approach
	return kc.isMainSDKByPattern(packageName) ||
		strings.Contains(strings.ToLower(packageName), "opentelemetry")
}

// isMainSDKByPattern provides fallback pattern-based detection
func (kc *Knowledge) isMainSDKByPattern(packageName string) bool {
	// Skip instrumentation packages
	if strings.Contains(strings.ToLower(packageName), "instrumentation") {
		return false
	}

	// Skip auto-instrumentation packages
	if strings.Contains(strings.ToLower(packageName), "auto-instrumentation") {
		return false
	}

	// Check for main SDK patterns
	mainSDKPatterns := []string{
		"sdk-node", "sdk-web", "sdk", // JavaScript
		"otel/sdk",           // Go
		"opentelemetry-sdk",  // Python, Ruby
		"OpenTelemetry.Sdk",  // .NET/C#
		"open-telemetry/sdk", // PHP
	}

	for _, pattern := range mainSDKPatterns {
		if strings.Contains(packageName, pattern) {
			return true
		}
	}

	return false
}

// GetInstrumentationPackage returns the package name for a specific instrumentation
func (kc *Knowledge) GetInstrumentationPackage(language, instrumentation string) (string, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeInstrumentation),
		Name:     instrumentation, // This will do a LIKE search
	}

	result := kc.storage.GetComponentsLight(query)

	// Find the best match for the instrumentation
	for _, component := range result.Components {
		if isInstrumentationFor(component.Name, instrumentation) {
			return component.Name, nil
		}
	}

	// If no exact match, try broader search
	query.Name = ""
	query.Framework = instrumentation // Search in instrumentation targets
	result = kc.storage.GetComponentsLight(query)

	for _, component := range result.Components {
		if isInstrumentationFor(component.Name, instrumentation) {
			return component.Name, nil
		}
	}

	return "", nil
}

// GetComponentPackage returns the package name for a specific component
func (kc *Knowledge) GetComponentPackage(language, componentType, component string) (string, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	// Map lowercase component types to actual ComponentType constants
	actualType := mapComponentType(componentType)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     actualType,
		Name:     component,
	}

	result := kc.storage.GetComponentsLight(query)

	for _, comp := range result.Components {
		if comp.Name == component || extractComponentSimpleName(comp.Name) == component {
			return comp.Name, nil
		}
	}

	return "", nil
}

// mapComponentType maps lowercase component type names to actual ComponentType constants
func mapComponentType(componentType string) string {
	switch strings.ToLower(componentType) {
	case "api":
		return string(types.ComponentTypeAPI)
	case "sdk":
		return string(types.ComponentTypeSDK)
	case "instrumentation":
		return string(types.ComponentTypeInstrumentation)
	case "exporter":
		return string(types.ComponentTypeExporter)
	case "propagator":
		return string(types.ComponentTypePropagator)
	case "sampler":
		return string(types.ComponentTypeSampler)
	case "processor":
		return string(types.ComponentTypeProcessor)
	case "resource":
		return string(types.ComponentTypeResource)
	case "resourcedetector":
		return string(types.ComponentTypeResourceDetector)
	default:
		return componentType // Return as-is if no mapping found
	}
}

// GetPrerequisites returns prerequisite rules for a language
// This is a simplified version - in practice, prerequisites would be derived from component metadata
func (kc *Knowledge) GetPrerequisites(language string) ([]PrerequisiteRule, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Type:     string(types.ComponentTypeInstrumentation),
	}

	result := kc.storage.GetComponentsLight(query)

	var rules []PrerequisiteRule

	// Create basic prerequisite rules based on common patterns
	for _, component := range result.Components {
		rule := createPrerequisiteRule(component)
		if rule != nil {
			rules = append(rules, *rule)
		}
	}

	return rules, nil
}

// GetComponentByName returns a component by name
func (kc *Knowledge) GetComponentByName(name string) (*types.Component, error) {
	return kc.storage.GetComponentByName(name), nil
}

// GetComponentsByLanguage returns components for a specific language with pagination
func (kc *Knowledge) GetComponentsByLanguage(language string, limit, offset int) (*ComponentResult, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := kc.normalizeLanguage(language)

	query := storage.Query{
		Language: normalizedLanguage,
		Limit:    limit,
		Offset:   offset,
	}

	result := kc.storage.GetComponentsLight(query)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

// GetComponentsByType returns components of a specific type with pagination
func (kc *Knowledge) GetComponentsByType(componentType string, limit, offset int) (*ComponentResult, error) {
	query := storage.Query{
		Type:   componentType,
		Limit:  limit,
		Offset: offset,
	}

	result := kc.storage.GetComponentsLight(query)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

// QueryComponents provides flexible querying with pagination
func (kc *Knowledge) QueryComponents(query ComponentQuery) (*ComponentResult, error) {
	// Normalize the language to handle aliases like "dotnet" -> "csharp"
	normalizedLanguage := ""
	if query.Language != "" {
		normalizedLanguage = kc.normalizeLanguage(query.Language)
	}

	storageQuery := storage.Query{
		Language:     normalizedLanguage,
		Type:         query.Type,
		Category:     query.Category,
		Status:       query.Status,
		SupportLevel: query.SupportLevel,
		Name:         query.Name,
		Framework:    query.Framework,
		MinDate:      query.MinDate,
		MaxDate:      query.MaxDate,
		Limit:        query.Limit,
		Offset:       query.Offset,
	}

	result := kc.storage.GetComponentsLight(storageQuery)

	return &ComponentResult{
		Components: result.Components,
		Total:      result.Total,
		HasMore:    result.HasMore,
	}, nil
}

// ComponentQuery represents a flexible query for components
type ComponentQuery struct {
	Language     string
	Type         string
	Category     string
	Status       string
	SupportLevel string
	Name         string
	Framework    string // For instrumentation targets
	MinDate      time.Time
	MaxDate      time.Time
	Limit        int
	Offset       int
}

// ComponentResult represents the result of a component query
type ComponentResult struct {
	Components []types.Component
	Total      int
	HasMore    bool
}

// PrerequisiteRule defines instrumentation prerequisites (legacy compatibility)
type PrerequisiteRule struct {
	If       []string `json:"if"`
	Requires []string `json:"requires"`
	Unless   []string `json:"unless"`
}

// Helper functions (moved from the old knowledge.go)

// isInstrumentationFor checks if a component is an instrumentation for a specific target
func isInstrumentationFor(componentName, target string) bool {
	// Extract the target framework/library name from the component name
	// e.g., "@opentelemetry/instrumentation-express" -> "express"
	extractedTarget := extractInstrumentationTarget(componentName)
	return extractedTarget == target
}

// extractInstrumentationTarget extracts the target framework/library from an instrumentation name
func extractInstrumentationTarget(name string) string {
	// Handle common patterns:
	// "@opentelemetry/instrumentation-express" -> "express"
	// "@opentelemetry/instrumentation-http" -> "http"
	// "opentelemetry-instrumentation-express" -> "express"
	// "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp" -> "http"

	// Handle Go instrumentation packages
	if strings.HasPrefix(name, "go.opentelemetry.io/contrib/instrumentation/") {
		// Extract path after the instrumentation prefix
		path := strings.TrimPrefix(name, "go.opentelemetry.io/contrib/instrumentation/")
		// Split by "/" and get the last meaningful part before "otel*"
		parts := strings.Split(path, "/")
		if len(parts) >= 2 {
			// For "net/http/otelhttp", we want "http"
			// For "github.com/gin-gonic/gin/otelgin", we want "gin"
			if len(parts) >= 3 && strings.HasPrefix(parts[len(parts)-1], "otel") {
				return parts[len(parts)-2] // Return the part before "otel*"
			}
		}
		// Fallback to last part
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Handle JavaScript instrumentation packages
	if strings.HasPrefix(name, "@opentelemetry/instrumentation-") {
		return strings.TrimPrefix(name, "@opentelemetry/instrumentation-")
	}
	if strings.HasPrefix(name, "opentelemetry-instrumentation-") {
		return strings.TrimPrefix(name, "opentelemetry-instrumentation-")
	}

	// Handle special case for auto-instrumentations
	if strings.HasPrefix(name, "@opentelemetry/auto-instrumentations-") {
		return "auto"
	}

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-instrumentation")

	return name
}

// extractComponentSimpleName extracts a simple name from a component name
func extractComponentSimpleName(name string) string {
	// Handle common patterns:
	// "@opentelemetry/sdk-trace-base" -> "sdk-trace-base"
	// "opentelemetry-sdk-trace-base" -> "sdk-trace-base"
	// "go.opentelemetry.io/otel/exporters/jaeger" -> "jaeger"

	// Handle Go packages - extract the last path component
	if strings.HasPrefix(name, "go.opentelemetry.io/") {
		// For instrumentation packages, extract just the target name
		if strings.Contains(name, "instrumentation/") {
			return extractInstrumentationTarget(name)
		}
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Handle JavaScript packages
	if strings.HasPrefix(name, "@opentelemetry/") {
		// For instrumentation packages, extract just the target name
		if strings.Contains(name, "instrumentation-") || strings.Contains(name, "auto-instrumentations-") {
			return extractInstrumentationTarget(name)
		}
		return strings.TrimPrefix(name, "@opentelemetry/")
	}
	if strings.HasPrefix(name, "opentelemetry-") {
		// For instrumentation packages, extract just the target name
		if strings.Contains(name, "instrumentation-") {
			return extractInstrumentationTarget(name)
		}
		return strings.TrimPrefix(name, "opentelemetry-")
	}

	return name
}

// createPrerequisiteRule creates a prerequisite rule for a component
func createPrerequisiteRule(component types.Component) *PrerequisiteRule {
	// This is a simplified rule creation - in practice, you might want to
	// derive these from component metadata, dependencies, or tags

	target := extractInstrumentationTarget(component.Name)

	// Create language-specific rules based on component targets
	if component.Language == types.ComponentLanguageJavaScript {
		// JavaScript-specific rules
		if target == "express" {
			return &PrerequisiteRule{
				If:       []string{"express"},
				Requires: []string{"http"},
				Unless:   []string{"auto"},
			}
		}
	}

	return nil
}

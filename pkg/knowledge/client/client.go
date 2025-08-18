package client

import (
	"fmt"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeClient provides a unified interface to the knowledge base
// It only loads data on-demand to keep memory usage low
type KnowledgeClient struct {
	storage *storage.Storage
}

// NewKnowledgeClient creates a new knowledge client
func NewKnowledgeClient(dbPath string, logger logger.Logger) (*KnowledgeClient, error) {
	if dbPath == "" {
		dbPath = "knowledge.db"
	}

	storageClient, err := storage.NewStorage(dbPath, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge storage: %w", err)
	}

	return &KnowledgeClient{
		storage: storageClient,
	}, nil
}

// Close closes the underlying storage connection
func (kc *KnowledgeClient) Close() error {
	return kc.storage.Close()
}

// GetCorePackages returns core packages for a language
func (kc *KnowledgeClient) GetCorePackages(language string) ([]string, error) {
	query := storage.Query{
		Language: language,
		Type:     string(types.ComponentTypeSDK),
	}

	// Also query for API components
	apiQuery := storage.Query{
		Language: language,
		Type:     string(types.ComponentTypeAPI),
	}

	// Get SDK components
	sdkResult := kc.storage.GetComponentsLight(query)
	apiResult := kc.storage.GetComponentsLight(apiQuery)

	var corePackages []string

	// Add main SDK packages (filter out framework-specific ones)
	for _, component := range sdkResult.Components {
		if isMainSDK(component.Name) {
			corePackages = append(corePackages, component.Name)
		}
	}

	// Add API packages
	for _, component := range apiResult.Components {
		corePackages = append(corePackages, component.Name)
	}

	return corePackages, nil
}

// GetInstrumentationPackage returns the package name for a specific instrumentation
func (kc *KnowledgeClient) GetInstrumentationPackage(language, instrumentation string) (string, error) {
	query := storage.Query{
		Language: language,
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
func (kc *KnowledgeClient) GetComponentPackage(language, componentType, component string) (string, error) {
	// Map lowercase component types to actual ComponentType constants
	actualType := mapComponentType(componentType)

	query := storage.Query{
		Language: language,
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
func (kc *KnowledgeClient) GetPrerequisites(language string) ([]PrerequisiteRule, error) {
	query := storage.Query{
		Language: language,
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
func (kc *KnowledgeClient) GetComponentByName(name string) (*types.Component, error) {
	return kc.storage.GetComponentByName(nil, name), nil
}

// GetComponentsByLanguage returns components for a specific language with pagination
func (kc *KnowledgeClient) GetComponentsByLanguage(language string, limit, offset int) (*ComponentResult, error) {
	query := storage.Query{
		Language: language,
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
func (kc *KnowledgeClient) GetComponentsByType(componentType string, limit, offset int) (*ComponentResult, error) {
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
func (kc *KnowledgeClient) QueryComponents(query ComponentQuery) (*ComponentResult, error) {
	storageQuery := storage.Query{
		Language:     query.Language,
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

// GetStatistics returns knowledge base statistics
func (kc *KnowledgeClient) GetStatistics() (*types.Statistics, error) {
	kb, err := kc.storage.LoadKnowledgeBase()
	if err != nil {
		return nil, err
	}
	return &kb.Statistics, nil
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

// isMainSDK determines if an SDK component is a main/general SDK (not framework-specific)
func isMainSDK(name string) bool {
	mainSDKs := []string{
		"@opentelemetry/sdk-node",      // Main Node.js SDK
		"@opentelemetry/sdk-web",       // Main Web SDK
		"go.opentelemetry.io/otel/sdk", // Main Go SDK
		// Add other main SDKs for other languages as needed
	}

	for _, mainSDK := range mainSDKs {
		if name == mainSDK {
			return true
		}
	}
	return false
}

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
		parts := strings.Split(name, "/")
		if len(parts) > 0 {
			return parts[len(parts)-1]
		}
	}

	// Handle JavaScript packages
	if strings.HasPrefix(name, "@opentelemetry/") {
		return strings.TrimPrefix(name, "@opentelemetry/")
	}
	if strings.HasPrefix(name, "opentelemetry-") {
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

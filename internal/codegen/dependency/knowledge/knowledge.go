package knowledge

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/pkg/knowledge/storage"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

// LanguagePackages defines packages for a language using the new knowledge system
type LanguagePackages struct {
	Core             []string                     `json:"core"`
	Instrumentations map[string]string            `json:"instrumentations"`
	Components       map[string]map[string]string `json:"components"`
	Prerequisites    []PrerequisiteRule           `json:"prerequisites"`
}

// PrerequisiteRule defines instrumentation prerequisites
type PrerequisiteRule struct {
	If       []string `json:"if"`
	Requires []string `json:"requires"`
	Unless   []string `json:"unless"`
}

// KnowledgeBase contains package information for all languages using the new system
type KnowledgeBase struct {
	Languages map[string]LanguagePackages `json:"languages"`
	// Store the actual new knowledge base for direct access
	NewKB *kbtypes.KnowledgeBase
}

// LoadFromFile loads the knowledge base using the new knowledge system
func LoadFromFile(root string) (*KnowledgeBase, error) {
	// Use a persistent database file that can be shared with the knowledge update command
	dbPath := "knowledge.db"
	storageClient, err := storage.NewStorage(dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge storage: %w", err)
	}
	defer storageClient.Close()

	// Load the new knowledge base
	newKB, err := storageClient.LoadKnowledgeBase("")
	if err != nil {
		return nil, fmt.Errorf("failed to load knowledge base: %w", err)
	}

	// Convert to the legacy format for compatibility with existing code
	legacyKB := convertToLegacyFormat(newKB)
	legacyKB.NewKB = newKB

	return legacyKB, nil
}

// convertToLegacyFormat converts the new knowledge base to the legacy format
func convertToLegacyFormat(kb *kbtypes.KnowledgeBase) *KnowledgeBase {
	legacyKB := &KnowledgeBase{
		Languages: make(map[string]LanguagePackages),
	}

	// Group components by language
	languageGroups := make(map[string][]kbtypes.Component)
	for _, component := range kb.Components {
		lang := string(component.Language)
		languageGroups[lang] = append(languageGroups[lang], component)
	}

	// Convert each language group
	for lang, components := range languageGroups {
		legacyLang := LanguagePackages{
			Core:             extractCorePackages(components),
			Instrumentations: extractInstrumentations(components),
			Components:       extractComponents(components),
			Prerequisites:    extractPrerequisites(components),
		}
		legacyKB.Languages[lang] = legacyLang
	}

	return legacyKB
}

// extractCorePackages extracts core OpenTelemetry packages for a language
func extractCorePackages(components []kbtypes.Component) []string {
	var corePackages []string

	for _, component := range components {
		// Core packages are typically SDK, API, or core components
		if component.Type == kbtypes.ComponentTypeSDK ||
			component.Type == kbtypes.ComponentTypeAPI ||
			component.Category == kbtypes.ComponentCategoryCore ||
			component.Category == kbtypes.ComponentCategoryStableSDK {
			corePackages = append(corePackages, component.Name)
		}
	}

	return corePackages
}

// extractInstrumentations extracts instrumentation packages for a language
func extractInstrumentations(components []kbtypes.Component) map[string]string {
	instrumentations := make(map[string]string)

	for _, component := range components {
		if component.Type == kbtypes.ComponentTypeInstrumentation {
			// Extract the target framework/library name from the component name
			// e.g., "@opentelemetry/instrumentation-express" -> "express"
			target := extractInstrumentationTarget(component.Name)
			if target != "" {
				instrumentations[target] = component.Name
			}
		}
	}

	return instrumentations
}

// extractComponents extracts component packages organized by type
func extractComponents(components []kbtypes.Component) map[string]map[string]string {
	componentMap := make(map[string]map[string]string)

	for _, component := range components {
		compType := string(component.Type)
		if compType == "" {
			continue
		}

		if componentMap[compType] == nil {
			componentMap[compType] = make(map[string]string)
		}

		// Use a simplified name for the component
		simpleName := extractComponentSimpleName(component.Name)
		if simpleName != "" {
			componentMap[compType][simpleName] = component.Name
		}
	}

	return componentMap
}

// extractPrerequisites extracts prerequisite rules for instrumentations
func extractPrerequisites(components []kbtypes.Component) []PrerequisiteRule {
	var rules []PrerequisiteRule

	// This is a simplified extraction - in practice, you might want to
	// derive these from component metadata, dependencies, or tags
	for _, component := range components {
		if component.Type == kbtypes.ComponentTypeInstrumentation {
			// Create basic prerequisite rules based on component information
			rule := createPrerequisiteRule(component)
			if rule != nil {
				rules = append(rules, *rule)
			}
		}
	}

	return rules
}

// extractInstrumentationTarget extracts the target framework/library from an instrumentation name
func extractInstrumentationTarget(name string) string {
	// Handle common patterns:
	// "@opentelemetry/instrumentation-express" -> "express"
	// "@opentelemetry/instrumentation-http" -> "http"
	// "opentelemetry-instrumentation-express" -> "express"

	// Remove common prefixes
	name = strings.TrimPrefix(name, "@opentelemetry/instrumentation-")
	name = strings.TrimPrefix(name, "opentelemetry-instrumentation-")

	// Remove common suffixes
	name = strings.TrimSuffix(name, "-instrumentation")

	return name
}

// extractComponentSimpleName extracts a simple name from a component name
func extractComponentSimpleName(name string) string {
	// Handle common patterns:
	// "@opentelemetry/sdk-trace-base" -> "sdk-trace-base"
	// "opentelemetry-sdk-trace-base" -> "sdk-trace-base"

	// Remove common prefixes
	name = strings.TrimPrefix(name, "@opentelemetry/")
	name = strings.TrimPrefix(name, "opentelemetry-")

	return name
}

// createPrerequisiteRule creates a prerequisite rule for a component
func createPrerequisiteRule(component kbtypes.Component) *PrerequisiteRule {
	// This is a simplified rule creation - in practice, you might want to
	// derive these from component metadata, dependencies, or tags

	// For now, create basic rules based on common patterns
	if strings.Contains(strings.ToLower(component.Name), "http") {
		return &PrerequisiteRule{
			If:       []string{"http"},
			Requires: []string{"core"},
			Unless:   []string{},
		}
	}

	if strings.Contains(strings.ToLower(component.Name), "express") {
		return &PrerequisiteRule{
			If:       []string{"express"},
			Requires: []string{"core", "http"},
			Unless:   []string{},
		}
	}

	return nil
}

// GetCorePackages returns core packages for a language
func (kb *KnowledgeBase) GetCorePackages(language string) []string {
	if lang, ok := kb.Languages[language]; ok {
		return lang.Core
	}
	return nil
}

// GetInstrumentationPackage returns the package for an instrumentation
func (kb *KnowledgeBase) GetInstrumentationPackage(language, instrumentation string) string {
	if lang, ok := kb.Languages[language]; ok {
		return lang.Instrumentations[instrumentation]
	}
	return ""
}

// GetComponentPackage returns the package for a component
func (kb *KnowledgeBase) GetComponentPackage(language, componentType, component string) string {
	if lang, ok := kb.Languages[language]; ok {
		if compType, ok := lang.Components[componentType]; ok {
			return compType[component]
		}
	}
	return ""
}

// GetPrerequisites returns prerequisite rules for a language
func (kb *KnowledgeBase) GetPrerequisites(language string) []PrerequisiteRule {
	if lang, ok := kb.Languages[language]; ok {
		return lang.Prerequisites
	}
	return nil
}

// GetNewKnowledgeBase returns the underlying new knowledge base for direct access
func (kb *KnowledgeBase) GetNewKnowledgeBase() *kbtypes.KnowledgeBase {
	return kb.NewKB
}

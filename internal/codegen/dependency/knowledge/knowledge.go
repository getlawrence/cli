package knowledge

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// LanguagePackages defines packages for a language
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

// KnowledgeBase contains package information for all languages
type KnowledgeBase struct {
	Languages map[string]LanguagePackages `json:"languages"`
}

// LoadFromFile loads the knowledge base from JSON
func LoadFromFile(root string) (*KnowledgeBase, error) {
	path := filepath.Join(root, "internal", "codegen", "dependency", "knowledge", "data", "otel_packages.json")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read knowledge base: %w", err)
	}

	var kb KnowledgeBase
	if err := json.Unmarshal(b, &kb); err != nil {
		return nil, fmt.Errorf("parse knowledge base: %w", err)
	}

	return &kb, nil
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

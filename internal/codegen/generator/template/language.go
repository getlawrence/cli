package template

import (
	"github.com/getlawrence/cli/internal/templates"
)

// LanguageCodeGenerator defines the interface for language-specific code generation
type LanguageCodeGenerator interface {
	// GetSupportedMethods returns the installation methods supported by this language
	GetSupportedMethods() []templates.InstallationMethod

	// GetOutputFilename returns the appropriate output filename for the given method
	GetOutputFilename(method templates.InstallationMethod) string

	// ValidateMethod checks if the given method is supported for this language
	ValidateMethod(method templates.InstallationMethod) error

	// GetLanguageName returns the language name for this generator
	GetLanguageName() string
}

// LanguageGeneratorRegistry holds all registered language generators
type LanguageGeneratorRegistry struct {
	generators map[string]LanguageCodeGenerator
}

// NewLanguageGeneratorRegistry creates a new registry
func NewLanguageGeneratorRegistry() *LanguageGeneratorRegistry {
	return &LanguageGeneratorRegistry{
		generators: make(map[string]LanguageCodeGenerator),
	}
}

// RegisterLanguage registers a language generator
func (r *LanguageGeneratorRegistry) RegisterLanguage(name string, generator LanguageCodeGenerator) {
	r.generators[name] = generator
}

// GetGenerator retrieves a language generator
func (r *LanguageGeneratorRegistry) GetGenerator(language string) (LanguageCodeGenerator, bool) {
	gen, exists := r.generators[language]
	return gen, exists
}

// GetSupportedLanguages returns all registered languages
func (r *LanguageGeneratorRegistry) GetSupportedLanguages() []string {
	languages := make([]string, 0, len(r.generators))
	for lang := range r.generators {
		languages = append(languages, lang)
	}
	return languages
}

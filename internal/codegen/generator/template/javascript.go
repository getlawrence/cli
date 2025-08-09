package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// JavaScriptCodeGenerator handles JS-specific code generation
type JavaScriptCodeGenerator struct{}

// NewJavaScriptCodeGenerator creates a new JS code generator
func NewJavaScriptCodeGenerator() *JavaScriptCodeGenerator { return &JavaScriptCodeGenerator{} }

// GetSupportedMethods returns supported methods for JS
func (g *JavaScriptCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

// GetOutputFilename suggests output filename
func (g *JavaScriptCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "otel.js"
	case templates.AutoInstrumentation:
		return "otel.js"
	default:
		return "otel.js"
	}
}

// ValidateMethod validates the method
func (g *JavaScriptCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	for _, m := range g.GetSupportedMethods() {
		if m == method {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for JavaScript", method)
}

// GetLanguageName returns the language name key for templates
func (g *JavaScriptCodeGenerator) GetLanguageName() string { return "javascript" }

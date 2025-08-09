package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// RubyCodeGenerator handles Ruby-specific code generation
type RubyCodeGenerator struct{}

// NewRubyCodeGenerator creates a new Ruby code generator
func NewRubyCodeGenerator() *RubyCodeGenerator { return &RubyCodeGenerator{} }

// GetSupportedMethods returns the installation methods supported by Ruby
func (g *RubyCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *RubyCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "otel.rb"
	case templates.AutoInstrumentation:
		return "otel.rb"
	default:
		return "otel.rb"
	}
}

// ValidateMethod checks if the given method is supported for Ruby
func (g *RubyCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	for _, m := range g.GetSupportedMethods() {
		if m == method {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for Ruby", method)
}

// GetLanguageName returns the language name
func (g *RubyCodeGenerator) GetLanguageName() string { return "ruby" }

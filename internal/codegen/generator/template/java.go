package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// JavaCodeGenerator handles Java-specific code generation
type JavaCodeGenerator struct{}

// NewJavaCodeGenerator creates a new Java code generator
func NewJavaCodeGenerator() *JavaCodeGenerator { return &JavaCodeGenerator{} }

// GetSupportedMethods returns supported methods for Java
func (g *JavaCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

// GetOutputFilename suggests output filename
func (g *JavaCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "OtelInit.java"
	case templates.AutoInstrumentation:
		return "otel-auto-config.md"
	default:
		return "otel.java"
	}
}

// ValidateMethod validates the method
func (g *JavaCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	for _, m := range g.GetSupportedMethods() {
		if m == method {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for Java", method)
}

// GetLanguageName returns the language name key for templates
func (g *JavaCodeGenerator) GetLanguageName() string { return "java" }

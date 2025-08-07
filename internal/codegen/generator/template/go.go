package generator

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// GoCodeGenerator handles Go-specific code generation
type GoCodeGenerator struct{}

// NewGoCodeGenerator creates a new Go code generator
func NewGoCodeGenerator() *GoCodeGenerator {
	return &GoCodeGenerator{}
}

// GetSupportedMethods returns the installation methods supported by Go
func (g *GoCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *GoCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "otel.go"
	case templates.AutoInstrumentation:
		return "otel_auto.go"
	default:
		return "otel.go"
	}
}

// ValidateMethod checks if the given method is supported for Go
func (g *GoCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	supportedMethods := g.GetSupportedMethods()
	for _, supportedMethod := range supportedMethods {
		if method == supportedMethod {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for Go", method)
}

// GetLanguageName returns the language name
func (g *GoCodeGenerator) GetLanguageName() string {
	return "go"
}

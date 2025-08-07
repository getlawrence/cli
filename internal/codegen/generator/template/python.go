package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// PythonCodeGenerator handles Python-specific code generation
type PythonCodeGenerator struct{}

// NewPythonCodeGenerator creates a new Python code generator
func NewPythonCodeGenerator() *PythonCodeGenerator {
	return &PythonCodeGenerator{}
}

// GetSupportedMethods returns the installation methods supported by Python
func (g *PythonCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *PythonCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "otel.py"
	case templates.AutoInstrumentation:
		return "otel.py"
	default:
		return "otel.py"
	}
}

// ValidateMethod checks if the given method is supported for Python
func (g *PythonCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	supportedMethods := g.GetSupportedMethods()
	for _, supportedMethod := range supportedMethods {
		if method == supportedMethod {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for Python", method)
}

// GetLanguageName returns the language name
func (g *PythonCodeGenerator) GetLanguageName() string {
	return "python"
}

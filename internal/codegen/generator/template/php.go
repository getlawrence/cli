package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// PHPCodeGenerator handles PHP-specific code generation
type PHPCodeGenerator struct{}

// NewPHPCodeGenerator creates a new PHP code generator
func NewPHPCodeGenerator() *PHPCodeGenerator { return &PHPCodeGenerator{} }

// GetSupportedMethods returns the installation methods supported by PHP
func (g *PHPCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
	}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *PHPCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "otel.php"
	default:
		return "otel.php"
	}
}

// ValidateMethod checks if the given method is supported for PHP
func (g *PHPCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	for _, m := range g.GetSupportedMethods() {
		if m == method {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for PHP", method)
}

// GetLanguageName returns the language name
func (g *PHPCodeGenerator) GetLanguageName() string { return "php" }

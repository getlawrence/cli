package template

import (
	"fmt"

	"github.com/getlawrence/cli/internal/templates"
)

// DotNetCodeGenerator handles .NET-specific code generation
type DotNetCodeGenerator struct{}

func NewDotNetCodeGenerator() *DotNetCodeGenerator { return &DotNetCodeGenerator{} }

func (g *DotNetCodeGenerator) GetSupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{
		templates.CodeInstrumentation,
		templates.AutoInstrumentation,
	}
}

func (g *DotNetCodeGenerator) GetOutputFilename(method templates.InstallationMethod) string {
	switch method {
	case templates.CodeInstrumentation:
		return "Otel.cs"
	case templates.AutoInstrumentation:
		return "otel-auto.md"
	default:
		return "Otel.cs"
	}
}

func (g *DotNetCodeGenerator) ValidateMethod(method templates.InstallationMethod) error {
	for _, m := range g.GetSupportedMethods() {
		if m == method {
			return nil
		}
	}
	return fmt.Errorf("unsupported method %s for .NET", method)
}

func (g *DotNetCodeGenerator) GetLanguageName() string { return "dotnet" }

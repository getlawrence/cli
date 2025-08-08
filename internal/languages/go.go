package languages

import (
	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type GoPlugin struct{}

func (p *GoPlugin) ID() string          { return "go" }
func (p *GoPlugin) DisplayName() string { return "Go" }

func (p *GoPlugin) EntryPointTreeSitterLanguage() *sitter.Language { return golang.GetLanguage() }
func (p *GoPlugin) EntrypointQuery() string {
	return `
                (function_declaration 
                    name: (identifier) @func_name
                    (#eq? @func_name "main")
                ) @main_function
            `
}
func (p *GoPlugin) FileExtensions() []string { return []string{".go"} }

func (p *GoPlugin) Injector() inj.LanguageInjector      { return inj.NewGoHandler() }
func (p *GoPlugin) Dependencies() dep.DependencyHandler { return dep.NewGoHandler() }

func (p *GoPlugin) SupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{templates.CodeInstrumentation, templates.AutoInstrumentation}
}
func (p *GoPlugin) OutputFilename(m templates.InstallationMethod) string {
	switch m {
	case templates.CodeInstrumentation:
		return "otel.go"
	case templates.AutoInstrumentation:
		return "otel_auto.go"
	default:
		return "otel.go"
	}
}

func init() {
	// Register core plugin
	DefaultRegistry.Register(&GoPlugin{})
	// Register injector to avoid import cycle from injector -> languages
	inj.RegisterLanguageInjector("go", inj.NewGoHandler())
	// Register dependency handler to avoid import cycle
	dep.RegisterDependencyHandler("go", dep.NewGoHandler())
}

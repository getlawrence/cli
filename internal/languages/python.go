package languages

import (
	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	tspython "github.com/smacker/go-tree-sitter/python"
)

type PythonPlugin struct{}

func (p *PythonPlugin) ID() string          { return "python" }
func (p *PythonPlugin) DisplayName() string { return "Python" }

func (p *PythonPlugin) EntryPointTreeSitterLanguage() *sitter.Language { return tspython.GetLanguage() }
func (p *PythonPlugin) EntrypointQuery() string {
	return `
                (if_statement
                    condition: (binary_operator
                        left: (identifier) @name_var
                        right: (string) @main_str
                    )
                    (#eq? @name_var "__name__")
                    (#match? @main_str ".*__main__.*")
                ) @main_if_block
            `
}
func (p *PythonPlugin) FileExtensions() []string { return []string{".py", ".pyw"} }

func (p *PythonPlugin) Injector() inj.LanguageInjector      { return inj.NewPythonHandler() }
func (p *PythonPlugin) Dependencies() dep.DependencyHandler { return dep.NewPythonHandler() }

func (p *PythonPlugin) SupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{templates.CodeInstrumentation, templates.AutoInstrumentation}
}
func (p *PythonPlugin) OutputFilename(m templates.InstallationMethod) string {
	switch m {
	case templates.CodeInstrumentation:
		return "otel.py"
	case templates.AutoInstrumentation:
		return "otel_auto.py"
	default:
		return "otel.py"
	}
}

func init() {
	DefaultRegistry.Register(&PythonPlugin{})
	inj.RegisterLanguageInjector("python", inj.NewPythonHandler())
	dep.RegisterDependencyHandler("python", dep.NewPythonHandler())
}

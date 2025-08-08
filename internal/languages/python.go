package languages

import (
	"fmt"
	"strings"

	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	tspython "github.com/smacker/go-tree-sitter/python"
)

// PythonPlugin is a single type implementing both the LanguagePlugin API and the injector
type PythonPlugin struct {
	config *types.LanguageConfig
}

func NewPythonPlugin() *PythonPlugin {
	return &PythonPlugin{
		config: &types.LanguageConfig{
			Language:       "Python",
			FileExtensions: []string{".py", ".pyw"},
			ImportQueries: map[string]string{
				"existing_imports": `
 (import_statement 
     name: (dotted_name) @import_path
 ) @import_location

 (import_from_statement
     module: (dotted_name) @import_path
 ) @import_location
`,
			},
			FunctionQueries: map[string]string{
				"main_function": `
 (if_statement
     condition: (binary_operator
         left: (identifier) @name_var
         right: (string) @main_str
     )
     (#eq? @name_var "__name__")
     (#match? @main_str ".*__main__.*")
 ) @main_if_block
`,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
 (block
     (assignment) @after_variables
 )

 (block
     (expression_statement
         (call)) @before_function_calls
 )

 (block) @function_start
`,
			},
			ImportTemplate: `from opentelemetry import %s`,
			InitializationTemplate: `
	# Initialize OpenTelemetry
	from otel import init_tracer
	tracer_provider = init_tracer()
`,
			CleanupTemplate: `tp.shutdown()`,
		},
	}
}

// LanguagePlugin core
func (p *PythonPlugin) ID() string                                     { return "python" }
func (p *PythonPlugin) DisplayName() string                            { return "Python" }
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

// Provide injector and dependencies
func (p *PythonPlugin) Injector() inj.LanguageInjector      { return p } // PythonPlugin itself implements LanguageInjector
func (p *PythonPlugin) Dependencies() dep.DependencyHandler { return dep.NewPythonHandler() }

// Template support
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

// Injector implementation (methods from injector.LanguageInjector)
func (p *PythonPlugin) GetTreeSitterLanguage() *sitter.Language { return tspython.GetLanguage() }
func (p *PythonPlugin) GetLanguage() *sitter.Language           { return tspython.GetLanguage() }
func (p *PythonPlugin) GetConfig() *types.LanguageConfig        { return p.config }
func (p *PythonPlugin) GetRequiredImports() []string {
	return []string{
		"opentelemetry.sdk.trace",
		"opentelemetry.exporter.otlp.proto.http.trace_exporter",
	}
}
func (p *PythonPlugin) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}
	var result strings.Builder
	for _, importPath := range imports {
		result.WriteString(p.FormatSingleImport(importPath))
	}
	return result.String()
}
func (p *PythonPlugin) FormatSingleImport(importPath string) string {
	if strings.Contains(importPath, ".") {
		parts := strings.Split(importPath, ".")
		return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	}
	return fmt.Sprintf("import %s\n", importPath)
}
func (p *PythonPlugin) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		importPath := node.Content(content)
		analysis.ExistingImports[importPath] = true
		if strings.Contains(importPath, "opentelemetry") {
			analysis.HasOTELImports = true
		}
	case "import_location":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: 2}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}
func (p *PythonPlugin) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_if_block":
		var bodyNode *sitter.Node
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "block" {
				bodyNode = child
				break
			}
		}
		if bodyNode != nil {
			insertionPoint := p.findBestInsertionPoint(bodyNode, content, config)
			hasOTELSetup := p.detectExistingOTELSetup(bodyNode, content)
			entryPoint := types.EntryPointInfo{
				Name:         "if __name__ == '__main__'",
				LineNumber:   bodyNode.StartPoint().Row + 1,
				Column:       bodyNode.StartPoint().Column + 1,
				BodyStart:    insertionPoint,
				BodyEnd:      types.InsertionPoint{LineNumber: bodyNode.EndPoint().Row + 1, Column: bodyNode.EndPoint().Column + 1},
				HasOTELSetup: hasOTELSetup,
			}
			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
		}
	}
	p.findMainBlockWithRegex(content, analysis)
}
func (p *PythonPlugin) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "after_variables":
		return 3
	case "before_function_calls":
		return 2
	case "function_start":
		return 1
	default:
		return 1
	}
}
func (p *PythonPlugin) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: bodyNode.StartPoint().Row + 1, Column: bodyNode.StartPoint().Column + 1, Priority: 1}
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), p.GetLanguage())
		if err != nil {
			return defaultPoint
		}
		defer query.Close()
		cursor := sitter.NewQueryCursor()
		defer cursor.Close()
		cursor.Exec(query, bodyNode)
		bestPoint := defaultPoint
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}
			for _, capture := range match.Captures {
				captureName := query.CaptureNameForId(capture.Index)
				node := capture.Node
				priority := p.GetInsertionPointPriority(captureName)
				if priority > bestPoint.Priority {
					bestPoint = types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: priority}
				}
			}
		}
		return bestPoint
	}
	return defaultPoint
}
func (p *PythonPlugin) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "initialize_otel") || strings.Contains(bodyContent, "TracerProvider") || strings.Contains(bodyContent, "set_tracer_provider")
}
func (p *PythonPlugin) findMainBlockWithRegex(content []byte, analysis *types.FileAnalysis) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `if __name__ == '__main__'`) || strings.Contains(trimmed, `if __name__ == "__main__"`) {
			insertionPoint := types.InsertionPoint{LineNumber: uint32(i + 2), Column: 1, Priority: 3}
			entryPoint := types.EntryPointInfo{
				Name:         "if __name__ == '__main__'",
				LineNumber:   uint32(i + 1),
				Column:       1,
				BodyStart:    insertionPoint,
				BodyEnd:      types.InsertionPoint{LineNumber: uint32(len(lines)), Column: 1},
				HasOTELSetup: false,
			}
			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
			break
		}
	}
}
func (p *PythonPlugin) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

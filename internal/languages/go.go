package languages

import (
	"fmt"
	"strings"

	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type GoPlugin struct {
	config *types.LanguageConfig
}

func NewGoPlugin() *GoPlugin {
	return &GoPlugin{
		config: &types.LanguageConfig{
			Language:       "Go",
			FileExtensions: []string{".go"},
			ImportQueries: map[string]string{
				"existing_imports": `
 (import_declaration
   (import_spec
     path: (interpreted_string_literal) @import_path
   ) @import_spec
 ) @import_declaration

 (import_declaration
   (import_spec
     path: (raw_string_literal) @import_path
   ) @import_spec
 ) @import_declaration
`,
			},
			FunctionQueries: map[string]string{
				"main_function": `
(function_declaration
  name: (identifier) @function_name
  body: (block) @function_body
  (#eq? @function_name "main"))

(function_declaration
  name: (identifier) @function_name
  body: (block) @init_function
  (#eq? @function_name "init"))
`,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
(block
  (var_declaration) @after_variables)

(block
  (short_var_declaration) @after_variables)

(block
  (assignment_statement) @after_variables)

(block
  (expression_statement 
    (call_expression)) @before_function_calls)

(block) @function_start
`,
			},
			ImportTemplate: `"go.opentelemetry.io/%s"`,
			InitializationTemplate: `
	// Initialize OpenTelemetry
	tp, err := SetupOTEL()
	if err != nil {
		log.Fatalf("Failed to initialize OTEL: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown tracer provider: %v", err)
		}
	}()
`,
			CleanupTemplate: `tp.Shutdown(context.Background())`,
		},
	}
}

func (p *GoPlugin) ID() string {
	return "go"
}
func (p *GoPlugin) DisplayName() string {
	return "Go"
}
func (p *GoPlugin) EntryPointTreeSitterLanguage() *sitter.Language {
	return golang.GetLanguage()
}
func (p *GoPlugin) EntrypointQuery() string {
	return `
                (function_declaration 
                    name: (identifier) @func_name
                    (#eq? @func_name "main")
                ) @main_function
            `
}
func (p *GoPlugin) FileExtensions() []string {
	return []string{".go"}
}

// Plugin acts as injector
func (p *GoPlugin) Injector() inj.LanguageInjector {
	return p
}
func (p *GoPlugin) Dependencies() dep.DependencyHandler {
	return dep.NewGoHandler()
}

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

// Injector methods
func (p *GoPlugin) GetTreeSitterLanguage() *sitter.Language { return golang.GetLanguage() }
func (p *GoPlugin) GetLanguage() *sitter.Language           { return golang.GetLanguage() }
func (p *GoPlugin) GetConfig() *types.LanguageConfig        { return p.config }
func (p *GoPlugin) GetRequiredImports() []string {
	return []string{
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/trace",
		"go.opentelemetry.io/otel/sdk/trace",
		"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		"context",
		"log",
	}
}
func (p *GoPlugin) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}
	var result strings.Builder
	if hasExistingImports {
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%q\n", importPath))
		}
	} else {
		result.WriteString("import (\n")
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%q\n", importPath))
		}
		result.WriteString(")\n")
	}
	return result.String()
}
func (p *GoPlugin) FormatSingleImport(importPath string) string { return fmt.Sprintf("%q", importPath) }
func (p *GoPlugin) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		importPath := strings.Trim(node.Content(content), "\"")
		analysis.ExistingImports[importPath] = true
		if strings.Contains(importPath, "go.opentelemetry.io") {
			analysis.HasOTELImports = true
		}
	case "import_spec":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: 3}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	case "import_declaration":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: 0, Context: node.Content(content), Priority: 2}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}
func (p *GoPlugin) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "function_body":
		insertionPoint := p.findBestInsertionPoint(node, content, config)
		hasOTELSetup := p.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasOTELSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	case "init_function":
		insertionPoint := p.findBestInsertionPoint(node, content, config)
		hasOTELSetup := p.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "init",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasOTELSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	}
}
func (p *GoPlugin) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "function_start":
		return 100
	case "after_variables":
		return 3
	case "after_imports":
		return 2
	case "before_function_calls":
		return 1
	default:
		return 1
	}
}
func (p *GoPlugin) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: bodyNode.StartPoint().Row + 1, Column: bodyNode.StartPoint().Column + 1, Priority: 1}
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), p.GetTreeSitterLanguage())
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
					if captureName == "function_start" {
						bestPoint = types.InsertionPoint{LineNumber: node.StartPoint().Row + 1, Column: node.StartPoint().Column + 1, Context: node.Content(content), Priority: priority}
					} else {
						bestPoint = types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: priority}
					}
				}
			}
		}
		return bestPoint
	}
	return defaultPoint
}
func (p *GoPlugin) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "trace.NewTracerProvider") ||
		strings.Contains(bodyContent, "otel.SetTracerProvider") ||
		strings.Contains(bodyContent, "SetupOTEL") ||
		strings.Contains(bodyContent, "setupTracing")
}

// Implement required method to satisfy injector.LanguageInjector
func (p *GoPlugin) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// RegisterGoPlugin registers the Go plugin with all necessary registries
func RegisterGoPlugin() {
	plugin := NewGoPlugin()
	DefaultRegistry.Register(plugin)
	inj.RegisterLanguageInjector("go", plugin)
	dep.RegisterDependencyHandler("go", dep.NewGoHandler())
}

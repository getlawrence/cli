package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// JavaScriptInjector implements LanguageInjector for JavaScript
type JavaScriptInjector struct {
	config *types.LanguageConfig
}

// NewJavaScriptInjector creates a new handler
func NewJavaScriptInjector() *JavaScriptInjector {
	return &JavaScriptInjector{
		config: &types.LanguageConfig{
			Language:       "JavaScript",
			FileExtensions: []string{".js", ".mjs"},
			ImportQueries: map[string]string{
				"existing_imports": `
                (import_statement (string) @import_path) @import_location
                (call_expression
                  function: (identifier) @require_ident
                  arguments: (arguments (string) @require_path)
                ) @import_location
            `,
			},
			FunctionQueries: map[string]string{
				"main_function": `
                (call_expression
                  function: (member_expression
                    object: (identifier)
                    property: (property_identifier) @method
                  )
                ) @server_listen
                (#eq? @method "listen")

                (program) @main_block
            `,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
                (program (lexical_declaration) @after_variables)
                (program (expression_statement (call_expression)) @before_function_calls)
                (program) @function_start
            `,
			},
			ImportTemplate: `const { %s } = require("%s")`,
			InitializationTemplate: `
const { setupOTel } = require('./otel'); 
setupOTel();
`,
			CleanupTemplate: `await sdk.shutdown()`,
		},
	}
}

// GetLanguage returns the tree-sitter language for JavaScript
func (h *JavaScriptInjector) GetLanguage() *sitter.Language { return javascript.GetLanguage() }

// GetConfig returns the language configuration
func (h *JavaScriptInjector) GetConfig() *types.LanguageConfig { return h.config }

// GetRequiredImports returns the list of imports needed for OTEL in JS
func (h *JavaScriptInjector) GetRequiredImports() []string {
	return []string{}
}

// FormatImports formats JS import statements (ESM)
func (h *JavaScriptInjector) FormatImports(imports []string, hasExisting bool) string {
	if len(imports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, imp := range imports {
		b.WriteString(fmt.Sprintf("import \"%s\"\n", imp))
	}
	return b.String()
}

// FormatSingleImport formats a single JS import
func (h *JavaScriptInjector) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("import \"%s\"\n", importPath)
}

// AnalyzeImportCapture records imports
func (h *JavaScriptInjector) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path", "require_path":
		path := strings.Trim(node.Content(content), "\"'")
		analysis.ExistingImports[path] = true
		if strings.Contains(path, "@opentelemetry/") {
			analysis.HasOTELImports = true
		}
	case "import_location":
		analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{
			LineNumber: node.EndPoint().Row + 1,
			Column:     node.EndPoint().Column + 1,
			Context:    node.Content(content),
			Priority:   2,
		})
	}
}

// AnalyzeFunctionCapture finds entry blocks
func (h *JavaScriptInjector) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "server_listen", "main_block":
		insertionPoint := h.findBestInsertionPoint(node, content, config)
		hasSetup := h.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	}
}

// GetInsertionPointPriority for JS
func (h *JavaScriptInjector) GetInsertionPointPriority(captureName string) int {
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

func (h *JavaScriptInjector) findBestInsertionPoint(node *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: node.StartPoint().Row + 1, Column: node.StartPoint().Column + 1, Priority: 1}
	if insertQuery, ok := config.InsertionQueries["optimal_insertion"]; ok {
		q, err := sitter.NewQuery([]byte(insertQuery), h.GetLanguage())
		if err != nil {
			return defaultPoint
		}
		defer q.Close()
		cur := sitter.NewQueryCursor()
		defer cur.Close()
		cur.Exec(q, node)
		best := defaultPoint
		for {
			m, ok := cur.NextMatch()
			if !ok {
				break
			}
			for _, c := range m.Captures {
				name := q.CaptureNameForId(c.Index)
				n := c.Node
				p := h.GetInsertionPointPriority(name)
				if p > best.Priority {
					best = types.InsertionPoint{LineNumber: n.EndPoint().Row + 1, Column: n.EndPoint().Column + 1, Context: n.Content(content), Priority: p}
				}
			}
		}
		return best
	}
	return defaultPoint
}

func (h *JavaScriptInjector) detectExistingOTELSetup(node *sitter.Node, content []byte) bool {
	body := node.Content(content)
	return strings.Contains(body, "@opentelemetry/sdk-node") || strings.Contains(body, "NodeSDK(")
}

// FallbackAnalyzeImports: no-op for JS
func (h *JavaScriptInjector) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// FallbackAnalyzeEntryPoints: no-op for JavaScript; default program node is already considered
func (h *JavaScriptInjector) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {
}

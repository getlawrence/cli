package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	csharp "github.com/smacker/go-tree-sitter/csharp"
)

// DotNetHandler implements LanguageInjector for C#/.NET
type DotNetHandler struct {
	config *types.LanguageConfig
}

func NewDotNetHandler() *DotNetHandler {
	return &DotNetHandler{
		config: &types.LanguageConfig{
			Language:       "csharp",
			FileExtensions: []string{".cs"},
			ImportQueries: map[string]string{
				"existing_imports": `
                (using_directive) @import_location
            `,
			},
			FunctionQueries: map[string]string{
				// Capture method bodies (we will not filter by name for compatibility across C# versions)
				"main_function": `
                (method_declaration
                  name: (identifier) @method_name
                  body: (block) @method_body
                )

                (global_statement) @global
            `,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
                (block (local_declaration_statement) @after_variables)
                (block (expression_statement (invocation_expression)) @before_function_calls)
                (block) @function_start
            `,
			},
			ImportTemplate: `using %s;`,
			InitializationTemplate: `
    // Initialize OpenTelemetry (basic tracing)
    builder.Services.AddOpenTelemetry()
        .WithTracing(tracing => tracing
            .AddAspNetCoreInstrumentation()
            .AddGrpcClientInstrumentation()
            .AddHttpClientInstrumentation()
            .AddRedisInstrumentation()
            .AddOtlpExporter());
`,
			CleanupTemplate: `// no-op`,
		},
	}
}

func (h *DotNetHandler) GetLanguage() *sitter.Language    { return csharp.GetLanguage() }
func (h *DotNetHandler) GetConfig() *types.LanguageConfig { return h.config }

func (h *DotNetHandler) GetRequiredImports() []string {
	return []string{
		"OpenTelemetry",
		"OpenTelemetry.Trace",
		"OpenTelemetry.Resources",
		"OpenTelemetry.Exporter",
		"Microsoft.Extensions.DependencyInjection",
	}
}

func (h *DotNetHandler) FormatImports(imports []string, hasExisting bool) string {
	if len(imports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, imp := range imports {
		b.WriteString(h.FormatSingleImport(imp))
		b.WriteString("\n")
	}
	return b.String()
}

func (h *DotNetHandler) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("using %s;", importPath)
}

func (h *DotNetHandler) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		path := strings.TrimSpace(node.Content(content))
		analysis.ExistingImports[path] = true
		if strings.Contains(path, "OpenTelemetry") {
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

func (h *DotNetHandler) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "method_name":
		// handled together with body capture
	case "method_body", "global":
		insertion := h.findBestInsertionPoint(node, content, config)
		entry := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertion,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: h.detectExistingOTELSetup(node, content),
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entry)
	}
}

func (h *DotNetHandler) GetInsertionPointPriority(captureName string) int {
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

func (h *DotNetHandler) findBestInsertionPoint(node *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
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

func (h *DotNetHandler) detectExistingOTELSetup(node *sitter.Node, content []byte) bool {
	body := node.Content(content)
	return strings.Contains(body, "AddOpenTelemetry(") || strings.Contains(body, "WithTracing(")
}

func (h *DotNetHandler) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis)     {}
func (h *DotNetHandler) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {}

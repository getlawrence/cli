package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/java"
)

// JavaInjector implements LanguageInjector for Java
type JavaInjector struct {
	config *types.LanguageConfig
}

// NewJavaInjector creates a new Java language handler
func NewJavaInjector() *JavaInjector {
	return &JavaInjector{
		config: &types.LanguageConfig{
			Language:       "Java",
			FileExtensions: []string{".java"},
			ImportQueries: map[string]string{
				"existing_imports": `
                (import_declaration
                    (scoped_identifier) @import_path
                ) @import_location
            `,
			},
			FunctionQueries: map[string]string{
				// Try to capture main method bodies
				"main_function": `
                (method_declaration
                  name: (identifier) @method_name
                  body: (block) @method_body
                  (#eq? @method_name "main")
                )
            `,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
                (block (local_variable_declaration) @after_variables)
                (block (expression_statement (method_invocation)) @before_function_calls)
                (block) @function_start
            `,
			},
			ImportTemplate: `import %s;`,
			InitializationTemplate: `
        // Initialize OpenTelemetry
        io.opentelemetry.api.GlobalOpenTelemetry.get(); // ensure OTEL available
`,
			CleanupTemplate: `// no-op cleanup for basic setup`,
		},
	}
}

// GetLanguage returns the tree-sitter language parser for Java
func (h *JavaInjector) GetLanguage() *sitter.Language { return java.GetLanguage() }

// GetConfig returns the language configuration for Java
func (h *JavaInjector) GetConfig() *types.LanguageConfig { return h.config }

// GetRequiredImports returns the list of imports needed for OTEL in Java
func (h *JavaInjector) GetRequiredImports() []string {
	return []string{
		"io.opentelemetry.api",
		"io.opentelemetry.sdk",
		"io.opentelemetry.trace",
		"io.opentelemetry.exporters.otlp.trace.OtlpGrpcSpanExporter",
		"io.opentelemetry.sdk.resources.Resource",
		"io.opentelemetry.sdk.trace.SdkTracerProvider",
		"io.opentelemetry.sdk.trace.export.BatchSpanProcessor",
	}
}

// GetFrameworkImports returns framework-specific imports based on detected frameworks
func (h *JavaInjector) GetFrameworkImports(content []byte) []string {
	// Java doesn't have framework-specific imports like Python
	return []string{}
}

// FormatFrameworkImports formats framework-specific import statements for Java
func (h *JavaInjector) FormatFrameworkImports(imports []string) string {
	// Java doesn't have framework-specific imports like Python
	return ""
}

// GenerateFrameworkModifications generates framework-specific instrumentation modifications for Java
func (h *JavaInjector) GenerateFrameworkModifications(content []byte, operationsData *types.OperationsData) []types.CodeModification {
	// Java doesn't have framework-specific modifications like Python
	return []types.CodeModification{}
}

// FormatImports formats Java import statements
func (h *JavaInjector) FormatImports(imports []string, hasExistingImports bool) string {
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

// FormatSingleImport formats a single Java import statement
func (h *JavaInjector) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("import %s;", importPath)
}

// AnalyzeImportCapture processes an import capture from tree-sitter query for Java
func (h *JavaInjector) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		path := strings.TrimSpace(node.Content(content))
		analysis.ExistingImports[path] = true
		if strings.HasPrefix(path, "io.opentelemetry.") {
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

// AnalyzeFunctionCapture processes a function capture from tree-sitter query for Java
func (h *JavaInjector) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "method_name":
		// handled with body capture
	case "method_body":
		insertionPoint := h.findBestInsertionPoint(node, content, config)
		hasOTELSetup := h.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasOTELSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	}
}

// GetInsertionPointPriority returns priority for Java insertion point types
func (h *JavaInjector) GetInsertionPointPriority(captureName string) int {
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

func (h *JavaInjector) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: bodyNode.StartPoint().Row + 1, Column: bodyNode.StartPoint().Column + 1, Priority: 1}
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		q, err := sitter.NewQuery([]byte(insertQuery), h.GetLanguage())
		if err != nil {
			return defaultPoint
		}
		defer q.Close()
		cur := sitter.NewQueryCursor()
		defer cur.Close()
		cur.Exec(q, bodyNode)
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

func (h *JavaInjector) detectExistingOTELSetup(node *sitter.Node, content []byte) bool {
	body := node.Content(content)
	return strings.Contains(body, "SdkTracerProvider") || strings.Contains(body, "GlobalOpenTelemetry")
}

// FallbackAnalyzeImports: no-op for Java
func (h *JavaInjector) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// FallbackAnalyzeEntryPoints: no-op for Java; main method capture should be sufficient
func (h *JavaInjector) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {}

// GenerateImportModifications generates modifications to fix import statements
func (h *JavaInjector) GenerateImportModifications(content []byte, analysis *types.FileAnalysis) []types.CodeModification {
	// No special import handling needed for Java
	return []types.CodeModification{}
}

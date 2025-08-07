package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonHandler implements LanguageHandler for Python
type PythonHandler struct {
	config *types.LanguageConfig
}

// NewPythonHandler creates a new Python language handler
func NewPythonHandler() *PythonHandler {
	return &PythonHandler{
		config: &types.LanguageConfig{
			Language:       "Python",
			FileExtensions: []string{".py", ".pyw"},
			ImportQueries: map[string]string{
				"existing_imports": `
				(import_statement 
					name: (dotted_name) @import_path
				) @import_location
				(import_from_statement
					module_name: (dotted_name) @import_path
				) @import_location
			`,
			},
			FunctionQueries: map[string]string{
				"main_function": `
				(function_definition 
					name: (identifier) @function_name
					body: (block) @function_body
					(#eq? @function_name "main")
				)
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
    tp = initialize_otel()
`,
			CleanupTemplate: `tp.shutdown()`,
		},
	}
}

// GetLanguage returns the tree-sitter language parser for Python
func (h *PythonHandler) GetLanguage() *sitter.Language {
	return python.GetLanguage()
}

// GetConfig returns the language configuration for Python
func (h *PythonHandler) GetConfig() *types.LanguageConfig {
	return h.config
}

// GetRequiredImports returns the list of imports needed for OTEL in Python
func (h *PythonHandler) GetRequiredImports() []string {
	return []string{
		"opentelemetry.sdk.trace",
		"opentelemetry.exporter.otlp.proto.http.trace_exporter",
	}
}

// FormatImports formats Python import statements
func (h *PythonHandler) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}

	var result strings.Builder
	for _, importPath := range imports {
		result.WriteString(h.FormatSingleImport(importPath))
	}
	return result.String()
}

// FormatSingleImport formats a single Python import statement
func (h *PythonHandler) FormatSingleImport(importPath string) string {
	if strings.Contains(importPath, ".") {
		parts := strings.Split(importPath, ".")
		return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	}
	return fmt.Sprintf("import %s\n", importPath)
}

// AnalyzeImportCapture processes an import capture from tree-sitter query for Python
func (h *PythonHandler) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		importPath := node.Content(content)
		analysis.ExistingImports[importPath] = true

		// Check if it's an OTEL import
		if strings.Contains(importPath, "opentelemetry") {
			analysis.HasOTELImports = true
		}

	case "import_location":
		// Record location where imports can be added
		insertionPoint := types.InsertionPoint{
			LineNumber: node.EndPoint().Row + 1,
			Column:     node.EndPoint().Column + 1,
			Context:    node.Content(content),
			Priority:   2, // Medium priority for existing import locations
		}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}

// AnalyzeFunctionCapture processes a function capture from tree-sitter query for Python
func (h *PythonHandler) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "function_name":
		functionName := node.Content(content)
		if functionName == "main" {
			// This is the main function - we'll handle the body in the next capture
		}

	case "function_body":
		// Find the best insertion point within the function body
		insertionPoint := h.findBestInsertionPoint(node, content, config)

		// Check if OTEL setup already exists
		hasOTELSetup := h.detectExistingOTELSetup(node, content)

		entryPoint := types.EntryPointInfo{
			Name:       "main",
			LineNumber: node.StartPoint().Row + 1,
			Column:     node.StartPoint().Column + 1,
			BodyStart:  insertionPoint,
			BodyEnd: types.InsertionPoint{
				LineNumber: node.EndPoint().Row + 1,
				Column:     node.EndPoint().Column + 1,
			},
			HasOTELSetup: hasOTELSetup,
		}

		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)

	case "name_var":
		// Handle if __name__ == "__main__": pattern
	case "main_str":
		// Handle if __name__ == "__main__": pattern
	}
}

// GetInsertionPointPriority returns priority for Python insertion point types
func (h *PythonHandler) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "after_variables":
		return 3 // High priority - after variable declarations
	case "before_function_calls":
		return 2 // Medium priority - before function calls
	case "function_start":
		return 1 // Low priority - start of function
	default:
		return 1
	}
}

// findBestInsertionPoint finds the optimal location to insert OTEL initialization code in Python
func (h *PythonHandler) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	// Default to the beginning of the function body
	defaultPoint := types.InsertionPoint{
		LineNumber: bodyNode.StartPoint().Row + 1,
		Column:     bodyNode.StartPoint().Column + 1,
		Priority:   1,
	}

	// Use language-specific insertion query if available
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), h.GetLanguage())
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

				priority := h.GetInsertionPointPriority(captureName)

				if priority > bestPoint.Priority {
					bestPoint = types.InsertionPoint{
						LineNumber: node.EndPoint().Row + 1,
						Column:     node.EndPoint().Column + 1,
						Context:    node.Content(content),
						Priority:   priority,
					}
				}
			}
		}

		return bestPoint
	}

	return defaultPoint
}

// detectExistingOTELSetup checks if OTEL initialization code already exists in Python
func (h *PythonHandler) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "initialize_otel") ||
		strings.Contains(bodyContent, "TracerProvider") ||
		strings.Contains(bodyContent, "set_tracer_provider")
}

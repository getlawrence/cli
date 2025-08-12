package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/python"
)

// PythonInjector implements LanguageHandler for Python
type PythonInjector struct {
	config *types.LanguageConfig
}

// NewPythonInjector creates a new Python language handler
func NewPythonInjector() *PythonInjector {
	return &PythonInjector{
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
    tracer_provider = init_tracer()
`,
			CleanupTemplate: `tp.shutdown()`,
		},
	}
}

// GetLanguage returns the tree-sitter language parser for Python
func (h *PythonInjector) GetLanguage() *sitter.Language {
	return python.GetLanguage()
}

// GetConfig returns the language configuration for Python
func (h *PythonInjector) GetConfig() *types.LanguageConfig {
	return h.config
}

// GetRequiredImports returns the list of imports needed for OTEL in Python
func (h *PythonInjector) GetRequiredImports() []string {
	// Rely on the generated otel.py bootstrap to handle OTEL imports.
	// Avoid injecting imports into user files to prevent syntax issues.
	return []string{}
}

// FormatImports formats Python import statements
func (h *PythonInjector) FormatImports(imports []string, hasExistingImports bool) string {
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
func (h *PythonInjector) FormatSingleImport(importPath string) string {
	if strings.Contains(importPath, ".") {
		parts := strings.Split(importPath, ".")
		return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	}
	return fmt.Sprintf("import %s\n", importPath)
}

// AnalyzeImportCapture processes an import capture from tree-sitter query for Python
func (h *PythonInjector) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
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
func (h *PythonInjector) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_if_block":
		// Handle if __name__ == "__main__": pattern
		// The node represents the entire if statement, we need to find its body
		var bodyNode *sitter.Node

		// Navigate to the body of the if statement
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "block" {
				bodyNode = child
				break
			}
		}

		if bodyNode != nil {
			insertionPoint := h.findBestInsertionPoint(bodyNode, content, config)

			// Check if OTEL setup already exists
			hasOTELSetup := h.detectExistingOTELSetup(bodyNode, content)

			entryPoint := types.EntryPointInfo{
				Name:       "if __name__ == '__main__'",
				LineNumber: bodyNode.StartPoint().Row + 1, // Use body's start position for insertion
				Column:     bodyNode.StartPoint().Column + 1,
				BodyStart:  insertionPoint,
				BodyEnd: types.InsertionPoint{
					LineNumber: bodyNode.EndPoint().Row + 1,
					Column:     bodyNode.EndPoint().Column + 1,
				},
				HasOTELSetup: hasOTELSetup,
			}

			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
		}

	case "name_var", "main_str":
		// These are part of the condition parsing, handled as part of main_if_block
		// No separate processing needed
	}

	// Always use regex fallback for now to ensure it works
	// TODO: Re-enable tree-sitter matching once it's working correctly
	h.findMainBlockWithRegex(content, analysis)
}

// findMainBlockWithRegex uses regex to find the if __name__ == '__main__': pattern
func (h *PythonInjector) findMainBlockWithRegex(content []byte, analysis *types.FileAnalysis) {
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `if __name__ == '__main__'`) || strings.Contains(trimmed, `if __name__ == "__main__"`) {
			// Found the main block, create an entry point
			// The insertion point should be right after this line
			insertionPoint := types.InsertionPoint{
				LineNumber: uint32(i + 2), // Next line after the if statement
				Column:     1,
				Priority:   3,
			}

			entryPoint := types.EntryPointInfo{
				Name:       "if __name__ == '__main__'",
				LineNumber: uint32(i + 1), // Line number is 1-based
				Column:     1,
				BodyStart:  insertionPoint,
				BodyEnd: types.InsertionPoint{
					LineNumber: uint32(len(lines)), // End of file
					Column:     1,
				},
				HasOTELSetup: false, // Simple regex-based detection doesn't check for existing setup
			}

			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
			break // Only add one entry point
		}
	}
} // GetInsertionPointPriority returns priority for Python insertion point types
func (h *PythonInjector) GetInsertionPointPriority(captureName string) int {
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
func (h *PythonInjector) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
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
func (h *PythonInjector) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	if strings.Contains(bodyContent, "initialize_otel") ||
		strings.Contains(bodyContent, "TracerProvider") ||
		strings.Contains(bodyContent, "set_tracer_provider") {
		return true
	}
	// Detect bootstrap usage inserted by our template
	if strings.Contains(bodyContent, "from otel import init_tracer") ||
		strings.Contains(bodyContent, "import otel") ||
		strings.Contains(bodyContent, "init_tracer(") {
		return true
	}
	return false
}

// FallbackAnalyzeImports for Python: no-op since tree-sitter captures are sufficient
func (h *PythonInjector) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {
	// Intentionally empty
}

// FallbackAnalyzeEntryPoints uses regex to find the if __name__ == '__main__' block when tree-sitter misses it
func (h *PythonInjector) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {
	h.findMainBlockWithRegex(content, analysis)
}

package injector

import (
	"fmt"
	"slices"
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
			ImportTemplate:         `from opentelemetry import %s`,
			InitializationTemplate: `init_tracer()`,
			CleanupTemplate:        `tp.shutdown()`,
			FrameworkTemplates: map[string]string{
				"flask": `
# Instrument Flask application
FlaskInstrumentor().instrument_app(app)
`,
			},
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
	// Add the otel import for the generated bootstrap file
	return []string{"otel"}
}

// GetFrameworkImports returns framework-specific imports based on detected frameworks
func (h *PythonInjector) GetFrameworkImports(content []byte) []string {
	var imports []string

	// Detect Flask usage
	if h.detectFlaskUsage(content) {
		imports = append(imports, "opentelemetry.instrumentation.flask")
	}

	return imports
}

// detectFlaskUsage checks if the code uses Flask
func (h *PythonInjector) detectFlaskUsage(content []byte) bool {
	contentStr := string(content)
	return strings.Contains(contentStr, "from flask import") ||
		strings.Contains(contentStr, "import flask") ||
		strings.Contains(contentStr, "Flask(")
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
	// Special handling for Flask instrumentation
	if importPath == "opentelemetry.instrumentation.flask" {
		return "from opentelemetry.instrumentation.flask import FlaskInstrumentor\n"
	}

	// Special handling for otel import - should import init_tracer function
	if importPath == "otel" {
		return "from otel import init_tracer\n"
	}

	if strings.Contains(importPath, ".") {
		parts := strings.Split(importPath, ".")
		return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	}
	return fmt.Sprintf("import %s\n", importPath)
}

// FormatFrameworkImports formats framework-specific import statements for Python
func (h *PythonInjector) FormatFrameworkImports(imports []string) string {
	if len(imports) == 0 {
		return ""
	}

	var result strings.Builder
	for _, importPath := range imports {
		result.WriteString(h.FormatSingleImport(importPath))
	}
	return result.String()
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

		// Special handling for 'otel' import - we need to replace this with 'from otel import init_tracer'
		if importPath == "otel" {
			analysis.HasOTELImports = true
			// Mark this as needing replacement
			analysis.ExistingImports["otel"] = false // Mark as invalid import
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

	// Ensure we also consider inserting before Flask app creation
	h.findMainBlockWithRegex(content, analysis)
	h.findFlaskAppCreation(content, analysis)
}

// GetInsertionPointPriority returns priority for Python insertion point types
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

// findFlaskAppCreation detects `app = Flask(__name__)` and proposes an insertion point just before it.
func (h *PythonInjector) findFlaskAppCreation(content []byte, analysis *types.FileAnalysis) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		if strings.Contains(line, "Flask(") && strings.Contains(line, "__name__") && strings.Contains(line, "=") {
			// Insert on the prior line to ensure OTEL is initialized before app is created
			lineNo := i + 1
			insertion := types.InsertionPoint{LineNumber: uint32(lineNo), Column: 1, Priority: 4}
			entry := types.EntryPointInfo{
				Name:       "flask_app_creation",
				LineNumber: uint32(lineNo),
				Column:     1,
				BodyStart:  insertion,
				BodyEnd:    types.InsertionPoint{LineNumber: uint32(len(lines)), Column: 1},
			}
			analysis.EntryPoints = append([]types.EntryPointInfo{entry}, analysis.EntryPoints...)
			return
		}
	}
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
		strings.Contains(bodyContent, "init_tracer(") {
		return true
	}
	return false
}

// GenerateImportModifications generates modifications to fix import statements
func (h *PythonInjector) GenerateImportModifications(content []byte, analysis *types.FileAnalysis) []types.CodeModification {
	var modifications []types.CodeModification

	// Always check if we need to add the correct import
	contentStr := string(content)
	hasCorrectImport := strings.Contains(contentStr, "from otel import init_tracer")
	hasIncorrectImport := strings.Contains(contentStr, "import otel")

	if !hasCorrectImport {
		// Find the best place to add the import
		lines := strings.Split(contentStr, "\n")
		insertLine := 2 // Default to after the first import

		// Look for existing imports to place after them
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "from ") || strings.HasPrefix(trimmed, "import ") {
				insertLine = i + 2 // Next line after this import
			}
		}

		// Add the correct import
		modifications = append(modifications, types.CodeModification{
			Type:        types.ModificationAddImport,
			Language:    "Python",
			FilePath:    "", // Will be set by caller
			LineNumber:  uint32(insertLine),
			Column:      1,
			InsertAfter: false,
			Content:     "from otel import init_tracer",
			Context:     "",
		})
	}

	// If there's an incorrect import, remove it
	if hasIncorrectImport {
		lines := strings.Split(contentStr, "\n")
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if trimmed == "import otel" {
				modifications = append(modifications, types.CodeModification{
					Type:        types.ModificationRemoveLine,
					Language:    "Python",
					FilePath:    "", // Will be set by caller
					LineNumber:  uint32(i + 1),
					Column:      1,
					InsertAfter: false,
					Content:     "", // Empty content for removal
					Context:     line,
				})
				break
			}
		}
	}

	return modifications
}

// GenerateFrameworkModifications generates framework-specific instrumentation modifications
func (h *PythonInjector) GenerateFrameworkModifications(content []byte, operationsData *types.OperationsData) []types.CodeModification {
	var modifications []types.CodeModification

	// First, handle any import modifications that are needed
	// This will handle replacing 'import otel' with 'from otel import init_tracer'
	// We need to analyze the content to detect existing imports
	analysis := &types.FileAnalysis{
		ExistingImports: make(map[string]bool),
	}

	// Check if there's an 'import otel' statement that needs replacement
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "import otel" {
			analysis.ExistingImports["otel"] = false // Mark as needing replacement
			break
		}
	}

	importMods := h.GenerateImportModifications(content, analysis)
	modifications = append(modifications, importMods...)

	// Check if Flask instrumentation is needed
	if h.detectFlaskUsage(content) && h.hasFlaskInstrumentation(operationsData) {
		// First, check if there are any existing Flask instrumentation issues to clean up
		cleanupMods := h.generateFlaskCleanupModifications(content)
		modifications = append(modifications, cleanupMods...)

		// Find the best place to inject Flask instrumentation
		insertionPoint := h.findFlaskInstrumentationPoint(content)
		if insertionPoint.LineNumber > 0 {
			// Create the complete Flask instrumentation content including import
			flaskContent := "from opentelemetry.instrumentation.flask import FlaskInstrumentor\n\n" + h.config.FrameworkTemplates["flask"]

			modifications = append(modifications, types.CodeModification{
				Type:        types.ModificationAddFramework,
				Language:    "Python",
				FilePath:    "", // Will be set by caller
				LineNumber:  insertionPoint.LineNumber,
				Column:      insertionPoint.Column,
				InsertAfter: true,
				Content:     flaskContent,
				Framework:   "flask",
			})
		}
	}

	return modifications
}

// generateFlaskCleanupModifications generates modifications to clean up incorrectly placed Flask instrumentation
func (h *PythonInjector) generateFlaskCleanupModifications(content []byte) []types.CodeModification {
	var modifications []types.CodeModification
	lines := strings.Split(string(content), "\n")

	// Look for incorrectly placed Flask instrumentation code that needs to be removed
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)

		// Remove duplicate/incorrect imports - be aggressive about this
		if strings.Contains(trimmed, "from opentelemetry.instrumentation import flask") {
			// Remove the incorrect import - we'll add the correct one back
			modifications = append(modifications, types.CodeModification{
				Type:        types.ModificationRemoveLine,
				Language:    "Python",
				FilePath:    "", // Will be set by caller
				LineNumber:  uint32(i + 1),
				Column:      1,
				InsertAfter: false,
				Content:     "", // Empty content for removal
				Context:     trimmed,
			})
		}

		// Remove the old incorrect import pattern that was being generated
		if strings.Contains(trimmed, "from opentelemetry.instrumentation.flask import FlaskInstrumentor") {
			// Only remove this if it's in the wrong location (not near Flask app creation)
			if !h.isFlaskImportInRightPlace(lines, i) {
				modifications = append(modifications, types.CodeModification{
					Type:        types.ModificationRemoveLine,
					Language:    "Python",
					FilePath:    "", // Will be set by caller
					LineNumber:  uint32(i + 1),
					Column:      1,
					InsertAfter: false,
					Content:     "", // Empty content for removal
					Context:     trimmed,
				})
			}
		}

		// Remove incorrectly placed Flask instrumentation code - be aggressive about this
		if strings.Contains(trimmed, "FlaskInstrumentor().instrument_app(app)") {
			// Only remove this if it's not near Flask app creation
			if !h.isFlaskInstrumentationInRightPlace(lines, i) {
				modifications = append(modifications, types.CodeModification{
					Type:        types.ModificationRemoveLine,
					Language:    "Python",
					FilePath:    "", // Will be set by caller
					LineNumber:  uint32(i + 1),
					Column:      1,
					InsertAfter: false,
					Content:     "", // Empty content for removal
					Context:     trimmed,
				})
			}
		}

		// Also remove any comment lines that mention Flask instrumentation
		if strings.Contains(trimmed, "# Instrument Flask application") {
			modifications = append(modifications, types.CodeModification{
				Type:        types.ModificationRemoveLine,
				Language:    "Python",
				FilePath:    "", // Will be set by caller
				LineNumber:  uint32(i + 1),
				Column:      1,
				InsertAfter: false,
				Content:     "", // Empty content for removal
				Context:     trimmed,
			})
		}
	}

	return modifications
}

// isFlaskImportInRightPlace checks if a Flask instrumentation import is in the right location
func (h *PythonInjector) isFlaskImportInRightPlace(lines []string, importLineIndex int) bool {
	// Look for Flask app creation within a reasonable distance
	start := max(0, importLineIndex-10)
	end := min(len(lines), importLineIndex+10)

	for i := start; i < end; i++ {
		if strings.Contains(lines[i], "Flask(") && strings.Contains(lines[i], "__name__") && strings.Contains(lines[i], "=") {
			return true
		}
	}
	return false
}

// isFlaskInstrumentationInRightPlace checks if Flask instrumentation code is in the right location
func (h *PythonInjector) isFlaskInstrumentationInRightPlace(lines []string, lineIndex int) bool {
	// Look for Flask app creation within a reasonable distance
	start := max(0, lineIndex-10)
	end := min(len(lines), lineIndex+10)

	for i := start; i < end; i++ {
		if strings.Contains(lines[i], "Flask(") && strings.Contains(lines[i], "__name__") && strings.Contains(lines[i], "=") {
			return true
		}
	}
	return false
}

// Helper function for max
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Helper function for min
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// hasFlaskInstrumentation checks if Flask instrumentation is planned to be installed
func (h *PythonInjector) hasFlaskInstrumentation(operationsData *types.OperationsData) bool {
	instrumentations := operationsData.InstallComponents["instrumentation"]
	return len(instrumentations) > 0 && slices.Contains(instrumentations, "flask")
}

// findFlaskInstrumentationPoint finds the best place to inject Flask instrumentation
func (h *PythonInjector) findFlaskInstrumentationPoint(content []byte) types.InsertionPoint {
	lines := strings.Split(string(content), "\n")

	// Look for Flask app creation: app = Flask(__name__)
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "Flask(") && strings.Contains(trimmed, "__name__") && strings.Contains(trimmed, "=") {
			// Insert after the Flask app creation line
			return types.InsertionPoint{
				LineNumber: uint32(i + 2), // Next line after Flask app creation
				Column:     1,
				Priority:   5, // High priority for framework instrumentation
			}
		}
	}

	// Fallback: look for any Flask import or usage
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, "from flask import") || strings.Contains(trimmed, "import flask") {
			// Insert after the Flask import
			return types.InsertionPoint{
				LineNumber: uint32(i + 2),
				Column:     1,
				Priority:   4,
			}
		}
	}

	// Default: insert at the end of the file
	return types.InsertionPoint{
		LineNumber: uint32(len(lines) + 1),
		Column:     1,
		Priority:   1,
	}
}

// FallbackAnalyzeImports for Python: no-op since tree-sitter captures are sufficient
func (h *PythonInjector) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {
	// Intentionally empty
}

// FallbackAnalyzeEntryPoints uses regex to find the if __name__ == '__main__' block when tree-sitter misses it
func (h *PythonInjector) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {
	h.findMainBlockWithRegex(content, analysis)
}

package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

// GoHandler implements LanguageHandler for Go
type GoHandler struct {
	config *types.LanguageConfig
}

// NewGoHandler creates a new Go language handler
func NewGoHandler() *GoHandler {
	return &GoHandler{
		config: &types.LanguageConfig{
			Language:       "Go",
			FileExtensions: []string{".go"},
			ImportQueries: map[string]string{
				// Capture existing imports and their locations so we can append instead of creating a new block
				// This captures:
				// - import path strings within import specs as @import_path
				// - individual import specs as @import_spec (good insertion point inside an existing block)
				// - the entire import declaration as @import_declaration (fallback insertion point after imports)
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

// GetLanguage returns the tree-sitter language parser for Go
func (h *GoHandler) GetLanguage() *sitter.Language {
	return golang.GetLanguage()
}

// GetConfig returns the language configuration for Go
func (h *GoHandler) GetConfig() *types.LanguageConfig {
	return h.config
}

// GetRequiredImports returns the list of imports needed for OTEL in Go
func (h *GoHandler) GetRequiredImports() []string {
	return []string{
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/trace",
		"go.opentelemetry.io/otel/sdk/trace",
		"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		"context",
		"log",
	}
}

// FormatImports formats Go import statements
func (h *GoHandler) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}

	var result strings.Builder

	if hasExistingImports {
		// Add to existing import block - just list the imports (no wrapping import (...))
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%s\n", h.FormatSingleImport(importPath)))
		}
	} else {
		// Create new import block
		result.WriteString("import (\n")
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%s\n", h.FormatSingleImport(importPath)))
		}
		result.WriteString(")\n")
	}

	return result.String()
}

// FormatSingleImport formats a single Go import statement
func (h *GoHandler) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("\"%s\"", importPath)
}

// AnalyzeImportCapture processes an import capture from tree-sitter query for Go
func (h *GoHandler) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		// Extract import path without quotes
		importPath := strings.Trim(node.Content(content), "\"")
		analysis.ExistingImports[importPath] = true

		// Check if it's an OTEL import
		if strings.Contains(importPath, "go.opentelemetry.io") {
			analysis.HasOTELImports = true
		}

	case "import_spec":
		// Record location where imports can be added within import block
		insertionPoint := types.InsertionPoint{
			LineNumber: node.EndPoint().Row + 1,
			Column:     node.EndPoint().Column + 1,
			Context:    node.Content(content),
			Priority:   3, // High priority for existing import blocks
		}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)

	case "import_declaration":
		// Record location after import declarations
		insertionPoint := types.InsertionPoint{
			LineNumber: node.EndPoint().Row + 1,
			Column:     0,
			Context:    node.Content(content),
			Priority:   2, // Medium priority for after imports
		}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}

// AnalyzeFunctionCapture processes a function capture from tree-sitter query for Go
func (h *GoHandler) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
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

	case "init_function":
		// Handle init() functions which are also entry points in Go
		insertionPoint := h.findBestInsertionPoint(node, content, config)
		hasOTELSetup := h.detectExistingOTELSetup(node, content)

		entryPoint := types.EntryPointInfo{
			Name:       "init",
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
	}
}

// GetInsertionPointPriority returns priority for Go insertion point types
func (h *GoHandler) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "function_start":
		return 100 // Highest priority - always start of function
	case "after_variables":
		return 3 // After variable declarations
	case "after_imports":
		return 2 // After other imports
	case "before_function_calls":
		return 1 // Before function calls
	default:
		return 1
	}
}

// findBestInsertionPoint finds the optimal location to insert OTEL initialization code in Go
func (h *GoHandler) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
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
					// For function_start we want the very beginning of the block
					if captureName == "function_start" {
						bestPoint = types.InsertionPoint{
							LineNumber: node.StartPoint().Row + 1,
							Column:     node.StartPoint().Column + 1,
							Context:    node.Content(content),
							Priority:   priority,
						}
					} else {
						bestPoint = types.InsertionPoint{
							LineNumber: node.EndPoint().Row + 1,
							Column:     node.EndPoint().Column + 1,
							Context:    node.Content(content),
							Priority:   priority,
						}
					}
				}
			}
		}

		return bestPoint
	}

	return defaultPoint
}

// detectExistingOTELSetup checks if OTEL initialization code already exists in Go
func (h *GoHandler) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "trace.NewTracerProvider") ||
		strings.Contains(bodyContent, "otel.SetTracerProvider") ||
		strings.Contains(bodyContent, "SetupOTEL") ||
		strings.Contains(bodyContent, "setupTracing")
}

// FallbackAnalyzeImports provides Go-specific import analysis when tree-sitter yields no insertion points
func (h *GoHandler) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {
	// Only apply if this is Go
	if analysis.Language != "Go" {
		return
	}

	lines := strings.Split(string(content), "\n")
	inBlock := false
	lastSpecLine := -1
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "import (") {
			inBlock = true
			continue
		}
		if inBlock {
			if trim == ")" {
				if lastSpecLine >= 0 {
					analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{
						LineNumber: uint32(lastSpecLine + 1),
						Column:     1,
						Context:    lines[lastSpecLine],
						Priority:   3,
					})
				} else {
					analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{
						LineNumber: uint32(i),
						Column:     1,
						Context:    trim,
						Priority:   2,
					})
				}
				inBlock = false
				continue
			}
			// Track last import spec line inside block
			if strings.HasPrefix(trim, "\"") || strings.HasPrefix(trim, "\t\"") {
				lastSpecLine = i
				// Record existing import path without quotes if we can
				imp := strings.TrimSpace(strings.Trim(trim, "\""))
				if imp != "" {
					analysis.ExistingImports[imp] = true
					if strings.Contains(imp, "go.opentelemetry.io/") {
						analysis.HasOTELImports = true
					}
				}
			}
		}
	}
	// Also handle single-line import form: import "pkg"
	for i, line := range lines {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "import \"") && strings.HasSuffix(trim, "\"") {
			// Insert after this line
			analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{
				LineNumber: uint32(i + 1),
				Column:     1,
				Context:    trim,
				Priority:   2,
			})
			imp := strings.TrimPrefix(trim, "import ")
			imp = strings.Trim(imp, "\"")
			if imp != "" {
				analysis.ExistingImports[imp] = true
				if strings.Contains(imp, "go.opentelemetry.io/") {
					analysis.HasOTELImports = true
				}
			}
		}
	}
}

// FallbackAnalyzeEntryPoints: no-op for Go since tree-sitter captures main/init reliably
func (h *GoHandler) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {}

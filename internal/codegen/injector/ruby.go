package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	rubylang "github.com/smacker/go-tree-sitter/ruby"
)

// RubyHandler implements LanguageInjector for Ruby
type RubyHandler struct {
	config *types.LanguageConfig
}

// NewRubyHandler creates a new Ruby language handler
func NewRubyHandler() *RubyHandler {
	return &RubyHandler{
		config: &types.LanguageConfig{
			Language:       "Ruby",
			FileExtensions: []string{".rb"},
			ImportQueries: map[string]string{
				"existing_imports": `
                (program (call
                  method: (identifier) @require_kw
                  arguments: (argument_list (string) @import_path)
                ) @import_location)
            `,
			},
			FunctionQueries: map[string]string{
				// Ruby doesn't have a canonical main, so treat entire program as entry block
				"main_function": `
                (program) @main_block
            `,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
                (program) @function_start
            `,
			},
			ImportTemplate: `require "%s"`,
			InitializationTemplate: `
# Initialize OpenTelemetry
require_relative "./otel"
Lawrence::OTel.setup
`,
			CleanupTemplate: ``,
		},
	}
}

// GetLanguage returns the tree-sitter language for Ruby
func (h *RubyHandler) GetLanguage() *sitter.Language { return rubylang.GetLanguage() }

// GetConfig returns the language configuration
func (h *RubyHandler) GetConfig() *types.LanguageConfig { return h.config }

// GetRequiredImports returns the list of requires needed for OTEL in Ruby
func (h *RubyHandler) GetRequiredImports() []string {
	return []string{
		"opentelemetry-sdk",
		"opentelemetry-exporter-otlp",
	}
}

// FormatImports formats Ruby require statements
func (h *RubyHandler) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, imp := range imports {
		b.WriteString(h.FormatSingleImport(imp))
	}
	return b.String()
}

// FormatSingleImport formats a single Ruby require
func (h *RubyHandler) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("require \"%s\"\n", importPath)
}

// AnalyzeImportCapture processes import captures
func (h *RubyHandler) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		path := strings.Trim(node.Content(content), "\"'")
		analysis.ExistingImports[path] = true
		if strings.HasPrefix(path, "opentelemetry") {
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

// AnalyzeFunctionCapture marks the whole program as main block
func (h *RubyHandler) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_block":
		insertionPoint := types.InsertionPoint{LineNumber: node.StartPoint().Row + 1, Column: node.StartPoint().Column + 1, Priority: 1}
		entryPoint := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: h.detectExistingOTELSetup(node, content),
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	}
}

// GetInsertionPointPriority returns priority for insertion types
func (h *RubyHandler) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "function_start":
		return 1
	default:
		return 1
	}
}

// FallbackAnalyzeImports: no-op for now
func (h *RubyHandler) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// FallbackAnalyzeEntryPoints: no-op; treat entire file as main
func (h *RubyHandler) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {}

func (h *RubyHandler) detectExistingOTELSetup(node *sitter.Node, content []byte) bool {
	body := node.Content(content)
	return strings.Contains(body, "OpenTelemetry") || strings.Contains(body, "opentelemetry")
}

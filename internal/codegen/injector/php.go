package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// PHPHandler implements LanguageInjector for PHP
type PHPHandler struct {
	config *types.LanguageConfig
}

// NewPHPHandler creates a new PHP handler
func NewPHPHandler() *PHPHandler {
	return &PHPHandler{
		config: &types.LanguageConfig{
			Language:       "PHP",
			FileExtensions: []string{".php"},
			ImportQueries:  map[string]string{
				// PHP uses require/include; capture none for now and rely on fallback
			},
			FunctionQueries: map[string]string{
				// Heuristic: top-level program node as entry
				"main_function": `
                (program) @main_block
            `,
			},
			InsertionQueries:       map[string]string{},
			ImportTemplate:         `require_once __DIR__ . '/otel.php';`,
			InitializationTemplate: `setup_otel();`,
			CleanupTemplate:        ``,
		},
	}
}

func (h *PHPHandler) GetLanguage() *sitter.Language    { return php.GetLanguage() }
func (h *PHPHandler) GetConfig() *types.LanguageConfig { return h.config }

func (h *PHPHandler) GetRequiredImports() []string { return []string{"./otel.php"} }

func (h *PHPHandler) FormatImports(imports []string, hasExisting bool) string {
	if len(imports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, imp := range imports {
		b.WriteString(fmt.Sprintf("require_once '%s';\n", imp))
	}
	return b.String()
}

func (h *PHPHandler) FormatSingleImport(importPath string) string {
	return fmt.Sprintf("require_once '%s';\n", importPath)
}

func (h *PHPHandler) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	// No-op: we rely on fallback scanning for now
}

func (h *PHPHandler) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_block":
		// Use beginning of program as insertion point
		insertion := types.InsertionPoint{LineNumber: node.StartPoint().Row + 1, Column: node.StartPoint().Column + 1, Priority: 1}
		entry := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertion,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: strings.Contains(strings.ToLower(node.Content(content)), "opentelemetry"),
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entry)
	}
}

func (h *PHPHandler) GetInsertionPointPriority(captureName string) int { return 1 }

// FallbackAnalyzeImports: try to place require after opening tag if no locations found
func (h *PHPHandler) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {
	text := string(content)
	lines := strings.Split(text, "\n")
	line := 1
	for i, l := range lines {
		if strings.Contains(l, "<?php") {
			line = i + 2
			break
		}
	}
	analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{LineNumber: uint32(line), Column: 1, Priority: 1})
}

// FallbackAnalyzeEntryPoints: if none detected, treat file start as entry
func (h *PHPHandler) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {
	if len(analysis.EntryPoints) > 0 {
		return
	}
	analysis.EntryPoints = append(analysis.EntryPoints, types.EntryPointInfo{
		Name:       "main",
		LineNumber: 1,
		Column:     1,
		BodyStart:  types.InsertionPoint{LineNumber: 1, Column: 1, Priority: 1},
		BodyEnd:    types.InsertionPoint{LineNumber: 1, Column: 1, Priority: 1},
	})
}

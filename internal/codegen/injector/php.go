package injector

import (
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/php"
)

// PHPInjector implements LanguageInjector for PHP
type PHPInjector struct {
	config *types.LanguageConfig
}

// NewPHPInjector creates a new PHP handler
func NewPHPInjector() *PHPInjector {
	return &PHPInjector{
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
			ImportTemplate:         `require_once './otel.php';`,
			InitializationTemplate: `setup_otel();`,
			CleanupTemplate:        ``,
			InitAtTop:              false,
		},
	}
}

func (h *PHPInjector) GetLanguage() *sitter.Language    { return php.GetLanguage() }
func (h *PHPInjector) GetConfig() *types.LanguageConfig { return h.config }

func (h *PHPInjector) GetRequiredImports() []string { return []string{"./otel.php"} }

func (h *PHPInjector) FormatImports(imports []string, hasExisting bool) string {
	if len(imports) == 0 {
		return ""
	}
	var b strings.Builder
	for _, imp := range imports {
		if imp == "./otel.php" || strings.HasSuffix(imp, "/otel.php") {
			b.WriteString("require_once './otel.php';\n")
		} else {
			b.WriteString(fmt.Sprintf("require_once '%s';\n", imp))
		}
	}
	return b.String()
}

func (h *PHPInjector) FormatSingleImport(importPath string) string {
	if importPath == "./otel.php" || strings.HasSuffix(importPath, "/otel.php") {
		return "require_once './otel.php';\n"
	}
	return fmt.Sprintf("require_once '%s';\n", importPath)
}

func (h *PHPInjector) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	// No-op: we rely on fallback scanning for now
}

func (h *PHPInjector) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_block":
		// Insert after opening tag and any declare(strict_types=1) statements
		insertionLine := determinePHPTopInsertionLine(string(content))
		insertion := types.InsertionPoint{LineNumber: uint32(insertionLine), Column: 1, Priority: 2}
		entry := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   uint32(insertionLine),
			Column:       1,
			BodyStart:    insertion,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: strings.Contains(strings.ToLower(node.Content(content)), "opentelemetry"),
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entry)
	}
}

func (h *PHPInjector) GetInsertionPointPriority(captureName string) int { return 1 }

// FallbackAnalyzeImports: try to place require after opening tag if no locations found
func (h *PHPInjector) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {
	line := determinePHPTopInsertionLine(string(content))
	analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{LineNumber: uint32(line), Column: 1, Priority: 2})
}

// FallbackAnalyzeEntryPoints: if none detected, treat file start as entry
func (h *PHPInjector) FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis) {
	if len(analysis.EntryPoints) > 0 {
		return
	}
	insertionLine := determinePHPTopInsertionLine(string(content))
	analysis.EntryPoints = append(analysis.EntryPoints, types.EntryPointInfo{
		Name:       "main",
		LineNumber: uint32(insertionLine),
		Column:     1,
		BodyStart:  types.InsertionPoint{LineNumber: uint32(insertionLine), Column: 1, Priority: 2},
		BodyEnd:    types.InsertionPoint{LineNumber: 1, Column: 1, Priority: 1},
	})
}

// determinePHPTopInsertionLine returns the line number right after opening tag and any declare(strict_types)
// to safely insert requires and initialization without violating PHP strict_types placement rules.
func determinePHPTopInsertionLine(text string) int {
	lines := strings.Split(text, "\n")
	line := 1
	// Find opening tag
	start := 0
	for i, l := range lines {
		if strings.Contains(l, "<?php") {
			start = i + 1
			line = i + 2
			break
		}
	}
	// Advance past whitespace/comments and declare(strict_types=...);
	for j := start; j < len(lines); j++ {
		tl := strings.TrimSpace(lines[j])
		if tl == "" || strings.HasPrefix(tl, "//") || strings.HasPrefix(tl, "#") || strings.HasPrefix(tl, "/*") {
			line = j + 2
			continue
		}
		if strings.HasPrefix(tl, "declare(") && strings.Contains(tl, "strict_types") {
			line = j + 2
			continue
		}
		break
	}
	if line < 1 {
		line = 1
	}
	return line
}

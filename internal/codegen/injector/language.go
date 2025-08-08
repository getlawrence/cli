package injector

import (
	"github.com/getlawrence/cli/internal/codegen/types"
	sitter "github.com/smacker/go-tree-sitter"
)

// LanguageHandler defines the interface for language-specific code injection operations
type LanguageInjector interface {
	// GetLanguage returns the tree-sitter language parser
	GetLanguage() *sitter.Language

	// GetConfig returns the language configuration
	GetConfig() *types.LanguageConfig

	// GetRequiredImports returns the list of imports needed for OTEL
	GetRequiredImports() []string

	// FormatImports formats import statements for this language
	FormatImports(imports []string, hasExistingImports bool) string

	// FormatSingleImport formats a single import statement
	FormatSingleImport(importPath string) string

	// AnalyzeImportCapture processes an import capture from tree-sitter query
	AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis)

	// AnalyzeFunctionCapture processes a function capture from tree-sitter query
	AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig)

	// GetInsertionPointPriority returns priority for insertion point types
	GetInsertionPointPriority(captureName string) int

	// FallbackAnalyzeImports allows language-specific analysis if tree-sitter didn't find enough info
	FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis)
}

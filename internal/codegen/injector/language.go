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

	// GetFrameworkImports returns framework-specific imports based on detected frameworks
	GetFrameworkImports(content []byte) []string

	// FormatImports formats import statements for this language
	FormatImports(imports []string, hasExistingImports bool) string

	// FormatSingleImport formats a single import statement
	FormatSingleImport(importPath string) string

	// FormatFrameworkImports formats framework-specific import statements
	FormatFrameworkImports(imports []string) string

	// AnalyzeImportCapture processes an import capture from tree-sitter query
	AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis)

	// AnalyzeFunctionCapture processes a function capture from tree-sitter query
	AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig)

	// GetInsertionPointPriority returns priority for insertion point types
	GetInsertionPointPriority(captureName string) int

	// GenerateFrameworkModifications generates framework-specific instrumentation modifications
	GenerateFrameworkModifications(content []byte, operationsData *types.OperationsData) []types.CodeModification

	// FallbackAnalyzeImports allows language-specific analysis if tree-sitter didn't find enough info
	FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis)

	// FallbackAnalyzeEntryPoints allows language-specific entrypoint discovery if tree-sitter didn't find any
	FallbackAnalyzeEntryPoints(content []byte, analysis *types.FileAnalysis)

	// GenerateImportModifications generates modifications to fix import statements
	GenerateImportModifications(content []byte, analysis *types.FileAnalysis) []types.CodeModification
}

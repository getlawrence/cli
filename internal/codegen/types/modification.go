package types

// ModificationType represents the type of code modification
type ModificationType string

const (
	ModificationAddImport     ModificationType = "add_import"
	ModificationAddInit       ModificationType = "add_initialization"
	ModificationAddCleanup    ModificationType = "add_cleanup"
	ModificationWrapFunction  ModificationType = "wrap_function"
	ModificationAddMiddleware ModificationType = "add_middleware"
)

// CodeModification represents a modification to be applied to source code
type CodeModification struct {
	Type         ModificationType `json:"type"`
	Language     string           `json:"language"`
	FilePath     string           `json:"file_path"`
	LineNumber   uint32           `json:"line_number"`
	Column       uint32           `json:"column"`
	InsertBefore bool             `json:"insert_before"`
	InsertAfter  bool             `json:"insert_after"`
	Content      string           `json:"content"`
	Context      string           `json:"context"` // Surrounding code context for validation
}

// LanguageConfig defines how to modify code for a specific language
type LanguageConfig struct {
	Language               string            `json:"language"`
	FileExtensions         []string          `json:"file_extensions"`
	ImportQueries          map[string]string `json:"import_queries"`          // Query name -> Tree-sitter query
	FunctionQueries        map[string]string `json:"function_queries"`        // Query name -> Tree-sitter query
	InsertionQueries       map[string]string `json:"insertion_queries"`       // Query name -> Tree-sitter query
	CodeTemplates          map[string]string `json:"code_templates"`          // Template name -> code template
	ImportTemplate         string            `json:"import_template"`         // How to format imports
	InitializationTemplate string            `json:"initialization_template"` // How to format OTEL initialization
	CleanupTemplate        string            `json:"cleanup_template"`        // How to format cleanup code
	// InitAtTop indicates the initialization snippet must be placed at the very top of the file
	// before any other imports/requires. Useful for languages/runtimes that require early bootstrap.
	InitAtTop bool `json:"init_at_top,omitempty"`
}

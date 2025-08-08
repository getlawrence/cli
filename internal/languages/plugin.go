package languages

import (
	"github.com/getlawrence/cli/internal/codegen/dependency"
	"github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
)

// LanguagePlugin defines all optional capabilities a language can provide.
// Implementors can return nil for unsupported capabilities.
type LanguagePlugin interface {
	// Core identifiers
	ID() string          // canonical id used in keys and template names (e.g., "go")
	DisplayName() string // human-readable and tree-sitter name (e.g., "Go")

	// Entrypoint discovery (tree-sitter)
	EntryPointTreeSitterLanguage() *sitter.Language // may be nil
	EntrypointQuery() string                        // may be empty
	FileExtensions() []string                       // file extensions for entrypoint scan

	// Source-code injection
	Injector() injector.LanguageInjector // may be nil

	// Dependency management (adding packages)
	Dependencies() dependency.DependencyHandler // may be nil

	// Template-based code generation support
	SupportedMethods() []templates.InstallationMethod // empty if not supported
	OutputFilename(method templates.InstallationMethod) string
}

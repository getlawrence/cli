package template

// JavaScriptCodeGenerator handles JS-specific code generation
type JavaScriptCodeGenerator struct{}

// NewJavaScriptCodeGenerator creates a new JS code generator
func NewJavaScriptCodeGenerator() *JavaScriptCodeGenerator { return &JavaScriptCodeGenerator{} }

// GetOutputFilename suggests output filename
func (g *JavaScriptCodeGenerator) GetOutputFilename() string {
	return "otel.js"
}

// GetLanguageName returns the language name key for templates
func (g *JavaScriptCodeGenerator) GetLanguageName() string { return "javascript" }

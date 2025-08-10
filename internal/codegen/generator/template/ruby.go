package template

// RubyCodeGenerator handles Ruby-specific code generation
type RubyCodeGenerator struct{}

// NewRubyCodeGenerator creates a new Ruby code generator
func NewRubyCodeGenerator() *RubyCodeGenerator { return &RubyCodeGenerator{} }

// GetOutputFilename returns the appropriate output filename for the given method
func (g *RubyCodeGenerator) GetOutputFilename() string {
	return "otel.rb"
}

// GetLanguageName returns the language name
func (g *RubyCodeGenerator) GetLanguageName() string { return "ruby" }

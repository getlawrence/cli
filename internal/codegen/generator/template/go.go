package template

// GoCodeGenerator handles Go-specific code generation
type GoCodeGenerator struct{}

// NewGoCodeGenerator creates a new Go code generator
func NewGoCodeGenerator() *GoCodeGenerator {
	return &GoCodeGenerator{}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *GoCodeGenerator) GetOutputFilename() string {
	return "otel.go"
}

// GetLanguageName returns the language name
func (g *GoCodeGenerator) GetLanguageName() string {
	return "go"
}

package template

// JavaCodeGenerator handles Java-specific code generation
type JavaCodeGenerator struct{}

// NewJavaCodeGenerator creates a new Java code generator
func NewJavaCodeGenerator() *JavaCodeGenerator { return &JavaCodeGenerator{} }

// GetOutputFilename suggests output filename
func (g *JavaCodeGenerator) GetOutputFilename() string {
	return "OtelInit.java"
}

// GetLanguageName returns the language name key for templates
func (g *JavaCodeGenerator) GetLanguageName() string { return "java" }

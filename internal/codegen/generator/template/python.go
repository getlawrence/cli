package template

// PythonCodeGenerator handles Python-specific code generation
type PythonCodeGenerator struct{}

// NewPythonCodeGenerator creates a new Python code generator
func NewPythonCodeGenerator() *PythonCodeGenerator {
	return &PythonCodeGenerator{}
}

// GetOutputFilename returns the appropriate output filename for the given method
func (g *PythonCodeGenerator) GetOutputFilename() string {
	return "otel.py"
}

// GetLanguageName returns the language name
func (g *PythonCodeGenerator) GetLanguageName() string {
	return "python"
}

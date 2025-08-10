package template

// PHPCodeGenerator handles PHP-specific code generation
type PHPCodeGenerator struct{}

// NewPHPCodeGenerator creates a new PHP code generator
func NewPHPCodeGenerator() *PHPCodeGenerator { return &PHPCodeGenerator{} }

// GetOutputFilename returns the appropriate output filename for the given method
func (g *PHPCodeGenerator) GetOutputFilename() string {
	return "otel.php"
}

// GetLanguageName returns the language name
func (g *PHPCodeGenerator) GetLanguageName() string { return "php" }

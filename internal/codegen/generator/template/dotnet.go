package template

// DotNetCodeGenerator handles .NET-specific code generation
type DotNetCodeGenerator struct{}

func NewDotNetCodeGenerator() *DotNetCodeGenerator { return &DotNetCodeGenerator{} }

func (g *DotNetCodeGenerator) GetOutputFilename() string {
	return "Otel.cs"
}

func (g *DotNetCodeGenerator) GetLanguageName() string { return "dotnet" }

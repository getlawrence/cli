package codegen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/templates"
)

const (
	dryRunOutputFormat  = "Would write to: %s\n"
	dryRunContentFormat = "Content:\n%s\n"
	defaultServiceName  = "my-service"
)

// TemplateGenerationStrategy implements direct code generation using templates
type TemplateGenerationStrategy struct {
	templateMgr *templates.Manager
}

// NewTemplateGenerationStrategy creates a new template-based generation strategy
func NewTemplateGenerationStrategy(templateMgr *templates.Manager) *TemplateGenerationStrategy {
	return &TemplateGenerationStrategy{
		templateMgr: templateMgr,
	}
}

// GetName returns the name of this strategy
func (s *TemplateGenerationStrategy) GetName() string {
	return "Template-based"
}

// IsAvailable checks if template generation is available (always true)
func (s *TemplateGenerationStrategy) IsAvailable() bool {
	return true
}

// GetRequiredFlags returns flags required for template generation
func (s *TemplateGenerationStrategy) GetRequiredFlags() []string {
	return []string{} // No required flags
}

// GenerateCode generates code directly using templates
func (s *TemplateGenerationStrategy) GenerateCode(ctx context.Context, opportunities []Opportunity, req GenerationRequest) error {
	if len(opportunities) == 0 {
		fmt.Println("No code generation opportunities found")
		return nil
	}

	// Group opportunities by language
	languageOpportunities := s.groupOpportunitiesByLanguage(opportunities)

	// Generate code for each language
	var generatedFiles []string
	for language, langOpportunities := range languageOpportunities {
		files, err := s.generateCodeForLanguage(language, langOpportunities, req)
		if err != nil {
			fmt.Printf("Warning: failed to generate code for %s: %v\n", language, err)
			continue
		}
		generatedFiles = append(generatedFiles, files...)
	}

	if len(generatedFiles) == 0 {
		fmt.Println("No code files were generated")
		return nil
	}

	// Report results
	fmt.Printf("Successfully generated %d instrumentation files:\n", len(generatedFiles))
	for _, file := range generatedFiles {
		fmt.Printf("  - %s\n", file)
	}

	return nil
}

func (s *TemplateGenerationStrategy) generateCodeForLanguage(language string, opportunities []Opportunity, req GenerationRequest) ([]string, error) {
	// Collect all instrumentations for this language
	allInstrumentations := s.collectAllInstrumentations(opportunities)

	if len(allInstrumentations) == 0 {
		return nil, fmt.Errorf("no instrumentations found for %s", language)
	}

	// Generate code based on the method and language
	switch strings.ToLower(language) {
	case "go":
		return s.generateGoCode(allInstrumentations, req)
	case "python":
		return s.generatePythonCode(allInstrumentations, req)
	default:
		return nil, fmt.Errorf("template-based code generation not supported for language: %s", language)
	}
}

func (s *TemplateGenerationStrategy) generateGoCode(instrumentations []string, req GenerationRequest) ([]string, error) {
	// Generate based on method
	switch req.Method {
	case templates.CodeInstrumentation:
		return s.generateGoCodeInstrumentation(instrumentations, req)
	case templates.AutoInstrumentation:
		return s.generateGoAutoInstrumentation(instrumentations, req)
	default:
		return nil, fmt.Errorf("unsupported method %s for Go", req.Method)
	}
}

func (s *TemplateGenerationStrategy) generateGoCodeInstrumentation(instrumentations []string, req GenerationRequest) ([]string, error) {
	serviceName := filepath.Base(req.CodebasePath)
	if serviceName == "." {
		// Get current directory name when path is "."
		if cwd, err := os.Getwd(); err == nil {
			serviceName = filepath.Base(cwd)
		} else {
			serviceName = defaultServiceName
		}
	}

	// Create template data
	data := templates.TemplateData{
		Language:         "go",
		Method:           req.Method,
		Instrumentations: instrumentations,
		ServiceName:      serviceName,
	}

	// Generate code using template
	code, err := s.templateMgr.GenerateInstructions("go", req.Method, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Go code: %w", err)
	}

	// Determine output directory
	outputDir := req.CodebasePath
	if req.Config.OutputDirectory != "" {
		outputDir = req.Config.OutputDirectory
	}

	// Write to file
	filename := "otel_instrumentation.go"
	outputPath := filepath.Join(outputDir, filename)

	if req.Config.DryRun {
		fmt.Printf("Generated Go instrumentation code (dry run):\n")
		fmt.Printf(dryRunOutputFormat, outputPath)
		fmt.Printf(dryRunContentFormat, code)
		return []string{outputPath}, nil
	}

	if err := s.writeCodeToFile(outputPath, code); err != nil {
		return nil, fmt.Errorf("failed to write Go code to %s: %w", outputPath, err)
	}

	return []string{outputPath}, nil
}

func (s *TemplateGenerationStrategy) generateGoAutoInstrumentation(instrumentations []string, req GenerationRequest) ([]string, error) {
	serviceName := filepath.Base(req.CodebasePath)
	if serviceName == "." {
		// Get current directory name when path is "."
		if cwd, err := os.Getwd(); err == nil {
			serviceName = filepath.Base(cwd)
		} else {
			serviceName = defaultServiceName
		}
	}

	// Create template data
	data := templates.TemplateData{
		Language:         "go",
		Method:           req.Method,
		Instrumentations: instrumentations,
		ServiceName:      serviceName,
	}

	// Generate code using auto template
	code, err := s.templateMgr.GenerateInstructions("go", req.Method, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Go auto instrumentation: %w", err)
	}

	// Determine output directory
	outputDir := req.CodebasePath
	if req.Config.OutputDirectory != "" {
		outputDir = req.Config.OutputDirectory
	}

	// Write to file
	filename := "otel_auto.go"
	outputPath := filepath.Join(outputDir, filename)

	if req.Config.DryRun {
		fmt.Printf("Generated Go auto instrumentation code (dry run):\n")
		fmt.Printf(dryRunOutputFormat, outputPath)
		fmt.Printf(dryRunContentFormat, code)
		return []string{outputPath}, nil
	}

	if err := s.writeCodeToFile(outputPath, code); err != nil {
		return nil, fmt.Errorf("failed to write Go auto code to %s: %w", outputPath, err)
	}

	return []string{outputPath}, nil
}

func (s *TemplateGenerationStrategy) generatePythonCode(instrumentations []string, req GenerationRequest) ([]string, error) {
	serviceName := filepath.Base(req.CodebasePath)
	if serviceName == "." {
		// Get current directory name when path is "."
		if cwd, err := os.Getwd(); err == nil {
			serviceName = filepath.Base(cwd)
		} else {
			serviceName = defaultServiceName
		}
	}

	// Create template data
	data := templates.TemplateData{
		Language:         "python",
		Method:           req.Method,
		Instrumentations: instrumentations,
		ServiceName:      serviceName,
	}

	// Generate code using template
	code, err := s.templateMgr.GenerateInstructions("python", req.Method, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate Python code: %w", err)
	}

	// Determine output directory
	outputDir := req.CodebasePath
	if req.Config.OutputDirectory != "" {
		outputDir = req.Config.OutputDirectory
	}

	// Write to file
	filename := "otel_instrumentation.py"
	outputPath := filepath.Join(outputDir, filename)

	if req.Config.DryRun {
		fmt.Printf("Generated Python instrumentation code (dry run):\n")
		fmt.Printf(dryRunOutputFormat, outputPath)
		fmt.Printf(dryRunContentFormat, code)
		return []string{outputPath}, nil
	}

	if err := s.writeCodeToFile(outputPath, code); err != nil {
		return nil, fmt.Errorf("failed to write Python code to %s: %w", outputPath, err)
	}

	return []string{outputPath}, nil
}

func (s *TemplateGenerationStrategy) writeCodeToFile(filePath, content string) error {
	// Ensure directory exists
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Write file
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// groupOpportunitiesByLanguage groups opportunities by programming language
func (s *TemplateGenerationStrategy) groupOpportunitiesByLanguage(opportunities []Opportunity) map[string][]Opportunity {
	grouped := make(map[string][]Opportunity)

	for _, opp := range opportunities {
		if opp.Language != "" {
			grouped[opp.Language] = append(grouped[opp.Language], opp)
		}
	}

	return grouped
}

// collectAllInstrumentations extracts unique instrumentations from all opportunities
func (s *TemplateGenerationStrategy) collectAllInstrumentations(opportunities []Opportunity) []string {
	seen := make(map[string]bool)
	var instrumentations []string

	for _, opp := range opportunities {
		for _, instr := range opp.Instrumentations {
			if !seen[instr] {
				seen[instr] = true
				instrumentations = append(instrumentations, instr)
			}
		}
	}

	return instrumentations
}

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

// OperationsData contains the analysis of opportunities organized by operation type
type OperationsData struct {
	InstallOTEL             bool                // Whether OTEL needs to be installed
	InstallInstrumentations []string            // Instrumentations to install
	InstallComponents       map[string][]string // Components to install by type (sdk, propagator, exporter)
	RemoveComponents        map[string][]string // Components to remove by type
}

// isEmpty checks if there are any operations to perform
func (od *OperationsData) isEmpty() bool {
	return !od.InstallOTEL &&
		len(od.InstallInstrumentations) == 0 &&
		len(od.InstallComponents) == 0 &&
		len(od.RemoveComponents) == 0
}

// TemplateGenerationStrategy implements direct code generation using templates
type TemplateGenerationStrategy struct {
	templateEngine *templates.TemplateEngine
}

// NewTemplateGenerationStrategy creates a new template-based generation strategy
func NewTemplateGenerationStrategy(templateEngine *templates.TemplateEngine) *TemplateGenerationStrategy {
	return &TemplateGenerationStrategy{
		templateEngine: templateEngine,
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
	var operationsSummary []string

	for language, langOpportunities := range languageOpportunities {
		// Analyze what operations will be performed
		operationsData := s.analyzeOpportunities(langOpportunities)
		operationsSummary = append(operationsSummary, s.createOperationsSummary(language, operationsData)...)

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
	fmt.Printf("Successfully generated %d files:\n", len(generatedFiles))
	for _, file := range generatedFiles {
		fmt.Printf("  - %s\n", file)
	}

	if len(operationsSummary) > 0 {
		fmt.Println("\nOperations performed:")
		for _, summary := range operationsSummary {
			fmt.Printf("  %s\n", summary)
		}
	}

	return nil
}

func (s *TemplateGenerationStrategy) generateCodeForLanguage(language string, opportunities []Opportunity, req GenerationRequest) ([]string, error) {
	// Analyze opportunities to determine what operations to perform
	operationsData := s.analyzeOpportunities(opportunities)

	if operationsData.isEmpty() {
		return nil, fmt.Errorf("no operations to perform for %s", language)
	}

	// Generate code based on the method and language
	switch strings.ToLower(language) {
	case "go":
		return s.generateGoCode(operationsData, req)
	case "python":
		return s.generatePythonCode(operationsData, req)
	default:
		return nil, fmt.Errorf("template-based code generation not supported for language: %s", language)
	}
}

func (s *TemplateGenerationStrategy) generateGoCode(operationsData *OperationsData, req GenerationRequest) ([]string, error) {
	// Generate based on method
	switch req.Method {
	case templates.CodeInstrumentation:
		return s.generateGoCodeInstrumentation(operationsData, req)
	case templates.AutoInstrumentation:
		return s.generateGoAutoInstrumentation(operationsData, req)
	default:
		return nil, fmt.Errorf("unsupported method %s for Go", req.Method)
	}
}

func (s *TemplateGenerationStrategy) generateGoCodeInstrumentation(operationsData *OperationsData, req GenerationRequest) ([]string, error) {
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
		Language:          "go",
		Method:            req.Method,
		Instrumentations:  operationsData.InstallInstrumentations,
		ServiceName:       serviceName,
		InstallOTEL:       operationsData.InstallOTEL,
		InstallComponents: operationsData.InstallComponents,
		RemoveComponents:  operationsData.RemoveComponents,
	}

	// Generate code using template
	code, err := s.templateEngine.GenerateInstructions("go", req.Method, data)
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

func (s *TemplateGenerationStrategy) generateGoAutoInstrumentation(operationsData *OperationsData, req GenerationRequest) ([]string, error) {
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
		Language:          "go",
		Method:            req.Method,
		Instrumentations:  operationsData.InstallInstrumentations,
		ServiceName:       serviceName,
		InstallOTEL:       operationsData.InstallOTEL,
		InstallComponents: operationsData.InstallComponents,
		RemoveComponents:  operationsData.RemoveComponents,
	}

	// Generate code using auto template
	code, err := s.templateEngine.GenerateInstructions("go", req.Method, data)
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

func (s *TemplateGenerationStrategy) generatePythonCode(operationsData *OperationsData, req GenerationRequest) ([]string, error) {
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
		Language:          "python",
		Method:            req.Method,
		Instrumentations:  operationsData.InstallInstrumentations,
		ServiceName:       serviceName,
		InstallOTEL:       operationsData.InstallOTEL,
		InstallComponents: operationsData.InstallComponents,
		RemoveComponents:  operationsData.RemoveComponents,
	}

	// Generate code using template
	code, err := s.templateEngine.GenerateInstructions("python", req.Method, data)
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

// analyzeOpportunities processes opportunities and organizes them by operation type
func (s *TemplateGenerationStrategy) analyzeOpportunities(opportunities []Opportunity) *OperationsData {
	data := &OperationsData{
		InstallComponents: make(map[string][]string),
		RemoveComponents:  make(map[string][]string),
	}

	for _, opp := range opportunities {
		switch opp.Type {
		case OpportunityInstallOTEL:
			data.InstallOTEL = true

		case OpportunityInstallComponent:
			if opp.ComponentType == ComponentTypeInstrumentation {
				data.InstallInstrumentations = append(data.InstallInstrumentations, opp.Component)
			} else {
				componentType := string(opp.ComponentType)
				data.InstallComponents[componentType] = append(data.InstallComponents[componentType], opp.Component)
			}

		case OpportunityRemoveComponent:
			componentType := string(opp.ComponentType)
			data.RemoveComponents[componentType] = append(data.RemoveComponents[componentType], opp.Component)
		}
	}

	return data
}

// createOperationsSummary generates a human-readable summary of operations for a language
func (s *TemplateGenerationStrategy) createOperationsSummary(language string, data *OperationsData) []string {
	var summary []string

	if data.InstallOTEL {
		summary = append(summary, fmt.Sprintf("[%s] Install OpenTelemetry SDK", language))
	}

	if len(data.InstallInstrumentations) > 0 {
		summary = append(summary, fmt.Sprintf("[%s] Install instrumentations: %s", language, strings.Join(data.InstallInstrumentations, ", ")))
	}

	for componentType, components := range data.InstallComponents {
		if len(components) > 0 {
			summary = append(summary, fmt.Sprintf("[%s] Install %s components: %s", language, componentType, strings.Join(components, ", ")))
		}
	}

	for componentType, components := range data.RemoveComponents {
		if len(components) > 0 {
			summary = append(summary, fmt.Sprintf("[%s] Remove %s components: %s", language, componentType, strings.Join(components, ", ")))
		}
	}

	return summary
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

package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/templates"
)

const (
	dryRunOutputFormat  = "Would write to: %s\n"
	dryRunContentFormat = "Content:\n%s\n"
	defaultServiceName  = "my-service"
)

// TemplateGenerationStrategy implements direct code generation using templates
type TemplateGenerationStrategy struct {
	templateEngine   *templates.TemplateEngine
	codeInjector     *injector.CodeInjector
	languageRegistry *LanguageGeneratorRegistry
}

// NewTemplateGenerationStrategy creates a new template-based generation strategy
func NewTemplateGenerationStrategy(templateEngine *templates.TemplateEngine) *TemplateGenerationStrategy {
	registry := NewLanguageGeneratorRegistry()

	registry.RegisterLanguage("python", NewPythonCodeGenerator())
	registry.RegisterLanguage("go", NewGoCodeGenerator())

	return &TemplateGenerationStrategy{
		templateEngine:   templateEngine,
		codeInjector:     injector.NewCodeInjector(),
		languageRegistry: registry,
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

// GetSupportedLanguages returns all supported languages for template generation
func (s *TemplateGenerationStrategy) GetSupportedLanguages() []string {
	return s.languageRegistry.GetSupportedLanguages()
}

// GenerateCode generates code directly using templates
func (s *TemplateGenerationStrategy) GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	if len(opportunities) == 0 {
		fmt.Println("No code generation opportunities found")
		return nil
	}

	directoryOpportunities := s.groupOpportunitiesByDirectory(opportunities)
	if len(directoryOpportunities) == 0 {
		fmt.Println("No opportunities to process")
		return nil
	}

	var generatedFiles []string
	var operationsSummary []string

	for directory, opps := range directoryOpportunities {
		languageOpportunities := s.groupOpportunitiesByLanguage(opps)

		for language, langOpportunities := range languageOpportunities {
			operationsData := s.analyzeOpportunities(langOpportunities)
			operationsSummary = append(operationsSummary, s.createOperationsSummary(language, operationsData)...)

			// Handle entry point modifications
			entryPointFiles, err := s.handleEntryPointModifications(langOpportunities, req, operationsData)
			if err != nil {
				fmt.Printf("Warning: failed to modify entry points for %s: %v\n", language, err)
			} else {
				generatedFiles = append(generatedFiles, entryPointFiles...)
			}

			// Handle regular file generation
			files, err := s.generateCodeForLanguage(language, langOpportunities, req, directory)
			if err != nil {
				fmt.Printf("Warning: failed to generate code for %s: %v\n", language, err)
				continue
			}
			generatedFiles = append(generatedFiles, files...)
		}
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

func (s *TemplateGenerationStrategy) generateCodeForLanguage(language string, opportunities []domain.Opportunity, req types.GenerationRequest, directory string) ([]string, error) {
	// Get language generator
	languageGen, exists := s.languageRegistry.GetGenerator(strings.ToLower(language))
	if !exists {
		return nil, fmt.Errorf("template-based code generation not supported for language: %s", language)
	}

	// Convert string method to InstallationMethod
	method := templates.InstallationMethod(req.Method)

	// Validate method is supported for this language
	if err := languageGen.ValidateMethod(method); err != nil {
		return nil, err
	}

	operationsData := s.analyzeOpportunities(opportunities)
	return s.generateCodeWithLanguageGenerator(languageGen, method, operationsData, req, directory)
}

// generateCodeWithLanguageGenerator is a generic method for generating code using any language generator
func (s *TemplateGenerationStrategy) generateCodeWithLanguageGenerator(
	languageGen LanguageCodeGenerator,
	method templates.InstallationMethod,
	operationsData *types.OperationsData,
	req types.GenerationRequest,
	directory string,
) ([]string, error) {
	serviceName := s.determineServiceName(req.CodebasePath)

	// Create template data
	data := templates.TemplateData{
		Language:          languageGen.GetLanguageName(),
		Method:            method,
		Instrumentations:  operationsData.InstallInstrumentations,
		ServiceName:       serviceName,
		InstallOTEL:       operationsData.InstallOTEL,
		InstallComponents: operationsData.InstallComponents,
		RemoveComponents:  operationsData.RemoveComponents,
	}

	// Generate code using template
	code, err := s.templateEngine.GenerateInstructions(languageGen.GetLanguageName(), method, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s code: %w", languageGen.GetLanguageName(), err)
	}

	// Determine output directory and filename
	outputDir := s.determineOutputDirectory(req, directory)
	filename := languageGen.GetOutputFilename(method)
	outputPath := filepath.Join(outputDir, filename)

	if req.Config.DryRun {
		fmt.Printf("Generated %s instrumentation code (dry run):\n", languageGen.GetLanguageName())
		fmt.Printf(dryRunOutputFormat, outputPath)
		fmt.Printf(dryRunContentFormat, code)
		return []string{outputPath}, nil
	}

	if err := s.writeCodeToFile(outputPath, code); err != nil {
		return nil, fmt.Errorf("failed to write %s code to %s: %w", languageGen.GetLanguageName(), outputPath, err)
	}

	return []string{outputPath}, nil
}

// determineServiceName extracts service name from codebase path
func (s *TemplateGenerationStrategy) determineServiceName(codebasePath string) string {
	serviceName := filepath.Base(codebasePath)
	if serviceName == "." {
		// Get current directory name when path is "."
		if cwd, err := os.Getwd(); err == nil {
			serviceName = filepath.Base(cwd)
		} else {
			serviceName = defaultServiceName
		}
	}
	return serviceName
}

// determineOutputDirectory determines the output directory for generated files
func (s *TemplateGenerationStrategy) determineOutputDirectory(req types.GenerationRequest, directory string) string {
	outputDir := req.CodebasePath
	if req.Config.OutputDirectory != "" {
		outputDir = req.Config.OutputDirectory
	}

	// Handle the "root" directory case - don't append it as a subdirectory
	if directory == "root" {
		return outputDir
	}

	return filepath.Join(outputDir, directory)
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
func (s *TemplateGenerationStrategy) analyzeOpportunities(opportunities []domain.Opportunity) *types.OperationsData {
	data := &types.OperationsData{
		InstallComponents: make(map[string][]string),
		RemoveComponents:  make(map[string][]string),
	}

	for _, opp := range opportunities {
		switch opp.Type {
		case domain.OpportunityInstallOTEL:
			data.InstallOTEL = true

		case domain.OpportunityInstallComponent:
			if opp.ComponentType == domain.ComponentTypeInstrumentation {
				data.InstallInstrumentations = append(data.InstallInstrumentations, opp.Component)
			} else {
				componentType := string(opp.ComponentType)
				data.InstallComponents[componentType] = append(data.InstallComponents[componentType], opp.Component)
			}

		case domain.OpportunityRemoveComponent:
			componentType := string(opp.ComponentType)
			data.RemoveComponents[componentType] = append(data.RemoveComponents[componentType], opp.Component)
		}
	}

	return data
}

// createOperationsSummary generates a human-readable summary of operations for a language
func (s *TemplateGenerationStrategy) createOperationsSummary(language string, data *types.OperationsData) []string {
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
func (s *TemplateGenerationStrategy) groupOpportunitiesByDirectory(opportunities []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)

	for _, opp := range opportunities {
		if opp.FilePath != "" {
			grouped[opp.FilePath] = append(grouped[opp.FilePath], opp)
		}
	}

	return grouped
}

// groupOpportunitiesByLanguage groups opportunities by programming language
func (s *TemplateGenerationStrategy) groupOpportunitiesByLanguage(opportunities []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)

	for _, opp := range opportunities {
		if opp.Language != "" {
			grouped[opp.Language] = append(grouped[opp.Language], opp)
		}
	}

	return grouped
}

// handleEntryPointModifications processes entry point modification opportunities
func (s *TemplateGenerationStrategy) handleEntryPointModifications(
	opportunities []domain.Opportunity,
	req types.GenerationRequest,
	operationsData *types.OperationsData,
) ([]string, error) {
	var modifiedFiles []string

	for _, opp := range opportunities {
		if opp.Type == domain.OpportunityInstallOTEL && opp.EntryPoint != nil {
			files, err := s.codeInjector.InjectOtelInitialization(
				context.Background(),
				opp.EntryPoint,
				operationsData,
				req,
			)
			if err != nil {
				return nil, fmt.Errorf("failed to modify entry point %s: %w", opp.EntryPoint.FilePath, err)
			}
			modifiedFiles = append(modifiedFiles, files...)
		}
	}

	return modifiedFiles, nil
}

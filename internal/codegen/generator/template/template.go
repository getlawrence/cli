package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency"
	"github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/internal/templates"
)

const (
	dryRunOutputFormat  = "Would write to: %s\n"
	dryRunContentFormat = "Content:\n%s\n"
	defaultServiceName  = "my-service"
)

// TemplateGenerationStrategy implements direct code generation using templates
type TemplateGenerationStrategy struct {
	logger           logger.Logger
	templateEngine   *templates.TemplateEngine
	codeInjector     *injector.CodeInjector
	dependencyWriter *dependency.DependencyWriter
}

// NewTemplateGenerationStrategy creates a new template-based generation strategy
func NewTemplateGenerationStrategy(templateEngine *templates.TemplateEngine, logger logger.Logger) *TemplateGenerationStrategy {
	return &TemplateGenerationStrategy{
		logger:           logger,
		templateEngine:   templateEngine,
		codeInjector:     injector.NewCodeInjector(logger),
		dependencyWriter: dependency.NewDependencyWriter(logger),
	}
}

// NOTE: Language-specific instrumentation prerequisite resolution is handled
// by language dependency handlers to keep this generator language-agnostic.

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
	return getSupportedLanguages()
}

// GenerateCode generates code directly using templates
func (s *TemplateGenerationStrategy) GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	if len(opportunities) == 0 {
		s.logger.Log("No code generation opportunities found")
		return nil
	}

	directoryOpportunities := s.groupOpportunitiesByDirectory(opportunities)
	if len(directoryOpportunities) == 0 {
		s.logger.Log("No opportunities to process")
		return nil
	}

	var generatedFiles []string
	var operationsSummary []string

	for directory, opps := range directoryOpportunities {
		languageOpportunities := s.groupOpportunitiesByLanguage(opps)

		for language, langOpportunities := range languageOpportunities {
			normalizedLanguage := normalizeLanguageForGeneration(language)
			operationsData := s.analyzeOpportunities(langOpportunities)
			operationsSummary = append(operationsSummary, s.createOperationsSummary(normalizedLanguage, operationsData)...)
			dependencyPath := s.determineOutputDirectory(req, directory)

			if err := s.addDependencies(ctx, dependencyPath, normalizedLanguage, operationsData, req); err != nil {
				s.logger.Logf("Warning: failed to add dependencies for %s: %v\n", normalizedLanguage, err)
				// If dependency installation fails, skip entry point modifications and file generation
				// to avoid producing code that won't compile/run (e.g., missing packages).
				continue
			}

			// Handle entry point modifications
			entryPointFiles, err := s.handleEntryPointModifications(langOpportunities, req, operationsData)
			if err != nil {
				s.logger.Logf("Warning: failed to modify entry points for %s: %v\n", language, err)
			} else {
				generatedFiles = append(generatedFiles, entryPointFiles...)
			}

			// Handle regular file generation
			files, err := s.generateCodeForLanguage(normalizedLanguage, langOpportunities, req, directory)
			if err != nil {
				s.logger.Logf("Warning: failed to generate code for %s: %v\n", normalizedLanguage, err)
				continue
			}
			generatedFiles = append(generatedFiles, files...)
		}
	}

	if len(generatedFiles) == 0 {
		s.logger.Log("No code files were generated")
		return nil
	}

	// Report results
	s.logger.Logf("Successfully generated %d files:\n", len(generatedFiles))
	for _, file := range generatedFiles {
		s.logger.Logf("  - %s\n", file)
	}

	if len(operationsSummary) > 0 {
		s.logger.Log("\nOperations performed:")
		for _, summary := range operationsSummary {
			s.logger.Logf("  %s\n", summary)
		}
	}

	return nil
}

func (s *TemplateGenerationStrategy) generateCodeForLanguage(language string, opportunities []domain.Opportunity, req types.GenerationRequest, directory string) ([]string, error) {
	operationsData := s.analyzeOpportunities(opportunities)
	normalized := strings.ToLower(language)
	if _, ok := supportedLanguageExtensions[normalized]; !ok {
		return nil, fmt.Errorf("template-based code generation not supported for language: %s", language)
	}
	return s.generateCodeWithLanguage(normalized, operationsData, req, directory)
}

// generateCodeWithLanguage renders the template and writes the language-specific output file.
func (s *TemplateGenerationStrategy) generateCodeWithLanguage(
	language string,
	operationsData *types.OperationsData,
	req types.GenerationRequest,
	directory string,
) ([]string, error) {
	serviceName := s.determineServiceName(req.CodebasePath)

	// Create template data
	data := templates.TemplateData{
		Language:          language,
		Instrumentations:  operationsData.InstallInstrumentations,
		ServiceName:       serviceName,
		InstallOTEL:       operationsData.InstallOTEL,
		InstallComponents: operationsData.InstallComponents,
		RemoveComponents:  operationsData.RemoveComponents,
	}

	// Apply advanced OTEL config to template data when provided
	if req.OTEL != nil {
		if req.OTEL.ServiceName != "" {
			data.ServiceName = req.OTEL.ServiceName
		}
		if len(req.OTEL.Propagators) > 0 {
			data.Propagators = append([]string{}, req.OTEL.Propagators...)
		}
		// Sampler config
		if sType := strings.ToLower(req.OTEL.Sampler.Type); sType != "" {
			data.SamplerType = sType
			if req.OTEL.Sampler.Ratio > 0 {
				data.SamplerRatio = req.OTEL.Sampler.Ratio
			}
		}
		if req.OTEL.Exporters.Traces.Type != "" {
			data.TraceExporterType = req.OTEL.Exporters.Traces.Type
			data.TraceProtocol = req.OTEL.Exporters.Traces.Protocol
			data.TraceEndpoint = req.OTEL.Exporters.Traces.Endpoint
			data.TraceHeaders = req.OTEL.Exporters.Traces.Headers
			data.TraceInsecure = req.OTEL.Exporters.Traces.Insecure
		}
	}

	// Generate code using template
	code, err := s.templateEngine.GenerateInstructions(language, data)
	if err != nil {
		return nil, fmt.Errorf("failed to generate %s code: %w", language, err)
	}

	// Determine output directory and filename
	outputDir := s.determineOutputDirectory(req, directory)
	filename := getOutputFilenameForLanguage(language)
	outputPath := filepath.Join(outputDir, filename)

	if req.Config.DryRun {
		s.logger.Logf("Generated %s instrumentation code (dry run):\n", language)
		s.logger.Logf(dryRunOutputFormat, outputPath)
		s.logger.Logf(dryRunContentFormat, code)
		return []string{outputPath}, nil
	}

	if err := s.writeCodeToFile(outputPath, code); err != nil {
		return nil, fmt.Errorf("failed to write %s code to %s: %w", language, outputPath, err)
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
			// Normalize language name to lowercase
			language := strings.ToLower(opp.Language)
			grouped[language] = append(grouped[language], opp)
		}
	}

	return grouped
}

// normalizeLanguageForGeneration maps language aliases to the canonical identifiers used by
// dependency management, templates, and injectors.
func normalizeLanguageForGeneration(language string) string {
	switch strings.ToLower(language) {
	case "js", "node", "nodejs":
		return "javascript"
	default:
		return strings.ToLower(language)
	}
}

// handleEntryPointModifications processes entry point modification opportunities
func (s *TemplateGenerationStrategy) handleEntryPointModifications(
	opportunities []domain.Opportunity,
	req types.GenerationRequest,
	operationsData *types.OperationsData,
) ([]string, error) {
	var modifiedFiles []string

	// Perform entry point modification if we plan to install OTEL or any instrumentations/components
	shouldModify := operationsData.InstallOTEL || len(operationsData.InstallInstrumentations) > 0 || len(operationsData.InstallComponents) > 0
	if !shouldModify {
		return modifiedFiles, nil
	}

	// Use any opportunity as a reference for language and directory
	for _, opp := range opportunities {
		// Skip entry point modifications for C#/.NET for now to avoid breaking
		// top-level Program.cs with premature references to builder/app.
		// Dependency updates and generated bootstrap files are still produced.
		if lang := strings.ToLower(opp.Language); lang == "csharp" || lang == "dotnet" {
			continue
		}
		entryPoint := &domain.EntryPoint{}
		dirPath := req.CodebasePath
		if opp.FilePath != "" && opp.FilePath != "root" {
			dirPath = filepath.Join(req.CodebasePath, opp.FilePath)
		}

		eps, err := s.codeInjector.DetectEntryPoints(dirPath, strings.ToLower(opp.Language))
		if err != nil || len(eps) == 0 {
			continue
		}

		best := eps[0]
		for _, ep := range eps {
			if ep.Confidence > best.Confidence {
				best = ep
			}
		}
		entryPoint = &best
		files, err := s.codeInjector.InjectOtelInitialization(
			context.Background(),
			entryPoint,
			operationsData,
			req,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to modify entry point %s: %w", entryPoint.FilePath, err)
		}
		modifiedFiles = append(modifiedFiles, files...)
		break
	}

	return modifiedFiles, nil
}

// addDependencies handles adding required dependencies to the project
func (s *TemplateGenerationStrategy) addDependencies(
	ctx context.Context,
	projectPath, language string,
	operationsData *types.OperationsData,
	req types.GenerationRequest,
) error {
	// Skip if no dependencies need to be added
	if !operationsData.InstallOTEL && len(operationsData.InstallInstrumentations) == 0 && len(operationsData.InstallComponents) == 0 {
		return nil
	}

	// In dry-run, don't touch the project filesystem. Just compute and display what would be added.
	if req.Config.DryRun {
		deps, err := s.dependencyWriter.GetRequiredDependencies(language, operationsData)
		if err != nil {
			return nil // best-effort: don't block generation on preview failures
		}
		if len(deps) > 0 {
			s.logger.Logf("Would add %d %s dependencies\n", len(deps), language)
		}
		return nil
	}

	// Validate project structure
	if err := s.dependencyWriter.ValidateProjectStructure(projectPath, language); err != nil {
		s.logger.Logf("Warning: %v\n", err)
	}

	// Add dependencies
	return s.dependencyWriter.AddDependencies(ctx, projectPath, language, operationsData, req)
}

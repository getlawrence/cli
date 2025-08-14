package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

// TemplateRenderer abstracts template execution for testability
type TemplateRenderer interface {
	GenerateInstructions(lang string, data templates.TemplateData) (string, error)
}

// TemplateGenerationStrategy implements direct code generation using templates
type TemplateGenerationStrategy struct {
	logger         logger.Logger
	templateEngine TemplateRenderer
}

// NewTemplateGenerationStrategy creates a new template-based generation strategy
func NewTemplateGenerationStrategy(templateEngine *templates.TemplateEngine, logger logger.Logger) *TemplateGenerationStrategy {
	return &TemplateGenerationStrategy{logger: logger, templateEngine: templateEngine}
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
		s.logger.Log("No code generation opportunities found; attempting fallback discovery")
	}

	directoryOpportunities := s.groupOpportunitiesByDirectory(opportunities)
	// Fallback: if some well-known language subdirectories exist but have no opportunities,
	// synthesize minimal InstallOTEL opportunities so we still generate bootstrap and inject init.
	s.addFallbackLanguageOpportunities(req.CodebasePath, directoryOpportunities)
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
			_ = s.determineOutputDirectory(req, directory) // compute path for generation below

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
	var outputDir string
	if language == "java" {
		outputDir = s.determineJavaOutputDirectory(req, directory)
	} else {
		outputDir = s.determineOutputDirectory(req, directory)
	}
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

// determineJavaOutputDirectory determines the output directory specifically for Java files
func (s *TemplateGenerationStrategy) determineJavaOutputDirectory(req types.GenerationRequest, directory string) string {
	baseDir := s.determineOutputDirectory(req, directory)

	// For Java, look for src/main/java structure
	javaSourceDir := filepath.Join(baseDir, "src", "main", "java")
	if _, err := os.Stat(javaSourceDir); err == nil {
		return javaSourceDir
	}

	// Fallback to base directory if src/main/java doesn't exist
	return baseDir
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
	// If instrumentations or components are planned, ensure OTEL core is also installed and initialized.
	// Many projects need the bootstrap even if the "missing otel" issue wasn't explicitly raised.
	if !data.InstallOTEL && (len(data.InstallInstrumentations) > 0 || len(data.InstallComponents) > 0) {
		data.InstallOTEL = true
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

// addFallbackLanguageOpportunities discovers common language subdirectories (e.g. python/, php/, ruby/, csharp/, javascript/, go/)
// under the provided root, and ensures there is at least one InstallOTEL opportunity per discovered directory.
func (s *TemplateGenerationStrategy) addFallbackLanguageOpportunities(root string, dirOpps map[string][]domain.Opportunity) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	// Heuristic: map subdir name to language identifier
	langByDir := map[string]string{
		"python":     "python",
		"php":        "php",
		"ruby":       "ruby",
		"go":         "go",
		"js":         "javascript",
		"javascript": "javascript",
		"csharp":     "dotnet",
		"dotnet":     "dotnet",
		"java":       "java",
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		lang, ok := langByDir[name]
		if !ok {
			continue
		}
		// If this directory already has opportunities, skip
		if _, exists := dirOpps[name]; exists {
			continue
		}
		// Quick sanity: verify directory contains some file indicative of a project to avoid polluting arbitrary dirs
		subdir := filepath.Join(root, name)
		if !s.seemsLikeProjectDir(subdir, lang) {
			continue
		}
		// Create a minimal InstallOTEL opportunity
		opp := domain.Opportunity{Type: domain.OpportunityInstallOTEL, Language: lang, FilePath: name}
		dirOpps[name] = []domain.Opportunity{opp}
	}
}

// seemsLikeProjectDir checks for a minimal marker file for the given language directory
func (s *TemplateGenerationStrategy) seemsLikeProjectDir(dir string, lang string) bool {
	lang = strings.ToLower(lang)
	checks := map[string][]string{
		"python":     {"requirements.txt", "app.py", "main.py"},
		"php":        {"composer.json", "index.php"},
		"ruby":       {"Gemfile", "app.rb"},
		"go":         {"go.mod", "main.go"},
		"javascript": {"package.json", "index.js"},
		"dotnet":     {"*.csproj", "Program.cs"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
	}
	markers := checks[lang]
	if len(markers) == 0 {
		return false
	}
	for _, m := range markers {
		if strings.Contains(m, "*") {
			if matches, _ := filepath.Glob(filepath.Join(dir, m)); len(matches) > 0 {
				return true
			}
			continue
		}
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}
	return false
}

// normalizeLanguageForGeneration maps language aliases to the canonical identifiers used by
// dependency management, templates, and injectors.
func normalizeLanguageForGeneration(language string) string {
	switch strings.ToLower(language) {
	case "js", "node", "nodejs":
		return "javascript"
	case "csharp":
		return "dotnet"
	default:
		return strings.ToLower(language)
	}
}

// Note: dependency management and entry-point modification are orchestrated outside this strategy now.

// findLanguageProjectDir tries to discover a more specific project directory for a language
// under the provided root. This helps when opportunities are grouped under "root" but the
// actual dependency files live in subfolders like "ruby/", "php/", etc.
// findLanguageProjectDir is retained for potential orchestrator use but currently unused in this strategy.
//
//lint:ignore U1000 kept for compatibility with orchestrators relying on this behavior
func (s *TemplateGenerationStrategy) findLanguageProjectDir(root, language string) (string, bool) {
	language = strings.ToLower(language)
	// Quick common folder names for examples
	candidates := []string{language}
	if language == "dotnet" || language == "csharp" {
		candidates = append(candidates, "csharp")
	}
	// Try candidate directories directly
	for _, c := range candidates {
		p := filepath.Join(root, c)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			// Without dependency validation, consider the presence of the directory sufficient here
			return p, true
		}
	}
	// Fallback: shallow scan for key files
	keyFiles := map[string][]string{
		"ruby":       {"Gemfile"},
		"php":        {"composer.json"},
		"python":     {"requirements.txt", "pyproject.toml"},
		"go":         {"go.mod"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
		"dotnet":     {".csproj"},
		"csharp":     {".csproj"},
		"javascript": {"package.json"},
	}
	wanted := keyFiles[language]
	if len(wanted) == 0 {
		return "", false
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return "", false
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		// Look for any of the wanted files in this subdir
		for _, k := range wanted {
			// Support suffix match for dotnet (csproj)
			if strings.HasPrefix(k, ".") {
				// suffix-only marker, handled below
				continue
			}
			if _, err := os.Stat(filepath.Join(dir, k)); err == nil {
				return dir, true
			}
		}
		if language == "dotnet" || language == "csharp" {
			// look for any *.csproj
			matches, _ := filepath.Glob(filepath.Join(dir, "*.csproj"))
			if len(matches) > 0 {
				return dir, true
			}
		}
	}
	return "", false
}

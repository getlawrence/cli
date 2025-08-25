package generator

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency"
	dependencyTypes "github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/logger"
)

// DependencyManager handles dependency operations
type DependencyManager interface {
	AddDependencies(ctx context.Context, projectPath, language string, operationsData *types.OperationsData, req types.GenerationRequest) error
	ValidateProjectStructure(projectPath, language string) error
	GetRequiredDependencies(language string, operationsData *types.OperationsData) ([]dependencyTypes.Dependency, error)
	GetEnhancedDependencies(language string, operationsData *types.OperationsData) ([]dependency.EnhancedDependency, error)
}

// EntryPointInjector handles entry point detection and injection
type EntryPointInjector interface {
	DetectEntryPoints(projectPath string, language string) ([]domain.EntryPoint, error)
	InjectOtelInitialization(ctx context.Context, entryPoint *domain.EntryPoint, operationsData *types.OperationsData, req types.GenerationRequest) ([]string, error)
}

// DirectoryExplorer handles file system operations
type DirectoryExplorer interface {
	ReadDir(path string) ([]os.DirEntry, error)
	FileExists(path string) (bool, error)
	GlobMatch(pattern string) ([]string, error)
}

// DefaultDirectoryExplorer implements DirectoryExplorer using os package
type DefaultDirectoryExplorer struct{}

func (d *DefaultDirectoryExplorer) ReadDir(path string) ([]os.DirEntry, error) {
	return os.ReadDir(path)
}

func (d *DefaultDirectoryExplorer) FileExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func (d *DefaultDirectoryExplorer) GlobMatch(pattern string) ([]string, error) {
	return filepath.Glob(pattern)
}

// OpportunityProcessor processes opportunities by directory and language
type OpportunityProcessor struct {
	logger   logger.Logger
	explorer DirectoryExplorer
}

func NewOpportunityProcessor(logger logger.Logger, explorer DirectoryExplorer) *OpportunityProcessor {
	return &OpportunityProcessor{
		logger:   logger,
		explorer: explorer,
	}
}

func (p *OpportunityProcessor) GroupByDirectory(opportunities []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)
	for _, o := range opportunities {
		key := o.FullPath
		if key == "" {
			key = o.FilePath
		}
		if key != "" {
			grouped[key] = append(grouped[key], o)
		}
	}
	return grouped
}

func (p *OpportunityProcessor) GroupByLanguage(opportunities []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)
	for _, o := range opportunities {
		if o.Language != "" {
			normalized := NormalizeLanguage(o.Language)
			grouped[normalized] = append(grouped[normalized], o)
		}
	}
	return grouped
}

func (p *OpportunityProcessor) AnalyzeOpportunities(opportunities []domain.Opportunity) *types.OperationsData {
	data := &types.OperationsData{
		InstallComponents: make(map[string][]string),
		RemoveComponents:  make(map[string][]string),
	}

	for _, opp := range opportunities {
		switch opp.Type {
		case domain.OpportunityInstallOTEL:
			data.InstallOTEL = true
		case domain.OpportunityInstallComponent:
			ct := string(opp.ComponentType)
			data.InstallComponents[ct] = append(data.InstallComponents[ct], opp.Component)
		case domain.OpportunityRemoveComponent:
			ct := string(opp.ComponentType)
			data.RemoveComponents[ct] = append(data.RemoveComponents[ct], opp.Component)
		}
	}

	// Auto-enable OTEL if instrumentations or components are being installed
	if !data.InstallOTEL && len(data.InstallComponents) > 0 {
		data.InstallOTEL = true
	}

	return data
}

// LanguageDetector detects project language from directory structure
type LanguageDetector struct {
	explorer DirectoryExplorer
}

func NewLanguageDetector(explorer DirectoryExplorer) *LanguageDetector {
	return &LanguageDetector{explorer: explorer}
}

func (d *LanguageDetector) DetectProjectLanguage(dir string) (string, bool) {
	languageMarkers := map[string][]string{
		"python":     {"requirements.txt", "app.py", "main.py", "setup.py", "pyproject.toml"},
		"php":        {"composer.json", "index.php"},
		"ruby":       {"Gemfile", "app.rb", "config.ru"},
		"go":         {"go.mod", "main.go", "go.sum"},
		"javascript": {"package.json", "index.js", "app.js"},
		"dotnet":     {"*.csproj", "Program.cs", "*.sln"},
		"java":       {"pom.xml", "build.gradle", "build.gradle.kts"},
	}

	for lang, markers := range languageMarkers {
		if d.hasAnyMarker(dir, markers) {
			return lang, true
		}
	}

	return "", false
}

func (d *LanguageDetector) hasAnyMarker(dir string, markers []string) bool {
	for _, marker := range markers {
		if strings.Contains(marker, "*") {
			pattern := filepath.Join(dir, marker)
			if matches, _ := d.explorer.GlobMatch(pattern); len(matches) > 0 {
				return true
			}
		} else {
			path := filepath.Join(dir, marker)
			if exists, _ := d.explorer.FileExists(path); exists {
				return true
			}
		}
	}
	return false
}

// DependencyHandler encapsulates dependency-related operations
type DependencyHandler struct {
	manager DependencyManager
	logger  logger.Logger
}

func NewDependencyHandler(manager DependencyManager, logger logger.Logger) *DependencyHandler {
	return &DependencyHandler{
		manager: manager,
		logger:  logger,
	}
}

func (h *DependencyHandler) ProcessDependencies(ctx context.Context, language string, ops *types.OperationsData, projectPath string, req types.GenerationRequest) error {
	if !h.requiresDependencies(ops) {
		return nil
	}

	if req.Config.DryRun {
		return h.logDependencies(language, ops)
	}

	return h.manager.AddDependencies(ctx, projectPath, language, ops, req)
}

func (h *DependencyHandler) requiresDependencies(ops *types.OperationsData) bool {
	return ops.InstallOTEL || len(ops.InstallComponents) > 0
}

func (h *DependencyHandler) logDependencies(language string, ops *types.OperationsData) error {
	// Try enhanced dependencies first
	if enhancedDeps, err := h.manager.GetEnhancedDependencies(language, ops); err == nil && len(enhancedDeps) > 0 {
		h.logger.Logf("Would add %d %s dependencies:\n", len(enhancedDeps), language)
		for _, dep := range enhancedDeps {
			version := ""
			if dep.Metadata != nil && dep.Metadata.LatestVersion != "" {
				version = "@" + dep.Metadata.LatestVersion
			}
			h.logger.Logf("  - %s%s\n", dep.Dependency.ImportPath, version)
		}
		return nil
	}

	// Fallback to basic dependencies
	if deps, err := h.manager.GetRequiredDependencies(language, ops); err == nil && len(deps) > 0 {
		h.logger.Logf("Would add %d %s dependencies:\n", len(deps), language)
		for _, dep := range deps {
			h.logger.Logf("  - %s\n", dep.ImportPath)
		}
	}

	return nil
}

// EntryPointHandler encapsulates entry point operations
type EntryPointHandler struct {
	injector EntryPointInjector
	logger   logger.Logger
}

func NewEntryPointHandler(injector EntryPointInjector, logger logger.Logger) *EntryPointHandler {
	return &EntryPointHandler{
		injector: injector,
		logger:   logger,
	}
}

func (h *EntryPointHandler) ProcessOtelInitialization(ctx context.Context, language string, ops *types.OperationsData, dir string, req types.GenerationRequest) error {
	if !h.requiresInitialization(ops) {
		return nil
	}

	dirPath := h.resolveDirPath(dir, req.CodebasePath)

	entryPoints, err := h.injector.DetectEntryPoints(dirPath, language)
	if err != nil || len(entryPoints) == 0 {
		return err
	}

	best := h.selectBestEntryPoint(entryPoints)
	_, err = h.injector.InjectOtelInitialization(ctx, &best, ops, req)
	return err
}

func (h *EntryPointHandler) requiresInitialization(ops *types.OperationsData) bool {
	return ops.InstallOTEL || len(ops.InstallComponents) > 0
}

func (h *EntryPointHandler) resolveDirPath(dir, codebasePath string) string {
	if dir == "" {
		return codebasePath
	}
	return dir
}

func (h *EntryPointHandler) selectBestEntryPoint(entryPoints []domain.EntryPoint) domain.EntryPoint {
	if len(entryPoints) == 0 {
		return domain.EntryPoint{}
	}

	best := entryPoints[0]
	for _, ep := range entryPoints[1:] {
		if ep.Confidence > best.Confidence {
			best = ep
		}
	}
	return best
}

// OrchestratedTemplateStrategy orchestrates template generation with dependency management
type OrchestratedTemplateStrategy struct {
	tmpl         types.CodeGenerationStrategy
	depHandler   *DependencyHandler
	entryHandler *EntryPointHandler
	oppProcessor *OpportunityProcessor
	langDetector *LanguageDetector
	logger       logger.Logger
}

// NewOrchestratedTemplateStrategy creates a new orchestrated template strategy
func NewOrchestratedTemplateStrategy(
	tmpl types.CodeGenerationStrategy,
	deps DependencyManager,
	inj EntryPointInjector,
	logger logger.Logger,
) *OrchestratedTemplateStrategy {
	explorer := &DefaultDirectoryExplorer{}
	return &OrchestratedTemplateStrategy{
		tmpl:         tmpl,
		depHandler:   NewDependencyHandler(deps, logger),
		entryHandler: NewEntryPointHandler(inj, logger),
		oppProcessor: NewOpportunityProcessor(logger, explorer),
		langDetector: NewLanguageDetector(explorer),
		logger:       logger,
	}
}

func (s *OrchestratedTemplateStrategy) GetName() string            { return "Template-based" }
func (s *OrchestratedTemplateStrategy) IsAvailable() bool          { return true }
func (s *OrchestratedTemplateStrategy) GetRequiredFlags() []string { return []string{} }

func (s *OrchestratedTemplateStrategy) GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	// Group opportunities by directory
	dirOpps := s.oppProcessor.GroupByDirectory(opportunities)

	// Process each directory
	for dir, opps := range dirOpps {
		if err := s.processDirectory(ctx, dir, opps, req); err != nil {
			s.logger.Logf("Warning: failed to process directory %s: %v\n", dir, err)
		}
	}

	// Run the template generation
	return s.tmpl.GenerateCode(ctx, opportunities, req)
}

func (s *OrchestratedTemplateStrategy) processDirectory(ctx context.Context, dir string, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	byLang := s.oppProcessor.GroupByLanguage(opportunities)
	projectPath := s.determineProjectPath(dir, req.CodebasePath)

	// Validate project structure if needed
	if s.shouldValidateProjectStructure(byLang) && !req.Config.DryRun {
		if err := s.validateProjectStructure(projectPath, byLang); err != nil {
			s.logger.Logf("Warning: %v\n", err)
		}
	}

	// Process each language
	for lang, langOpps := range byLang {
		ops := s.oppProcessor.AnalyzeOpportunities(langOpps)
		if err := s.processLanguage(ctx, lang, ops, projectPath, dir, req); err != nil {
			s.logger.Logf("Warning: failed to process language %s in %s: %v\n", lang, dir, err)
		}
	}

	return nil
}

func (s *OrchestratedTemplateStrategy) processLanguage(ctx context.Context, language string, ops *types.OperationsData, projectPath, dir string, req types.GenerationRequest) error {
	// Process dependencies
	if err := s.depHandler.ProcessDependencies(ctx, language, ops, projectPath, req); err != nil {
		return err
	}

	// Process OTEL initialization
	return s.entryHandler.ProcessOtelInitialization(ctx, language, ops, dir, req)
}

func (s *OrchestratedTemplateStrategy) shouldValidateProjectStructure(byLang map[string][]domain.Opportunity) bool {
	for _, langOpps := range byLang {
		ops := s.oppProcessor.AnalyzeOpportunities(langOpps)
		if ops.InstallOTEL || len(ops.InstallComponents) > 0 {
			return true
		}
	}
	return false
}

func (s *OrchestratedTemplateStrategy) validateProjectStructure(projectPath string, byLang map[string][]domain.Opportunity) error {
	for lang := range byLang {
		return s.depHandler.manager.ValidateProjectStructure(projectPath, lang)
	}

	return nil
}

func (s *OrchestratedTemplateStrategy) determineProjectPath(dir, codebasePath string) string {
	dirName := filepath.Base(dir)

	if dirName == "root" || dir == codebasePath {
		return codebasePath
	}

	if dir != codebasePath {
		return filepath.Join(codebasePath, dirName)
	}

	return codebasePath
}

func (s *OrchestratedTemplateStrategy) addFallbackLanguageOpportunities(root string, dirOpps map[string][]domain.Opportunity) {
	entries, err := s.oppProcessor.explorer.ReadDir(root)
	if err != nil {
		return
	}

	langByDir := map[string]string{
		"python": "python", "php": "php", "ruby": "ruby",
		"go": "go", "js": "javascript", "javascript": "javascript",
		"csharp": "dotnet", "dotnet": "dotnet", "java": "java",
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		name := strings.ToLower(entry.Name())
		expectedLang, ok := langByDir[name]
		if !ok {
			continue
		}

		if _, exists := dirOpps[name]; exists {
			continue
		}

		subdir := filepath.Join(root, name)
		if detectedLang, found := s.langDetector.DetectProjectLanguage(subdir); found && detectedLang == expectedLang {
			opp := domain.Opportunity{
				Type:     domain.OpportunityInstallOTEL,
				Language: expectedLang,
				FilePath: name,
				FullPath: subdir,
			}
			dirOpps[name] = []domain.Opportunity{opp}
		}
	}
}

// NormalizeLanguage normalizes language names to standard form
func NormalizeLanguage(language string) string {
	switch strings.ToLower(language) {
	case "js", "node", "nodejs":
		return "javascript"
	case "csharp":
		return "dotnet"
	default:
		return strings.ToLower(language)
	}
}

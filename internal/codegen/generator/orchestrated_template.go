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

// OrchestratedTemplateStrategy composes the template strategy with dependency management
// and entry-point injection. It enables unit testing the template strategy separately.
type DependencyManager interface {
	AddDependencies(ctx context.Context, projectPath, language string, operationsData *types.OperationsData, req types.GenerationRequest) error
	ValidateProjectStructure(projectPath, language string) error
	GetRequiredDependencies(language string, operationsData *types.OperationsData) ([]dependencyTypes.Dependency, error)
	GetEnhancedDependencies(language string, operationsData *types.OperationsData) ([]dependency.EnhancedDependency, error)
}

type EntryPointInjector interface {
	DetectEntryPoints(projectPath string, language string) ([]domain.EntryPoint, error)
	InjectOtelInitialization(ctx context.Context, entryPoint *domain.EntryPoint, operationsData *types.OperationsData, req types.GenerationRequest) ([]string, error)
}

type OrchestratedTemplateStrategy struct {
	tmpl   types.CodeGenerationStrategy
	deps   DependencyManager
	inj    EntryPointInjector
	logger logger.Logger
}

func NewOrchestratedTemplateStrategy(tmpl types.CodeGenerationStrategy, deps DependencyManager, inj EntryPointInjector, logger logger.Logger) *OrchestratedTemplateStrategy {
	return &OrchestratedTemplateStrategy{tmpl: tmpl, deps: deps, inj: inj, logger: logger}
}

func (s *OrchestratedTemplateStrategy) GetName() string            { return "Template-based" }
func (s *OrchestratedTemplateStrategy) IsAvailable() bool          { return true }
func (s *OrchestratedTemplateStrategy) GetRequiredFlags() []string { return []string{} }

func (s *OrchestratedTemplateStrategy) GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	// Compute operations by dir/lang to orchestrate deps/injection
	dirOpps := groupByDirectory(opportunities)
	// Apply fallback discovery similar to template strategy so we orchestrate matching work
	s.addFallbackLanguageOpportunities(req.CodebasePath, dirOpps)

	for dir, opps := range dirOpps {
		byLang := groupByLanguage(opps)
		for lang, langOpps := range byLang {
			normalized := normalizeLanguage(lang)
			ops := analyze(langOpps)

			// Dependencies
			if ops.InstallOTEL || len(ops.InstallInstrumentations) > 0 || len(ops.InstallComponents) > 0 {
				projectPath := req.CodebasePath
				// For most languages, dependencies are managed at the project root
				// Only use subdirectory for languages that support nested dependency management
				if dir != "root" && !isProjectRootDependencyLanguage(normalized) {
					projectPath = filepath.Join(req.CodebasePath, dir)
				}
				if req.Config.DryRun {
					// Try to get enhanced dependencies with versions first
					if enhancedDeps, err := s.deps.GetEnhancedDependencies(normalized, ops); err == nil && len(enhancedDeps) > 0 {
						s.logger.Logf("Would add %d %s dependencies:\n", len(enhancedDeps), normalized)
						for _, dep := range enhancedDeps {
							version := ""
							if dep.Metadata != nil && dep.Metadata.LatestVersion != "" {
								version = "@" + dep.Metadata.LatestVersion
							}
							s.logger.Logf("  - %s%s\n", dep.Dependency.ImportPath, version)
						}
					} else if deps, err := s.deps.GetRequiredDependencies(normalized, ops); err == nil && len(deps) > 0 {
						// Fallback to basic dependencies
						s.logger.Logf("Would add %d %s dependencies:\n", len(deps), normalized)
						for _, dep := range deps {
							s.logger.Logf("  - %s\n", dep.ImportPath)
						}
					}
				} else {
					if err := s.deps.ValidateProjectStructure(projectPath, normalized); err != nil {
						s.logger.Logf("Warning: %v\n", err)
					}
					if err := s.deps.AddDependencies(ctx, projectPath, normalized, ops, req); err != nil {
						s.logger.Logf("Warning: failed to add dependencies for %s: %v\n", normalized, err)
					}
				}
			}

			// Inject OTEL initialization into entry point when planned
			if ops.InstallOTEL || len(ops.InstallInstrumentations) > 0 || len(ops.InstallComponents) > 0 {
				dirPath := req.CodebasePath
				if dir != "root" {
					dirPath = filepath.Join(req.CodebasePath, dir)
				}
				eps, _ := s.inj.DetectEntryPoints(dirPath, normalized)
				if len(eps) > 0 {
					// Choose best by confidence
					best := eps[0]
					for _, ep := range eps {
						if ep.Confidence > best.Confidence {
							best = ep
						}
					}
					if _, err := s.inj.InjectOtelInitialization(ctx, &best, ops, req); err != nil {
						s.logger.Logf("Warning: failed to modify entry point for %s: %v\n", normalized, err)
					}
				}
			}
		}
	}

	// Finally, run the pure template generation
	return s.tmpl.GenerateCode(ctx, opportunities, req)
}

// Helpers (duplicated minimal logic from template for orchestration)

func groupByDirectory(opps []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)
	for _, o := range opps {
		if o.FilePath != "" {
			grouped[o.FilePath] = append(grouped[o.FilePath], o)
		}
	}
	return grouped
}

func groupByLanguage(opps []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)
	for _, o := range opps {
		if o.Language != "" {
			grouped[strings.ToLower(o.Language)] = append(grouped[strings.ToLower(o.Language)], o)
		}
	}
	return grouped
}

func normalizeLanguage(language string) string {
	switch strings.ToLower(language) {
	case "js", "node", "nodejs":
		return "javascript"
	case "csharp":
		return "dotnet"
	default:
		return strings.ToLower(language)
	}
}

// isProjectRootDependencyLanguage returns true for languages where dependencies are managed at project root
func isProjectRootDependencyLanguage(language string) bool {
	switch language {
	case "java", "csharp", "dotnet", "go", "php", "ruby":
		return true
	default:
		return false
	}
}

func analyze(opps []domain.Opportunity) *types.OperationsData {
	data := &types.OperationsData{InstallComponents: map[string][]string{}, RemoveComponents: map[string][]string{}}
	for _, opp := range opps {
		switch opp.Type {
		case domain.OpportunityInstallOTEL:
			data.InstallOTEL = true
		case domain.OpportunityInstallComponent:
			if opp.ComponentType == domain.ComponentTypeInstrumentation {
				data.InstallInstrumentations = append(data.InstallInstrumentations, opp.Component)
			} else {
				ct := string(opp.ComponentType)
				data.InstallComponents[ct] = append(data.InstallComponents[ct], opp.Component)
			}
		case domain.OpportunityRemoveComponent:
			ct := string(opp.ComponentType)
			data.RemoveComponents[ct] = append(data.RemoveComponents[ct], opp.Component)
		}
	}
	if !data.InstallOTEL && (len(data.InstallInstrumentations) > 0 || len(data.InstallComponents) > 0) {
		data.InstallOTEL = true
	}
	return data
}

// addFallbackLanguageOpportunities mirrors the template strategy's fallback to discover language dirs
func (s *OrchestratedTemplateStrategy) addFallbackLanguageOpportunities(root string, dirOpps map[string][]domain.Opportunity) {
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	langByDir := map[string]string{"python": "python", "php": "php", "ruby": "ruby", "go": "go", "js": "javascript", "javascript": "javascript", "csharp": "dotnet", "dotnet": "dotnet", "java": "java"}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := strings.ToLower(e.Name())
		lang, ok := langByDir[name]
		if !ok {
			continue
		}
		if _, exists := dirOpps[name]; exists {
			continue
		}
		subdir := filepath.Join(root, name)
		if !seemsLikeProjectDir(subdir, lang) {
			continue
		}
		opp := domain.Opportunity{Type: domain.OpportunityInstallOTEL, Language: lang, FilePath: name}
		dirOpps[name] = []domain.Opportunity{opp}
	}
}

func seemsLikeProjectDir(dir string, lang string) bool {
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

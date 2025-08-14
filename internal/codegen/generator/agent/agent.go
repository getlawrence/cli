package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/internal/templates"
)

// AIGenerationStrategy implements code generation using AI agents
type AIGenerationStrategy struct {
	agentDetector  *agents.Detector
	templateEngine *templates.TemplateEngine
	logger         logger.Logger

	// Cached context from analysis to enrich agent prompts
	projectLanguages   []string
	existingLibraries  []string
	existingPackages   []string
	directoryLanguages map[string]string
	rootDirectory      string
	directoryLibraries map[string][]string
	directoryPackages  map[string][]string
}

// NewAIGenerationStrategy creates a new AI-based generation strategy
func NewAIGenerationStrategy(agentDetector *agents.Detector, templateEngine *templates.TemplateEngine, logger logger.Logger) *AIGenerationStrategy {
	return &AIGenerationStrategy{
		agentDetector:  agentDetector,
		templateEngine: templateEngine,
		logger:         logger,
	}
}

// SetAnalysisContext provides analysis-derived context to include in agent prompts
func (s *AIGenerationStrategy) SetAnalysisContext(analysis *detector.Analysis) {
	if analysis == nil || analysis.DirectoryAnalyses == nil {
		return
	}
	langSet := make(map[string]bool)
	libSet := make(map[string]bool)
	pkgSet := make(map[string]bool)
	s.directoryLanguages = make(map[string]string)
	s.directoryLibraries = make(map[string][]string)
	s.directoryPackages = make(map[string][]string)
	for directory, dir := range analysis.DirectoryAnalyses {
		if dir.Language != "" {
			langSet[dir.Language] = true
			s.directoryLanguages[directory] = dir.Language
		}
		for _, lib := range dir.Libraries {
			if lib.Name != "" {
				libSet[lib.Name] = true
				s.directoryLibraries[directory] = append(s.directoryLibraries[directory], lib.Name)
			}
		}
		for _, pkg := range dir.Packages {
			if pkg.Name != "" {
				pkgSet[pkg.Name] = true
				s.directoryPackages[directory] = append(s.directoryPackages[directory], pkg.Name)
			}
		}
	}
	s.projectLanguages = s.sliceFromSet(langSet)
	s.existingLibraries = s.sliceFromSet(libSet)
	s.existingPackages = s.sliceFromSet(pkgSet)
	s.rootDirectory = analysis.RootPath
}

func (s *AIGenerationStrategy) sliceFromSet(set map[string]bool) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return out
}

// GetName returns the name of this strategy
func (s *AIGenerationStrategy) GetName() string {
	return "AI Agent"
}

// IsAvailable checks if AI agents are available on the system
func (s *AIGenerationStrategy) IsAvailable() bool {
	availableAgents := s.agentDetector.DetectAvailableAgents()
	return len(availableAgents) > 0
}

// GetRequiredFlags returns flags required for AI generation
func (s *AIGenerationStrategy) GetRequiredFlags() []string {
	return []string{"agent"}
}

// GenerateCode generates code using AI agents
func (s *AIGenerationStrategy) GenerateCode(ctx context.Context, opportunities []domain.Opportunity, req types.GenerationRequest) error {
	// Verify requested agent is available
	if err := s.verifyAgentAvailability(agents.AgentType(req.Config.AgentType)); err != nil {
		return err
	}

	// Group opportunities by language to create comprehensive instructions
	languageOpportunities := s.groupOpportunitiesByLanguage(opportunities)

	if len(languageOpportunities) == 0 {
		s.logger.Log("No opportunities to process")
		return nil
	}

	// Generate comprehensive instructions
	allInstructions, err := s.generateInstructionsForLanguages(languageOpportunities, req)
	if err != nil {
		return err
	}

	if len(allInstructions) == 0 {
		s.logger.Log("No instructions generated")
		return nil
	}

	// Combine and send to agent, including rich analysis context
	return s.sendInstructionsToAgent(allInstructions, req, languageOpportunities)
}

func (s *AIGenerationStrategy) verifyAgentAvailability(agentType agents.AgentType) error {
	availableAgents := s.agentDetector.DetectAvailableAgents()
	for _, agent := range availableAgents {
		if agent.Type == agentType {
			return nil
		}
	}
	return fmt.Errorf("requested agent %s is not available", agentType)
}

func (s *AIGenerationStrategy) generateInstructionsForLanguages(languageOpportunities map[string][]domain.Opportunity, req types.GenerationRequest) ([]string, error) {
	var allInstructions []string

	for language, langOpportunities := range languageOpportunities {
		// Collect all instrumentations for this language
		allInstrumentations := s.collectAllInstrumentations(langOpportunities)

		// Generate instructions for this language
		instruction, err := s.templateEngine.GenerateInstructions(
			language,
			templates.TemplateData{
				Language:         language,
				Instrumentations: allInstrumentations,
				ServiceName:      filepath.Base(req.CodebasePath),
				// Best-effort include exporter/propagator hints from config
				TraceExporterType: func() string {
					if req.OTEL != nil {
						return req.OTEL.Exporters.Traces.Type
					}
					return ""
				}(),
				TraceProtocol: func() string {
					if req.OTEL != nil {
						return req.OTEL.Exporters.Traces.Protocol
					}
					return ""
				}(),
				TraceEndpoint: func() string {
					if req.OTEL != nil {
						return req.OTEL.Exporters.Traces.Endpoint
					}
					return ""
				}(),
				Propagators: func() []string {
					if req.OTEL != nil {
						return req.OTEL.Propagators
					}
					return nil
				}(),
			},
		)
		if err != nil {
			s.logger.Logf("Warning: failed to generate comprehensive instructions for %s: %v\n", language, err)
			continue
		}

		s.logger.Logf("Generated comprehensive instrumentation instructions for %s\n", language)
		allInstructions = append(allInstructions, instruction)
	}

	return allInstructions, nil
}

func (s *AIGenerationStrategy) sendInstructionsToAgent(allInstructions []string, req types.GenerationRequest, languageOpportunities map[string][]domain.Opportunity) error {
	language := req.Language
	if language == "" {
		language = "multi-language" // Default for comprehensive guides
	}

	// Build per-directory plans for prompt organization
	var directoryPlans []templates.DirectoryPlan
	for directory, lang := range s.directoryLanguages {
		plan := templates.DirectoryPlan{
			Directory:         directory,
			Language:          lang,
			InstallComponents: make(map[string][]string),
			RemoveComponents:  make(map[string][]string),
		}

		if libs := s.directoryLibraries[directory]; len(libs) > 0 {
			plan.Libraries = libs
			sort.Strings(plan.Libraries)
		}
		if pkgs := s.directoryPackages[directory]; len(pkgs) > 0 {
			plan.Packages = pkgs
			sort.Strings(plan.Packages)
		}

		if opps, ok := languageOpportunities[lang]; ok {
			for _, opp := range opps {
				// best-effort directory match
				if opp.FilePath == directory || strings.Contains(opp.FilePath, directory) {
					if opp.Framework != "" {
						plan.DetectedFrameworks = append(plan.DetectedFrameworks, opp.Framework)
					}
					switch opp.Type {
					case domain.OpportunityInstallOTEL:
						plan.InstallOTEL = true
					case domain.OpportunityInstallComponent:
						if opp.ComponentType == domain.ComponentTypeInstrumentation {
							plan.InstallInstrumentations = append(plan.InstallInstrumentations, string(opp.Component))
						} else {
							ctype := string(opp.ComponentType)
							plan.InstallComponents[ctype] = append(plan.InstallComponents[ctype], string(opp.Component))
						}
					case domain.OpportunityRemoveComponent:
						ctype := string(opp.ComponentType)
						plan.RemoveComponents[ctype] = append(plan.RemoveComponents[ctype], string(opp.Component))
					}
					if opp.Suggestion != "" {
						plan.Issues = append(plan.Issues, opp.Suggestion)
					}
				}
			}
		}

		sort.Strings(plan.DetectedFrameworks)
		sort.Strings(plan.InstallInstrumentations)
		for _, v := range plan.InstallComponents {
			sort.Strings(v)
		}
		for _, v := range plan.RemoveComponents {
			sort.Strings(v)
		}

		directoryPlans = append(directoryPlans, plan)
	}

	agentRequest := agents.AgentExecutionRequest{
		Language:       language,
		TargetDir:      req.CodebasePath,
		Directory:      req.CodebasePath,
		DirectoryPlans: directoryPlans,
	}

	// If requested (or in dry-run), generate and show/save the prompt before execution
	if req.Config.ShowPrompt || req.Config.SavePrompt != "" || req.Config.DryRun {
		prompt, err := s.agentDetector.GeneratePrompt(agentRequest)
		if err != nil {
			return fmt.Errorf("failed to generate prompt: %v", err)
		}
		if req.Config.ShowPrompt || req.Config.DryRun {
			fmt.Print("\n===== AGENT PROMPT =====\n\n")
			s.logger.Log(prompt)
			fmt.Print("\n========================\n\n")
		}
		if req.Config.SavePrompt != "" {
			if err := os.WriteFile(req.Config.SavePrompt, []byte(prompt), 0o644); err != nil {
				return fmt.Errorf("failed to save prompt to %s: %v", req.Config.SavePrompt, err)
			}
			s.logger.Logf("Saved prompt to %s\n", req.Config.SavePrompt)
		}
	}

	// In AI dry-run mode, skip invoking the external agent
	if req.Config.DryRun {
		s.logger.Log("AI dry-run: skipping agent execution")
		return nil
	}

	// Execute with selected agent - single call with comprehensive instructions
	if err := s.agentDetector.ExecuteWithAgent(agents.AgentType(req.Config.AgentType), agentRequest); err != nil {
		return fmt.Errorf("failed to execute with agent %s: %v", req.Config.AgentType, err)
	}

	s.logger.Logf("Successfully sent comprehensive instrumentation guide to %s agent\n", req.Config.AgentType)
	return nil
}

func (s *AIGenerationStrategy) combineInstructions(allInstructions []string, codebasePath string) string {
	if len(allInstructions) == 1 {
		return allInstructions[0]
	}

	// Multiple languages - create a multi-language guide
	combinedInstructions := fmt.Sprintf("# OpenTelemetry Instrumentation Guide for %s\n\n", filepath.Base(codebasePath))
	combinedInstructions += "This guide provides comprehensive OpenTelemetry instrumentation for multiple languages detected in your project.\n\n"

	for i, instructions := range allInstructions {
		if i > 0 {
			combinedInstructions += "\n\n---\n\n"
		}
		combinedInstructions += instructions
	}

	return combinedInstructions
}

// groupOpportunitiesByLanguage groups opportunities by programming language
func (s *AIGenerationStrategy) groupOpportunitiesByLanguage(opportunities []domain.Opportunity) map[string][]domain.Opportunity {
	grouped := make(map[string][]domain.Opportunity)

	for _, opp := range opportunities {
		if opp.Language != "" {
			grouped[opp.Language] = append(grouped[opp.Language], opp)
		}
	}

	return grouped
}

// collectAllInstrumentations extracts unique instrumentations from all opportunities
func (s *AIGenerationStrategy) collectAllInstrumentations(opportunities []domain.Opportunity) []string {
	var instrumentations []string
	for _, opp := range opportunities {
		if opp.ComponentType == domain.ComponentTypeInstrumentation {
			instrumentations = append(instrumentations, string(opp.Component))
		}
	}
	return instrumentations
}

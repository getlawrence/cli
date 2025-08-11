package agent

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/templates"
	"github.com/getlawrence/cli/internal/ui"
)

// AIGenerationStrategy implements code generation using AI agents
type AIGenerationStrategy struct {
	agentDetector  *agents.Detector
	templateEngine *templates.TemplateEngine

	// Cached context from analysis to enrich agent prompts
	projectLanguages   []string
	existingLibraries  []string
	existingPackages   []string
	directoryLanguages map[string]string
	rootDirectory      string
}

// NewAIGenerationStrategy creates a new AI-based generation strategy
func NewAIGenerationStrategy(agentDetector *agents.Detector, templateEngine *templates.TemplateEngine) *AIGenerationStrategy {
	return &AIGenerationStrategy{
		agentDetector:  agentDetector,
		templateEngine: templateEngine,
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
	for directory, dir := range analysis.DirectoryAnalyses {
		if dir.Language != "" {
			langSet[dir.Language] = true
			s.directoryLanguages[directory] = dir.Language
		}
		for _, lib := range dir.Libraries {
			if lib.Name != "" {
				libSet[lib.Name] = true
			}
		}
		for _, pkg := range dir.Packages {
			if pkg.Name != "" {
				pkgSet[pkg.Name] = true
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
		ui.Log("No opportunities to process")
		return nil
	}

	// Generate comprehensive instructions
	allInstructions, err := s.generateInstructionsForLanguages(languageOpportunities, req)
	if err != nil {
		return err
	}

	if len(allInstructions) == 0 {
		ui.Log("No instructions generated")
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

		// Generate comprehensive instructions for this language
		instruction, err := s.templateEngine.GenerateComprehensiveInstructions(
			language,
			allInstrumentations,
			req.CodebasePath,
		)
		if err != nil {
			ui.Logf("Warning: failed to generate comprehensive instructions for %s: %v\n", language, err)
			continue
		}

		ui.Logf("Generated comprehensive instrumentation instructions for %s\n", language)
		allInstructions = append(allInstructions, instruction)
	}

	return allInstructions, nil
}

func (s *AIGenerationStrategy) sendInstructionsToAgent(allInstructions []string, req types.GenerationRequest, languageOpportunities map[string][]domain.Opportunity) error {
	// Combine all language instructions into a single comprehensive guide
	combinedInstructions := s.combineInstructions(allInstructions, req.CodebasePath)

	ui.Logf("Generated comprehensive instrumentation guide\n")

	// Determine the primary language or use "multi-language" if multiple
	language := req.Language
	if language == "" {
		language = "multi-language" // Default for comprehensive guides
	}

	// Aggregate analysis context across languages
	projectLanguageSet := make(map[string]bool)
	frameworkSet := make(map[string]bool)
	installInstrumentationSet := make(map[string]bool)
	installComponents := make(map[string][]string)
	removeComponents := make(map[string][]string)
	var issues []string
	installOTEL := false

	for lang, opps := range languageOpportunities {
		projectLanguageSet[lang] = true
		for _, opp := range opps {
			if opp.Framework != "" {
				frameworkSet[opp.Framework] = true
			}
			switch opp.Type {
			case domain.OpportunityInstallOTEL:
				installOTEL = true
			case domain.OpportunityInstallComponent:
				if opp.ComponentType == domain.ComponentTypeInstrumentation {
					installInstrumentationSet[opp.Component] = true
				} else {
					ctype := string(opp.ComponentType)
					installComponents[ctype] = append(installComponents[ctype], opp.Component)
				}
			case domain.OpportunityRemoveComponent:
				ctype := string(opp.ComponentType)
				removeComponents[ctype] = append(removeComponents[ctype], opp.Component)
			}
			if opp.Suggestion != "" {
				issues = append(issues, opp.Suggestion)
			}
		}
	}

	// De-duplicate component lists
	dedupe := func(items []string) []string {
		seen := make(map[string]bool)
		var out []string
		for _, it := range items {
			if !seen[it] {
				seen[it] = true
				out = append(out, it)
			}
		}
		return out
	}
	for k, v := range installComponents {
		installComponents[k] = dedupe(v)
	}
	for k, v := range removeComponents {
		removeComponents[k] = dedupe(v)
	}

	var projectLanguages []string
	for l := range projectLanguageSet {
		projectLanguages = append(projectLanguages, l)
	}
	var detectedFrameworks []string
	for f := range frameworkSet {
		detectedFrameworks = append(detectedFrameworks, f)
	}
	var installInstrumentations []string
	for inst := range installInstrumentationSet {
		installInstrumentations = append(installInstrumentations, inst)
	}

	// Create agent execution request with extended context
	agentRequest := agents.AgentExecutionRequest{
		Language:                language,
		Instructions:            combinedInstructions,
		TargetDir:               req.CodebasePath,
		ServiceName:             filepath.Base(req.CodebasePath), // Use directory name as default service name
		Directory:               filepath.Base(req.CodebasePath),
		DirectoryLanguages:      s.directoryLanguages,
		DetectedFrameworks:      detectedFrameworks,
		ProjectLanguages:        projectLanguages,
		InstallOTEL:             installOTEL,
		InstallInstrumentations: installInstrumentations,
		InstallComponents:       installComponents,
		RemoveComponents:        removeComponents,
		ExistingLibraries:       s.existingLibraries,
		ExistingPackages:        s.existingPackages,
		Issues:                  issues,
	}

	// If requested (or in dry-run), generate and show/save the prompt before execution
	if req.Config.ShowPrompt || req.Config.SavePrompt != "" || req.Config.DryRun {
		prompt, err := s.agentDetector.GeneratePrompt(agentRequest)
		if err != nil {
			return fmt.Errorf("failed to generate prompt: %v", err)
		}
		if req.Config.ShowPrompt || req.Config.DryRun {
			fmt.Print("\n===== AGENT PROMPT =====\n\n")
			ui.Log(prompt)
			fmt.Print("\n========================\n\n")
		}
		if req.Config.SavePrompt != "" {
			if err := os.WriteFile(req.Config.SavePrompt, []byte(prompt), 0o644); err != nil {
				return fmt.Errorf("failed to save prompt to %s: %v", req.Config.SavePrompt, err)
			}
			ui.Logf("Saved prompt to %s\n", req.Config.SavePrompt)
		}
	}

	// In AI dry-run mode, skip invoking the external agent
	if req.Config.DryRun {
		ui.Log("AI dry-run: skipping agent execution")
		return nil
	}

	// Execute with selected agent - single call with comprehensive instructions
	if err := s.agentDetector.ExecuteWithAgent(agents.AgentType(req.Config.AgentType), agentRequest); err != nil {
		return fmt.Errorf("failed to execute with agent %s: %v", req.Config.AgentType, err)
	}

	ui.Logf("Successfully sent comprehensive instrumentation guide to %s agent\n", req.Config.AgentType)
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

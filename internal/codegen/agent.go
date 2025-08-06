package codegen

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/templates"
)

// AIGenerationStrategy implements code generation using AI agents
type AIGenerationStrategy struct {
	agentMgr    *agents.Detector
	templateMgr *templates.Manager
}

// NewAIGenerationStrategy creates a new AI-based generation strategy
func NewAIGenerationStrategy(agentMgr *agents.Detector, templateMgr *templates.Manager) *AIGenerationStrategy {
	return &AIGenerationStrategy{
		agentMgr:    agentMgr,
		templateMgr: templateMgr,
	}
}

// GetName returns the name of this strategy
func (s *AIGenerationStrategy) GetName() string {
	return "AI Agent"
}

// IsAvailable checks if AI agents are available on the system
func (s *AIGenerationStrategy) IsAvailable() bool {
	availableAgents := s.agentMgr.DetectAvailableAgents()
	return len(availableAgents) > 0
}

// GetRequiredFlags returns flags required for AI generation
func (s *AIGenerationStrategy) GetRequiredFlags() []string {
	return []string{"agent"}
}

// GenerateCode generates code using AI agents
func (s *AIGenerationStrategy) GenerateCode(ctx context.Context, opportunities []Opportunity, req GenerationRequest) error {
	// Verify requested agent is available
	if err := s.verifyAgentAvailability(agents.AgentType(req.Config.AgentType)); err != nil {
		return err
	}

	// Group opportunities by language to create comprehensive instructions
	languageOpportunities := s.groupOpportunitiesByLanguage(opportunities)

	if len(languageOpportunities) == 0 {
		fmt.Println("No opportunities to process")
		return nil
	}

	// Generate comprehensive instructions
	allInstructions, err := s.generateInstructionsForLanguages(languageOpportunities, req)
	if err != nil {
		return err
	}

	if len(allInstructions) == 0 {
		fmt.Println("No instructions generated")
		return nil
	}

	// Combine and send to agent
	return s.sendInstructionsToAgent(allInstructions, req)
}

func (s *AIGenerationStrategy) verifyAgentAvailability(agentType agents.AgentType) error {
	availableAgents := s.agentMgr.DetectAvailableAgents()
	for _, agent := range availableAgents {
		if agent.Type == agentType {
			return nil
		}
	}
	return fmt.Errorf("requested agent %s is not available", agentType)
}

func (s *AIGenerationStrategy) generateInstructionsForLanguages(languageOpportunities map[string][]Opportunity, req GenerationRequest) ([]string, error) {
	var allInstructions []string

	for language, langOpportunities := range languageOpportunities {
		// Collect all instrumentations for this language
		allInstrumentations := s.collectAllInstrumentations(langOpportunities)

		// Generate comprehensive instructions for this language
		instructions, err := s.templateMgr.GenerateComprehensiveInstructions(
			language,
			req.Method,
			allInstrumentations,
			filepath.Base(req.CodebasePath),
		)
		if err != nil {
			fmt.Printf("Warning: failed to generate comprehensive instructions for %s: %v\n", language, err)
			continue
		}

		fmt.Printf("Generated comprehensive instrumentation instructions for %s\n", language)
		allInstructions = append(allInstructions, instructions)
	}

	return allInstructions, nil
}

func (s *AIGenerationStrategy) sendInstructionsToAgent(allInstructions []string, req GenerationRequest) error {
	// Combine all language instructions into a single comprehensive guide
	combinedInstructions := s.combineInstructions(allInstructions, req.CodebasePath)

	fmt.Printf("Generated comprehensive instrumentation guide\n")

	// Determine the primary language or use "multi-language" if multiple
	language := req.Language
	if language == "" {
		language = "multi-language" // Default for comprehensive guides
	}

	// Create agent execution request
	agentRequest := agents.AgentExecutionRequest{
		Language:     language,
		Instructions: combinedInstructions,
		TargetDir:    req.CodebasePath,
		ServiceName:  filepath.Base(req.CodebasePath), // Use directory name as default service name
	}

	// Execute with selected agent - single call with comprehensive instructions
	if err := s.agentMgr.ExecuteWithAgent(agents.AgentType(req.Config.AgentType), agentRequest); err != nil {
		return fmt.Errorf("failed to execute with agent %s: %v", req.Config.AgentType, err)
	}

	fmt.Printf("Successfully sent comprehensive instrumentation guide to %s agent\n", req.Config.AgentType)
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
func (s *AIGenerationStrategy) groupOpportunitiesByLanguage(opportunities []Opportunity) map[string][]Opportunity {
	grouped := make(map[string][]Opportunity)

	for _, opp := range opportunities {
		if opp.Language != "" {
			grouped[opp.Language] = append(grouped[opp.Language], opp)
		}
	}

	return grouped
}

// collectAllInstrumentations extracts unique instrumentations from all opportunities
func (s *AIGenerationStrategy) collectAllInstrumentations(opportunities []Opportunity) []string {
	var instrumentations []string
	for _, opp := range opportunities {
		if opp.ComponentType == ComponentTypeInstrumentation {
			instrumentations = append(instrumentations, string(opp.Component))
		}
	}
	return instrumentations
}

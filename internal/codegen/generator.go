package codegen

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/templates"
)

// Opportunity represents a code generation opportunity
type Opportunity struct {
	Language         string          `json:"language"`
	Framework        string          `json:"framework"`
	Instrumentations []string        `json:"instrumentations"`
	FilePath         string          `json:"file_path"`
	Suggestion       string          `json:"suggestion"`
	Issue            *detector.Issue `json:"issue,omitempty"`
}

// GenerationRequest contains parameters for code generation
type GenerationRequest struct {
	CodebasePath string                       `json:"codebase_path"`
	Language     string                       `json:"language,omitempty"`
	Method       templates.InstallationMethod `json:"method"`
	AgentType    agents.AgentType             `json:"agent_type"`
}

// Generator extends the detector system for code generation
type Generator struct {
	detector    *detector.Manager
	templateMgr *templates.Manager
	agentMgr    *agents.Detector
}

// NewGenerator creates a new code generator
func NewGenerator(detectorMgr *detector.Manager) (*Generator, error) {
	templateMgr, err := templates.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template manager: %w", err)
	}

	agentMgr, err := agents.NewDetector()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent manager: %w", err)
	}

	return &Generator{
		detector:    detectorMgr,
		templateMgr: templateMgr,
		agentMgr:    agentMgr,
	}, nil
}

// GenerateInstrumentation analyzes and generates code
func (g *Generator) GenerateInstrumentation(ctx context.Context, req GenerationRequest) error {
	// Use existing detector for analysis
	analysis, issues, err := g.detector.AnalyzeCodebase(ctx, req.CodebasePath)
	if err != nil {
		return fmt.Errorf("codebase analysis failed: %w", err)
	}

	// Convert issues to opportunities
	opportunities := g.convertIssuesToOpportunities(analysis, issues)

	// Filter by language if specified
	if req.Language != "" {
		opportunities = g.filterByLanguage(opportunities, req.Language)
	}

	if len(opportunities) == 0 {
		fmt.Println("No code generation opportunities found")
		return nil
	}

	// Generate and execute for each opportunity
	return g.executeOpportunities(ctx, opportunities, req)
}

// ListAvailableAgents returns all detected coding agents
func (g *Generator) ListAvailableAgents() []agents.Agent {
	return g.agentMgr.DetectAvailableAgents()
}

// ListAvailableTemplates returns all available templates
func (g *Generator) ListAvailableTemplates() []string {
	return g.templateMgr.GetAvailableTemplates()
}

func (g *Generator) convertIssuesToOpportunities(analysis *detector.Analysis, issues []detector.Issue) []Opportunity {
	var opportunities []Opportunity

	for _, issue := range issues {
		// Convert specific issue types to opportunities
		switch issue.Category {
		case detector.CategoryMissingLibrary, detector.CategoryInstrumentation:
			opp := Opportunity{
				Language:   issue.Language,
				FilePath:   issue.File,
				Suggestion: issue.Suggestion,
				Issue:      &issue,
			}

			// Extract instrumentations from available instrumentations
			opp.Instrumentations = g.extractRelevantInstrumentations(analysis, issue.Language)

			opportunities = append(opportunities, opp)
		}
	}

	// Also create opportunities from available instrumentations
	opportunities = append(opportunities, g.createOpportunitiesFromInstrumentations(analysis)...)

	return opportunities
}

func (g *Generator) createOpportunitiesFromInstrumentations(analysis *detector.Analysis) []Opportunity {
	var opportunities []Opportunity

	for _, instr := range analysis.AvailableInstrumentations {
		if instr.IsAvailable && !g.isAlreadyInstrumented(analysis, instr) {
			opp := Opportunity{
				Language:         instr.Language,
				Framework:        instr.Package.Name,
				Instrumentations: []string{instr.Package.Name},
				Suggestion:       fmt.Sprintf("Add OpenTelemetry instrumentation for %s", instr.Package.Name),
			}
			opportunities = append(opportunities, opp)
		}
	}

	return opportunities
}

func (g *Generator) isAlreadyInstrumented(analysis *detector.Analysis, instr detector.InstrumentationInfo) bool {
	// Check if the instrumentation library is already in use
	for _, lib := range analysis.Libraries {
		if lib.Name == instr.Package.Name ||
			lib.ImportPath == instr.Package.ImportPath {
			return true
		}
	}
	return false
}

func (g *Generator) extractRelevantInstrumentations(analysis *detector.Analysis, language string) []string {
	var instrumentations []string

	for _, instr := range analysis.AvailableInstrumentations {
		if instr.Language == language && instr.IsAvailable {
			instrumentations = append(instrumentations, instr.Package.Name)
		}
	}

	return instrumentations
}

func (g *Generator) filterByLanguage(opportunities []Opportunity, language string) []Opportunity {
	var filtered []Opportunity

	for _, opp := range opportunities {
		if opp.Language == language {
			filtered = append(filtered, opp)
		}
	}

	return filtered
}

func (g *Generator) executeOpportunities(ctx context.Context, opportunities []Opportunity, req GenerationRequest) error {
	// Verify requested agent is available
	if err := g.verifyAgentAvailability(req.AgentType); err != nil {
		return err
	}

	// Group opportunities by language to create comprehensive instructions
	languageOpportunities := g.groupOpportunitiesByLanguage(opportunities)

	if len(languageOpportunities) == 0 {
		fmt.Println("No opportunities to process")
		return nil
	}

	// Generate comprehensive instructions
	allInstructions, err := g.generateInstructionsForLanguages(languageOpportunities, req)
	if err != nil {
		return err
	}

	if len(allInstructions) == 0 {
		fmt.Println("No instructions generated")
		return nil
	}

	// Combine and send to agent
	return g.sendInstructionsToAgent(allInstructions, req)
}

func (g *Generator) verifyAgentAvailability(agentType agents.AgentType) error {
	availableAgents := g.agentMgr.DetectAvailableAgents()
	for _, agent := range availableAgents {
		if agent.Type == agentType {
			return nil
		}
	}
	return fmt.Errorf("requested agent %s is not available", agentType)
}

func (g *Generator) generateInstructionsForLanguages(languageOpportunities map[string][]Opportunity, req GenerationRequest) ([]string, error) {
	var allInstructions []string

	for language, langOpportunities := range languageOpportunities {
		// Collect all instrumentations for this language
		allInstrumentations := g.collectAllInstrumentations(langOpportunities)

		// Generate comprehensive instructions for this language
		instructions, err := g.templateMgr.GenerateComprehensiveInstructions(
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

func (g *Generator) sendInstructionsToAgent(allInstructions []string, req GenerationRequest) error {
	// Combine all language instructions into a single comprehensive guide
	combinedInstructions := g.combineInstructions(allInstructions, req.CodebasePath)

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
	if err := g.agentMgr.ExecuteWithAgent(req.AgentType, agentRequest); err != nil {
		return fmt.Errorf("failed to execute with agent %s: %v", req.AgentType, err)
	}

	fmt.Printf("Successfully sent comprehensive instrumentation guide to %s agent\n", req.AgentType)
	return nil
}

func (g *Generator) combineInstructions(allInstructions []string, codebasePath string) string {
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
func (g *Generator) groupOpportunitiesByLanguage(opportunities []Opportunity) map[string][]Opportunity {
	grouped := make(map[string][]Opportunity)

	for _, opp := range opportunities {
		if opp.Language != "" {
			grouped[opp.Language] = append(grouped[opp.Language], opp)
		}
	}

	return grouped
}

// collectAllInstrumentations extracts unique instrumentations from all opportunities
func (g *Generator) collectAllInstrumentations(opportunities []Opportunity) []string {
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

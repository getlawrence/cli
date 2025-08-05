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

	return &Generator{
		detector:    detectorMgr,
		templateMgr: templateMgr,
		agentMgr:    agents.NewDetector(),
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
	availableAgents := g.agentMgr.DetectAvailableAgents()
	agentAvailable := false
	for _, agent := range availableAgents {
		if agent.Type == req.AgentType {
			agentAvailable = true
			break
		}
	}

	if !agentAvailable {
		return fmt.Errorf("requested agent %s is not available", req.AgentType)
	}

	// Collect all instructions before sending to agent
	var allInstructions []string

	for _, opportunity := range opportunities {
		data := templates.TemplateData{
			Language:         opportunity.Language,
			Method:           req.Method,
			Instrumentations: opportunity.Instrumentations,
			ServiceName:      filepath.Base(req.CodebasePath),
		}

		instructions, err := g.templateMgr.GenerateInstructions(
			opportunity.Language,
			req.Method,
			data,
		)
		if err != nil {
			fmt.Printf("Warning: failed to generate instructions for %s: %v\n",
				opportunity.Language, err)
			continue
		}

		fmt.Printf("Generated instrumentation instructions for %s in %s\n",
			opportunity.Framework, opportunity.FilePath)

		allInstructions = append(allInstructions, instructions)
	}

	if len(allInstructions) == 0 {
		fmt.Println("No instructions generated")
		return nil
	}

	// Combine all instructions and send to agent once
	combinedInstructions := fmt.Sprintf("# OpenTelemetry Instrumentation for %s\n\nPlease implement the following instrumentation opportunities:\n\n%s",
		filepath.Base(req.CodebasePath),
		allInstructions[0]) // Start with first instruction

	// Add subsequent instructions
	for i := 1; i < len(allInstructions); i++ {
		combinedInstructions += fmt.Sprintf("\n\n---\n\n%s", allInstructions[i])
	}

	fmt.Printf("Combined Instructions:\n%s\n", combinedInstructions)

	// Execute with selected agent - single call
	if err := g.agentMgr.ExecuteWithAgent(req.AgentType, combinedInstructions, req.CodebasePath); err != nil {
		return fmt.Errorf("failed to execute with agent %s: %v", req.AgentType, err)
	}

	fmt.Printf("Successfully sent combined instructions to %s agent\n", req.AgentType)
	return nil
}

package codegen

import (
	"context"
	"fmt"

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
	AgentType    agents.AgentType             `json:"agent_type"` // Deprecated: use Config.AgentType
	Config       StrategyConfig               `json:"config"`
}

// Generator extends the detector system for code generation
type Generator struct {
	detector        *detector.Manager
	templateMgr     *templates.Manager
	agentMgr        *agents.Detector
	strategies      map[GenerationMode]CodeGenerationStrategy
	defaultStrategy GenerationMode
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

	// Initialize strategies
	strategies := make(map[GenerationMode]CodeGenerationStrategy)
	strategies[AIMode] = NewAIGenerationStrategy(agentMgr, templateMgr)
	strategies[TemplateMode] = NewTemplateGenerationStrategy(templateMgr)

	// Default to AI mode if agents are available, otherwise template mode
	defaultStrategy := TemplateMode
	if strategies[AIMode].IsAvailable() {
		defaultStrategy = AIMode
	}

	return &Generator{
		detector:        detectorMgr,
		templateMgr:     templateMgr,
		agentMgr:        agentMgr,
		strategies:      strategies,
		defaultStrategy: defaultStrategy,
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

	// Select generation mode
	mode := req.Config.Mode
	if mode == "" {
		mode = g.defaultStrategy
	}

	// Get the appropriate strategy
	strategy, exists := g.strategies[mode]
	if !exists {
		return fmt.Errorf("unsupported generation mode: %s", mode)
	}

	// Check if strategy is available
	if !strategy.IsAvailable() {
		return fmt.Errorf("generation mode %s is not available on this system", mode)
	}

	// Validate required flags for the strategy
	if err := g.validateStrategyRequirements(strategy, req); err != nil {
		return err
	}

	fmt.Printf("Using %s generation strategy\n", strategy.GetName())

	// Execute generation using the selected strategy
	return strategy.GenerateCode(ctx, opportunities, req)
}

// ListAvailableAgents returns all detected coding agents
func (g *Generator) ListAvailableAgents() []agents.Agent {
	return g.agentMgr.DetectAvailableAgents()
}

// ListAvailableTemplates returns all available templates
func (g *Generator) ListAvailableTemplates() []string {
	return g.templateMgr.GetAvailableTemplates()
}

// ListAvailableStrategies returns all available generation strategies
func (g *Generator) ListAvailableStrategies() map[GenerationMode]bool {
	strategies := make(map[GenerationMode]bool)
	for mode, strategy := range g.strategies {
		strategies[mode] = strategy.IsAvailable()
	}
	return strategies
}

// GetDefaultStrategy returns the default generation strategy
func (g *Generator) GetDefaultStrategy() GenerationMode {
	return g.defaultStrategy
}

// validateStrategyRequirements checks if all required flags are provided for a strategy
func (g *Generator) validateStrategyRequirements(strategy CodeGenerationStrategy, req GenerationRequest) error {
	requiredFlags := strategy.GetRequiredFlags()

	for _, flag := range requiredFlags {
		switch flag {
		case "agent":
			if req.Config.AgentType == "" {
				// Fallback to deprecated field
				if req.AgentType == "" {
					return fmt.Errorf("agent type is required for AI generation. Use --list-agents to see available options")
				}
				// Copy from deprecated field
				req.Config.AgentType = string(req.AgentType)
			}
		}
	}

	return nil
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

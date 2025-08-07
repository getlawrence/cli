package codegen

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/types"
	"github.com/getlawrence/cli/internal/templates"
)

type OpportunityType string

const (
	OpportunityInstallOTEL      OpportunityType = "install_otel"
	OpportunityInstallComponent OpportunityType = "install_component"
	OpportunityRemoveComponent  OpportunityType = "remove_component"
)

type ComponentType string

const (
	ComponentTypeInstrumentation ComponentType = "instrumentation"
	ComponentTypeSDK             ComponentType = "sdk"
	ComponentTypePropagator      ComponentType = "propagator"
	ComponentTypeExporter        ComponentType = "exporter"
)

type Opportunity struct {
	Type          OpportunityType `json:"type"`
	Language      string          `json:"language"`
	Framework     string          `json:"framework"`
	ComponentType ComponentType   `json:"componentType"`
	Component     string          `json:"component"`
	FilePath      string          `json:"file_path"`
	Suggestion    string          `json:"suggestion"`
	Issue         *types.Issue    `json:"issue,omitempty"`
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
	detector        *detector.CodebaseAnalyzer
	templateEngine  *templates.TemplateEngine
	agentDetector   *agents.Detector
	strategies      map[GenerationMode]CodeGenerationStrategy
	defaultStrategy GenerationMode
}

// NewGenerator creates a new code generator
func NewGenerator(codebaseAnalyzer *detector.CodebaseAnalyzer) (*Generator, error) {
	templateEngine, err := templates.NewTemplateEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize template engine: %w", err)
	}

	agentDetector, err := agents.NewDetector()
	if err != nil {
		return nil, fmt.Errorf("failed to initialize agent detector: %w", err)
	}

	// Initialize strategies
	strategies := make(map[GenerationMode]CodeGenerationStrategy)
	strategies[AIMode] = NewAIGenerationStrategy(agentDetector, templateEngine)
	strategies[TemplateMode] = NewTemplateGenerationStrategy(templateEngine)
	defaultStrategy := TemplateMode

	return &Generator{
		detector:        codebaseAnalyzer,
		templateEngine:  templateEngine,
		agentDetector:   agentDetector,
		strategies:      strategies,
		defaultStrategy: defaultStrategy,
	}, nil
}

// GenerateInstrumentation analyzes and generates code
func (g *Generator) Generate(ctx context.Context, req GenerationRequest) error {
	// Use existing detector for analysis
	analysis, err := g.detector.AnalyzeCodebase(ctx, req.CodebasePath)
	if err != nil {
		return fmt.Errorf("codebase analysis failed: %w", err)
	}

	// Convert issues to opportunities
	opportunities := g.convertIssuesToOpportunities(analysis)

	// Filter by language if specified
	if req.Language != "" {
		opportunities = g.filterByLanguage(opportunities, req.Language)
	}

	if len(opportunities) == 0 {
		fmt.Println("Generate: No code generation opportunities found")
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
	return g.agentDetector.DetectAvailableAgents()
}

// ListAvailableTemplates returns all available templates
func (g *Generator) ListAvailableTemplates() []string {
	return g.templateEngine.GetAvailableTemplates()
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

func (g *Generator) convertIssuesToOpportunities(analysis *detector.Analysis) []Opportunity {
	var opportunities []Opportunity

	// Extract issues from the analysis
	for _, dirAnalysis := range analysis.DirectoryAnalyses {
		opportunities = append(opportunities, g.createOpportunitiesFromInstrumentations(dirAnalysis)...)
		for _, issue := range dirAnalysis.Issues {
			switch issue.Category {
			case types.CategoryMissingOtel:
				opportunities = append(opportunities, Opportunity{
					Type:     OpportunityInstallOTEL,
					Language: issue.Language,
					FilePath: dirAnalysis.Directory,
				})
			}
		}
	}
	return opportunities
}

func (g *Generator) createOpportunitiesFromInstrumentations(analysis *detector.DirectoryAnalysis) []Opportunity {
	var opportunities []Opportunity

	for _, instr := range analysis.AvailableInstrumentations {
		if instr.IsAvailable && !g.isAlreadyInstrumented(analysis, instr) {
			opp := Opportunity{
				Language:      instr.Language,
				Framework:     instr.Package.Name,
				Component:     instr.Package.Name,
				ComponentType: ComponentTypeInstrumentation,
				Type:          OpportunityInstallComponent,
				Suggestion:    fmt.Sprintf("Add OpenTelemetry instrumentation for %s", instr.Package.Name),
				FilePath:      analysis.Directory,
			}
			opportunities = append(opportunities, opp)
		}
	}

	return opportunities
}

func (g *Generator) isAlreadyInstrumented(analysis *detector.DirectoryAnalysis, instr types.InstrumentationInfo) bool {
	// Check if the instrumentation library is already in use
	for _, lib := range analysis.Libraries {
		if lib.Name == instr.Package.Name ||
			lib.ImportPath == instr.Package.ImportPath {
			return true
		}
	}
	return false
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

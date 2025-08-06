package codegen

import (
	"context"
)

// GenerationMode represents different code generation approaches
type GenerationMode string

const (
	// AIMode uses AI agents for code generation
	AIMode GenerationMode = "ai"
	// TemplateMode uses templates for direct code generation
	TemplateMode GenerationMode = "template"
)

// CodeGenerationStrategy defines the interface for different code generation approaches
type CodeGenerationStrategy interface {
	// GenerateCode generates instrumentation code for the given opportunities
	GenerateCode(ctx context.Context, opportunities []Opportunity, req GenerationRequest) error

	// GetName returns the name of the strategy
	GetName() string

	// IsAvailable checks if this strategy can be used in the current environment
	IsAvailable() bool

	// GetRequiredFlags returns flags that are required for this strategy
	GetRequiredFlags() []string
}

// StrategyConfig contains configuration for generation strategies
type StrategyConfig struct {
	Mode GenerationMode `json:"mode"`
	// AI-specific config
	AgentType string `json:"agent_type,omitempty"`
	// Template-specific config
	OutputDirectory string `json:"output_directory,omitempty"`
	DryRun          bool   `json:"dry_run,omitempty"`
}

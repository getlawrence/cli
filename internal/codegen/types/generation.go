package types

// GenerationMode represents different code generation approaches
type GenerationMode string

const (
	// AgentMode uses AI agents for code generation
	AgentMode GenerationMode = "agent"
	// TemplateMode uses templates for direct code generation
	TemplateMode GenerationMode = "template"
)

// StrategyConfig contains configuration for generation strategies
type StrategyConfig struct {
	Mode            GenerationMode `json:"mode"`
	AgentType       string         `json:"agent_type,omitempty"`
	OutputDirectory string         `json:"output_directory,omitempty"`
	DryRun          bool           `json:"dry_run,omitempty"`
	// AI mode options
	ShowPrompt bool   `json:"show_prompt,omitempty"`
	SavePrompt string `json:"save_prompt,omitempty"`
}

// GenerationRequest contains parameters for code generation
type GenerationRequest struct {
	CodebasePath string         `json:"codebase_path"`
	Language     string         `json:"language,omitempty"`
	Method       string         `json:"method"`     // Changed from templates.InstallationMethod to string
	AgentType    string         `json:"agent_type"` // Changed from agents.AgentType to string
	Config       StrategyConfig `json:"config"`
}

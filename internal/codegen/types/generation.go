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

// OTELConfig represents advanced OpenTelemetry configuration that can be
// provided via a YAML file and applied across languages.
// Deprecated: use internal/config.OTELConfig instead. Kept temporarily to avoid massive churn.
type OTELConfig struct {
	// Service metadata
	ServiceName    string            `json:"service_name" yaml:"service_name"`
	ServiceVersion string            `json:"service_version" yaml:"service_version"`
	Environment    string            `json:"environment" yaml:"environment"`
	ResourceAttrs  map[string]string `json:"resource_attributes" yaml:"resource_attributes"`

	// Instrumentations to enable (per language these map to specific packages)
	Instrumentations []string `json:"instrumentations" yaml:"instrumentations"`

	// Context propagation
	Propagators []string `json:"propagators" yaml:"propagators"`

	// Sampling configuration
	Sampler struct {
		Type   string   `json:"type" yaml:"type"`     // e.g., always_on, always_off, traceidratio, parentbased_traceidratio
		Ratio  float64  `json:"ratio" yaml:"ratio"`   // used for ratio-based samplers
		Parent string   `json:"parent" yaml:"parent"` // for parentbased sub-type
		Rules  []string `json:"rules" yaml:"rules"`   // reserved for advanced/language-specific
	} `json:"sampler" yaml:"sampler"`

	// Exporters and endpoints
	Exporters struct {
		Traces struct {
			Type      string            `json:"type" yaml:"type"`         // e.g., otlp, console, jaeger
			Protocol  string            `json:"protocol" yaml:"protocol"` // http/protobuf, grpc
			Endpoint  string            `json:"endpoint" yaml:"endpoint"`
			Headers   map[string]string `json:"headers" yaml:"headers"`
			Insecure  bool              `json:"insecure" yaml:"insecure"`
			TimeoutMs int               `json:"timeout_ms" yaml:"timeout_ms"`
		} `json:"traces" yaml:"traces"`
		Metrics struct {
			Type     string `json:"type" yaml:"type"`
			Protocol string `json:"protocol" yaml:"protocol"`
			Endpoint string `json:"endpoint" yaml:"endpoint"`
			Insecure bool   `json:"insecure" yaml:"insecure"`
		} `json:"metrics" yaml:"metrics"`
		Logs struct {
			Type     string `json:"type" yaml:"type"`
			Protocol string `json:"protocol" yaml:"protocol"`
			Endpoint string `json:"endpoint" yaml:"endpoint"`
			Insecure bool   `json:"insecure" yaml:"insecure"`
		} `json:"logs" yaml:"logs"`
	} `json:"exporters" yaml:"exporters"`

	// Span processors and additional SDK knobs
	SpanProcessors []string          `json:"span_processors" yaml:"span_processors"`
	SDK            map[string]string `json:"sdk" yaml:"sdk"`
}

// GenerationRequest contains parameters for code generation
type GenerationRequest struct {
	CodebasePath string         `json:"codebase_path"`
	Language     string         `json:"language,omitempty"`
	Method       string         `json:"method"`     // Changed from templates.InstallationMethod to string
	AgentType    string         `json:"agent_type"` // Changed from agents.AgentType to string
	Config       StrategyConfig `json:"config"`
	// Advanced OpenTelemetry configuration loaded from YAML (optional)
	OTEL *OTELConfig `json:"otel,omitempty"`
}

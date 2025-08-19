package config

import (
	"errors"
	"fmt"
	"net/url"
	"strings"

	"gopkg.in/yaml.v3"
)

// OTELConfig represents advanced OpenTelemetry configuration that can be
// provided via a YAML file and applied across languages.
type OTELConfig struct {
	// GitHub configuration for API authentication
	GitHub struct {
		Token string `json:"token" yaml:"token"` // GitHub personal access token
	} `json:"github" yaml:"github"`

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
		Type   string   `json:"type" yaml:"type"`     // e.g., always_on, always_off, traceidratio
		Ratio  float64  `json:"ratio" yaml:"ratio"`   // used for ratio-based samplers
		Parent string   `json:"parent" yaml:"parent"` // reserved for parentbased variants
		Rules  []string `json:"rules" yaml:"rules"`   // reserved for advanced/language-specific
	} `json:"sampler" yaml:"sampler"`

	// Exporters and endpoints
	Exporters struct {
		Traces struct {
			Type      string            `json:"type" yaml:"type"`         // e.g., otlp, console, jaeger (only otlp supported in templates for now)
			Protocol  string            `json:"protocol" yaml:"protocol"` // http/http+protobuf, grpc
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

// LoadOTELConfig parses YAML content into an OTELConfig.
func LoadOTELConfig(yamlContent []byte) (*OTELConfig, error) {
	var cfg OTELConfig
	if err := yaml.Unmarshal(yamlContent, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML config: %w", err)
	}
	return &cfg, nil
}

// Validate checks the configuration for common mistakes and unsupported values.
func (c *OTELConfig) Validate() error {
	if c == nil {
		return errors.New("nil config")
	}

	var errs []string

	// Sampler
	if c.Sampler.Type != "" {
		t := strings.ToLower(c.Sampler.Type)
		switch t {
		case "always_on", "always-off", "always_off":
			// ok
		case "traceidratio":
			if c.Sampler.Ratio < 0 || c.Sampler.Ratio > 1 {
				errs = append(errs, "sampler.ratio must be between 0 and 1")
			}
		default:
			errs = append(errs, fmt.Sprintf("unsupported sampler.type: %s", c.Sampler.Type))
		}
	}

	// Propagators
	allowedProps := map[string]bool{
		"tracecontext": true, "w3c": true, "baggage": true, "b3": true, "b3multi": true,
	}
	for _, p := range c.Propagators {
		if !allowedProps[strings.ToLower(p)] {
			errs = append(errs, fmt.Sprintf("unsupported propagator: %s", p))
		}
	}

	// Exporters - traces
	if c.Exporters.Traces.Type != "" {
		t := strings.ToLower(c.Exporters.Traces.Type)
		switch t {
		case "otlp", "otlphttp", "otlpgrpc", "console", "jaeger":
			// templates currently primarily support OTLP; others may be ignored by generators
		default:
			errs = append(errs, fmt.Sprintf("unsupported exporters.traces.type: %s", c.Exporters.Traces.Type))
		}
	}
	if c.Exporters.Traces.Protocol != "" {
		proto := strings.ToLower(c.Exporters.Traces.Protocol)
		if !(strings.HasPrefix(proto, "http") || proto == "grpc") {
			errs = append(errs, fmt.Sprintf("unsupported exporters.traces.protocol: %s", c.Exporters.Traces.Protocol))
		}
	}
	if c.Exporters.Traces.Endpoint != "" {
		if _, err := url.Parse(c.Exporters.Traces.Endpoint); err != nil {
			errs = append(errs, fmt.Sprintf("invalid exporters.traces.endpoint: %v", err))
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("invalid config: %s", strings.Join(errs, "; "))
	}
	return nil
}

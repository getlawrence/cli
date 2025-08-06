package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *
var templateFS embed.FS

// InstallationMethod represents different OTEL installation approaches
type InstallationMethod string

const (
	CodeInstrumentation InstallationMethod = "code"
	AutoInstrumentation InstallationMethod = "auto"
	EBPFInstrumentation InstallationMethod = "ebpf"
)

// TemplateData contains all data needed for template generation
type TemplateData struct {
	Language         string             `json:"language"`
	Method           InstallationMethod `json:"method"`
	Instrumentations []string           `json:"instrumentations"`
	ServiceName      string             `json:"service_name"`
	Samplers         []string           `json:"samplers,omitempty"`
	ContextProps     []string           `json:"context_props,omitempty"`
	SpanProcessors   []string           `json:"span_processors,omitempty"`
}

// AgentPromptData contains all data needed for agent prompt generation
type AgentPromptData struct {
	Language               string   `json:"language"`
	Instructions           string   `json:"instructions"`
	DetectedFrameworks     []string `json:"detected_frameworks,omitempty"`
	ServiceName            string   `json:"service_name,omitempty"`
	AdditionalRequirements []string `json:"additional_requirements,omitempty"`
	TemplateContent        string   `json:"template_content,omitempty"`
}

// Manager handles template loading and execution
type Manager struct {
	templates map[string]*template.Template
}

// NewManager creates a new template manager
func NewManager() (*Manager, error) {
	m := &Manager{
		templates: make(map[string]*template.Template),
	}

	if err := m.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return m, nil
}

// GenerateAgentPrompt creates a prompt for coding agents
func (m *Manager) GenerateAgentPrompt(data AgentPromptData) (string, error) {
	tmpl, exists := m.templates["agent_prompt"]
	if !exists {
		return "", fmt.Errorf("agent prompt template not found")
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("agent prompt template execution failed: %w", err)
	}

	return buf.String(), nil
}

// GenerateInstructions creates instructions based on language and method
func (m *Manager) GenerateInstructions(lang string, method InstallationMethod, data TemplateData) (string, error) {
	// For template-based code generation, try code generation templates first
	codeGenKey := fmt.Sprintf("%s_%s_gen", lang, method)
	if tmpl, exists := m.templates[codeGenKey]; exists {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("code generation template execution failed: %w", err)
		}
		return buf.String(), nil
	}

	// First try comprehensive template
	comprehensiveKey := fmt.Sprintf("%s_comprehensive", lang)
	if tmpl, exists := m.templates[comprehensiveKey]; exists {
		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("comprehensive template execution failed: %w", err)
		}
		return buf.String(), nil
	}

	// Fallback to method-specific template
	templateKey := fmt.Sprintf("%s_%s", lang, method)
	tmpl, exists := m.templates[templateKey]
	if !exists {
		return "", fmt.Errorf("template not found for %s with method %s", lang, method)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("template execution failed: %w", err)
	}

	return buf.String(), nil
}

// GenerateComprehensiveInstructions creates a single comprehensive instruction
// that includes all instrumentations for a given language
func (m *Manager) GenerateComprehensiveInstructions(lang string, method InstallationMethod, allInstrumentations []string, serviceName string) (string, error) {
	// Use comprehensive template if available
	comprehensiveKey := fmt.Sprintf("%s_comprehensive", lang)
	if tmpl, exists := m.templates[comprehensiveKey]; exists {
		data := TemplateData{
			Language:         lang,
			Method:           method,
			Instrumentations: allInstrumentations,
			ServiceName:      serviceName,
		}

		var buf bytes.Buffer
		if err := tmpl.Execute(&buf, data); err != nil {
			return "", fmt.Errorf("comprehensive template execution failed: %w", err)
		}
		return buf.String(), nil
	}

	// Fallback: generate individual instructions and combine them
	return m.GenerateInstructions(lang, method, TemplateData{
		Language:         lang,
		Method:           method,
		Instrumentations: allInstrumentations,
		ServiceName:      serviceName,
	})
}

func (m *Manager) loadTemplates() error {
	// Load embedded templates
	entries, err := templateFS.ReadDir("templates")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != ".gitkeep" {
			templateName := entry.Name()
			templatePath := fmt.Sprintf("templates/%s", templateName)

			content, err := templateFS.ReadFile(templatePath)
			if err != nil {
				return err
			}

			// Remove .tmpl extension for key
			key := templateName
			if len(templateName) > 5 && templateName[len(templateName)-5:] == ".tmpl" {
				key = templateName[:len(templateName)-5]
			}

			tmpl, err := template.New(key).Parse(string(content))
			if err != nil {
				return err
			}

			m.templates[key] = tmpl
		}
	}

	return nil
}

// GetAvailableTemplates returns all available template keys
func (m *Manager) GetAvailableTemplates() []string {
	var keys []string
	for key := range m.templates {
		keys = append(keys, key)
	}
	return keys
}

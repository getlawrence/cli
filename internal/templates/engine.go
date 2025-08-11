package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed *
var templateFS embed.FS

// TemplateData contains all data needed for template generation
type TemplateData struct {
	Language         string   `json:"language"`
	Instrumentations []string `json:"instrumentations"`
	ServiceName      string   `json:"service_name"`
	Samplers         []string `json:"samplers,omitempty"`
	ContextProps     []string `json:"context_props,omitempty"`
	SpanProcessors   []string `json:"span_processors,omitempty"`

	// New fields for extended operations
	InstallOTEL       bool                `json:"install_otel,omitempty"`
	InstallComponents map[string][]string `json:"install_components,omitempty"`
	RemoveComponents  map[string][]string `json:"remove_components,omitempty"`
}

// AgentPromptData contains all data needed for agent prompt generation
type AgentPromptData struct {
	Language       string          `json:"language"`
	Directory      string          `json:"directory,omitempty"`
	DirectoryPlans []DirectoryPlan `json:"directory_plans,omitempty"`
}

// DirectoryPlan summarizes the tech stack and planned actions for a directory
type DirectoryPlan struct {
	Directory                string              `json:"directory"`
	Language                 string              `json:"language"`
	Libraries                []string            `json:"libraries,omitempty"`
	Packages                 []string            `json:"packages,omitempty"`
	DetectedFrameworks       []string            `json:"detected_frameworks,omitempty"`
	ExistingInstrumentations []string            `json:"existing_instrumentations,omitempty"`
	InstallOTEL              bool                `json:"install_otel,omitempty"`
	InstallInstrumentations  []string            `json:"install_instrumentations,omitempty"`
	InstallComponents        map[string][]string `json:"install_components,omitempty"`
	RemoveComponents         map[string][]string `json:"remove_components,omitempty"`
	Issues                   []string            `json:"issues,omitempty"`
}

// TemplateEngine handles template loading and execution
type TemplateEngine struct {
	templates map[string]*template.Template
}

// NewTemplateEngine creates a new template engine
func NewTemplateEngine() (*TemplateEngine, error) {
	engine := &TemplateEngine{
		templates: make(map[string]*template.Template),
	}

	if err := engine.loadTemplates(); err != nil {
		return nil, fmt.Errorf("failed to load templates: %w", err)
	}

	return engine, nil
}

// GenerateAgentPrompt creates a prompt for coding agents
func (e *TemplateEngine) GenerateAgentPrompt(data AgentPromptData) (string, error) {
	tmpl, exists := e.templates["agent_prompt"]
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
func (e *TemplateEngine) GenerateInstructions(lang string, data TemplateData) (string, error) {
	// Only use code generation templates for template-based generation
	codeGenKey := fmt.Sprintf("%s_code_gen", lang)
	tmpl, exists := e.templates[codeGenKey]
	if !exists {
		return "", fmt.Errorf("code generation template not found for %s", lang)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("code generation template execution failed: %w", err)
	}

	return buf.String(), nil
}

func (e *TemplateEngine) loadTemplates() error {
	// Load embedded templates
	entries, err := templateFS.ReadDir(".")
	if err != nil {
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() && entry.Name() != ".gitkeep" {
			templateName := entry.Name()

			content, err := templateFS.ReadFile(templateName)
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

			e.templates[key] = tmpl
		}
	}

	return nil
}

// GetAvailableTemplates returns all available template keys
func (e *TemplateEngine) GetAvailableTemplates() []string {
	var keys []string
	for key := range e.templates {
		keys = append(keys, key)
	}
	return keys
}

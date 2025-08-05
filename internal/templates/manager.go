package templates

import (
	"bytes"
	"embed"
	"fmt"
	"text/template"
)

//go:embed templates/*
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
	// Future expansion fields
	Samplers       []string `json:"samplers,omitempty"`
	ContextProps   []string `json:"context_props,omitempty"`
	SpanProcessors []string `json:"span_processors,omitempty"`
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

// GenerateInstructions creates instructions based on language and method
func (m *Manager) GenerateInstructions(lang string, method InstallationMethod, data TemplateData) (string, error) {
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

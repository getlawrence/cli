package agents

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/getlawrence/cli/internal/templates"
)

// AgentType represents different coding agent types
type AgentType string

const (
	GeminiCLI   AgentType = "gemini"
	ClaudeCode  AgentType = "claude"
	OpenAICodex AgentType = "openai"
	GitHubCLI   AgentType = "github"
)

// Agent represents a coding agent with its metadata
type Agent struct {
	Type      AgentType `json:"type"`
	Name      string    `json:"name"`
	Command   string    `json:"command"`
	Available bool      `json:"available"`
	Version   string    `json:"version"`
}

// AgentExecutionRequest contains all parameters needed for agent execution
type AgentExecutionRequest struct {
	Language               string   `json:"language"`
	Instructions           string   `json:"instructions"`
	TargetDir              string   `json:"target_dir"`
	DetectedFrameworks     []string `json:"detected_frameworks,omitempty"`
	ServiceName            string   `json:"service_name,omitempty"`
	AdditionalRequirements []string `json:"additional_requirements,omitempty"`
	TemplateContent        string   `json:"template_content,omitempty"`
}

// Detector handles detection of available coding agents
type Detector struct {
	agents          []Agent
	templateManager *templates.Manager
}

// NewDetector creates a new agent detector
func NewDetector() (*Detector, error) {
	templateManager, err := templates.NewManager()
	if err != nil {
		return nil, fmt.Errorf("failed to create template manager: %w", err)
	}

	return &Detector{
		agents: []Agent{
			{Type: GeminiCLI, Name: "Gemini CLI", Command: "gemini"},
			{Type: ClaudeCode, Name: "Claude Code", Command: "claude"},
			{Type: OpenAICodex, Name: "OpenAI Codex", Command: "openai"},
			{Type: GitHubCLI, Name: "GitHub Copilot CLI", Command: "gh"},
		},
		templateManager: templateManager,
	}, nil
}

// DetectAvailableAgents scans the system for available coding agents
func (d *Detector) DetectAvailableAgents() []Agent {
	var available []Agent

	for _, agent := range d.agents {
		if d.isAgentAvailable(agent) {
			agent.Available = true
			agent.Version = d.getAgentVersion(agent)
			available = append(available, agent)
		}
	}

	return available
}

func (d *Detector) isAgentAvailable(agent Agent) bool {
	command := strings.Split(agent.Command, " ")[0]
	_, err := exec.LookPath(command)
	return err == nil
}

func (d *Detector) getAgentVersion(agent Agent) string {
	command := strings.Split(agent.Command, " ")[0]
	cmd := exec.Command(command, "--version")
	output, err := cmd.Output()
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(output))
}

// ExecuteWithAgent sends instructions to the specified agent
func (d *Detector) ExecuteWithAgent(agentType AgentType, request AgentExecutionRequest) error {
	for _, agent := range d.agents {
		if agent.Type == agentType && d.isAgentAvailable(agent) {
			return d.executeCommand(agent, request)
		}
	}
	return fmt.Errorf("agent %s not available", agentType)
}

// GeneratePrompt creates a prompt using the template manager
func (d *Detector) GeneratePrompt(request AgentExecutionRequest) (string, error) {
	promptData := templates.AgentPromptData{
		Language:               request.Language,
		Instructions:           request.Instructions,
		DetectedFrameworks:     request.DetectedFrameworks,
		ServiceName:            request.ServiceName,
		AdditionalRequirements: request.AdditionalRequirements,
		TemplateContent:        request.TemplateContent,
	}

	return d.templateManager.GenerateAgentPrompt(promptData)
}

func (d *Detector) executeCommand(agent Agent, request AgentExecutionRequest) error {
	// Generate the prompt using the template
	prompt, err := d.GeneratePrompt(request)
	if err != nil {
		return fmt.Errorf("failed to generate prompt: %w", err)
	}

	// Implementation depends on each agent's API
	switch agent.Type {
	case GitHubCLI:
		return d.executeGitHubCopilot(prompt, request.TargetDir)
	case GeminiCLI:
		return d.executeGemini(prompt, request.TargetDir)
	case ClaudeCode:
		return d.executeClaude(prompt, request.TargetDir)
	case OpenAICodex:
		return d.executeOpenAI(prompt, request.TargetDir)
	default:
		return fmt.Errorf("agent execution not implemented for %s", agent.Type)
	}
}

func (d *Detector) executeGitHubCopilot(prompt string, targetDir string) error {
	cmd := exec.Command("gh", "copilot", "suggest", prompt)
	cmd.Dir = targetDir
	cmd.Stdout = nil // Let output go to terminal
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeGemini(prompt string, targetDir string) error {
	// Use --yolo flag to automatically accept file modifications
	// Use --all-files to ensure Gemini can see all files in the directory
	cmd := exec.Command("gemini", "--prompt", prompt, "--yolo", "--all-files")
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeClaude(prompt string, targetDir string) error {
	cmd := exec.Command("claude", "code", prompt)
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeOpenAI(prompt string, targetDir string) error {
	cmd := exec.Command("openai", "api", "completions.create", "-p", prompt)
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

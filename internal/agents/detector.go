package agents

import (
	"fmt"
	"os"
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
	Language       string                    `json:"language"`
	TargetDir      string                    `json:"target_dir"`
	Directory      string                    `json:"directory,omitempty"`
	DirectoryPlans []templates.DirectoryPlan `json:"directory_plans,omitempty"`
}

// Detector handles detection of available coding agents
type Detector struct {
	agents         []Agent
	templateEngine *templates.TemplateEngine
}

// NewDetector creates a new agent detector
func NewDetector() (*Detector, error) {
	templateEngine, err := templates.NewTemplateEngine()
	if err != nil {
		return nil, fmt.Errorf("failed to create template engine: %w", err)
	}

	return &Detector{
		agents: []Agent{
			{Type: GeminiCLI, Name: "Gemini CLI", Command: "gemini"},
			{Type: ClaudeCode, Name: "Claude Code", Command: "claude"},
			{Type: OpenAICodex, Name: "OpenAI Codex", Command: "codex"},
			{Type: GitHubCLI, Name: "GitHub Copilot CLI", Command: "gh copilot"},
		},
		templateEngine: templateEngine,
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

// GeneratePrompt creates a prompt using the template engine
func (d *Detector) GeneratePrompt(request AgentExecutionRequest) (string, error) {
	promptData := templates.AgentPromptData{
		Language:       request.Language,
		Directory:      request.Directory,
		DirectoryPlans: request.DirectoryPlans,
	}

	return d.templateEngine.GenerateAgentPrompt(promptData)
}

func (d *Detector) executeCommand(agent Agent, request AgentExecutionRequest) error {
	// Generate the prompt using the template
	prompt, err := d.GeneratePrompt(request)
	if err != nil {
		return fmt.Errorf("failed to generate prompt: %w", err)
	}

	var cmd *exec.Cmd

	switch agent.Type {
	case GitHubCLI:
		cmd = d.getGitHubCopilotCommand(prompt)
	case GeminiCLI:
		cmd = d.getGeminiCommand(prompt)
	case ClaudeCode:
		cmd = d.getClaudeCommand(prompt)
	case OpenAICodex:
		cmd = d.getOpenAICommand(prompt)
	default:
		return fmt.Errorf("agent execution not implemented for %s", agent.Type)
	}

	cmd.Dir = request.TargetDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (d *Detector) getGitHubCopilotCommand(prompt string) *exec.Cmd {
	return exec.Command("gh", "copilot", "suggest", prompt)
}

func (d *Detector) getGeminiCommand(prompt string) *exec.Cmd {
	return exec.Command("gemini", "--prompt", prompt, "--yolo", "--all-files")

}

func (d *Detector) getClaudeCommand(prompt string) *exec.Cmd {
	return exec.Command("claude", "-p", prompt)
}

func (d *Detector) getOpenAICommand(prompt string) *exec.Cmd {
	return exec.Command("codex", prompt)
}

package agents

import (
	"fmt"
	"os/exec"
	"strings"
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

// Detector handles detection of available coding agents
type Detector struct {
	agents []Agent
}

// NewDetector creates a new agent detector
func NewDetector() *Detector {
	return &Detector{
		agents: []Agent{
			{Type: GeminiCLI, Name: "Gemini CLI", Command: "gemini"},
			{Type: ClaudeCode, Name: "Claude Code", Command: "claude"},
			{Type: OpenAICodex, Name: "OpenAI Codex", Command: "openai"},
			{Type: GitHubCLI, Name: "GitHub Copilot CLI", Command: "gh"},
		},
	}
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
func (d *Detector) ExecuteWithAgent(agentType AgentType, instructions string, targetDir string) error {
	for _, agent := range d.agents {
		if agent.Type == agentType && d.isAgentAvailable(agent) {
			return d.executeCommand(agent, instructions, targetDir)
		}
	}
	return fmt.Errorf("agent %s not available", agentType)
}

func (d *Detector) executeCommand(agent Agent, instructions string, targetDir string) error {
	// Implementation depends on each agent's API
	switch agent.Type {
	case GitHubCLI:
		return d.executeGitHubCopilot(instructions, targetDir)
	case GeminiCLI:
		return d.executeGemini(instructions, targetDir)
	case ClaudeCode:
		return d.executeClaude(instructions, targetDir)
	case OpenAICodex:
		return d.executeOpenAI(instructions, targetDir)
	default:
		return fmt.Errorf("agent execution not implemented for %s", agent.Type)
	}
}

func (d *Detector) executeGitHubCopilot(instructions string, targetDir string) error {
	cmd := exec.Command("gh", "copilot", "suggest", instructions)
	cmd.Dir = targetDir
	cmd.Stdout = nil // Let output go to terminal
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeGemini(instructions string, targetDir string) error {
	cmd := exec.Command("gemini", "--prompt", instructions)
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeClaude(instructions string, targetDir string) error {
	cmd := exec.Command("claude", "code", instructions)
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

func (d *Detector) executeOpenAI(instructions string, targetDir string) error {
	cmd := exec.Command("openai", "api", "completions.create", "-p", instructions)
	cmd.Dir = targetDir
	cmd.Stdout = nil
	cmd.Stderr = nil
	return cmd.Run()
}

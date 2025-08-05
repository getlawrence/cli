package cmd

import (
	"context"
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/agents"
	"github.com/getlawrence/cli/internal/codegen"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/templates"
	"github.com/spf13/cobra"
)

var codegenCmd = &cobra.Command{
	Use:   "codegen",
	Short: "Generate OTEL instrumentation code using AI agents",
	Long: `Analyze your codebase and generate OpenTelemetry instrumentation 
using available coding agents like GitHub Copilot, Gemini CLI, or Claude Code.

This command will:
1. Detect available coding agents on your system
2. Analyze your codebase for instrumentation opportunities  
3. Generate appropriate instructions using templates
4. Execute the instructions with your chosen AI agent`,
	RunE: runCodegen,
}

var (
	language      string
	method        string
	agentType     string
	codebasePath  string
	listAgents    bool
	listTemplates bool
)

func init() {
	rootCmd.AddCommand(codegenCmd)

	codegenCmd.Flags().StringVarP(&language, "language", "l", "",
		"Target language (go, python, java, javascript)")
	codegenCmd.Flags().StringVarP(&method, "method", "m", "code",
		"Installation method (code, auto, ebpf)")
	codegenCmd.Flags().StringVarP(&agentType, "agent", "a", "",
		"Preferred coding agent (gemini, claude, openai, github)")
	codegenCmd.Flags().StringVarP(&codebasePath, "path", "p", ".",
		"Path to codebase")
	codegenCmd.Flags().BoolVar(&listAgents, "list-agents", false,
		"List available coding agents")
	codegenCmd.Flags().BoolVar(&listTemplates, "list-templates", false,
		"List available templates")
}

func runCodegen(cmd *cobra.Command, args []string) error {
	ctx := context.Background()

	// Initialize detector manager with all detectors including codegen
	detectorMgr := detector.NewManager()

	// Register language detectors
	detectorMgr.RegisterLanguage(languages.NewGoDetector())
	detectorMgr.RegisterLanguage(languages.NewPythonDetector())

	// Register the codegen detector
	detectorMgr.RegisterDetector(detector.NewCodeGenDetector())

	// Initialize generator with existing detector system
	generator, err := codegen.NewGenerator(detectorMgr)
	if err != nil {
		return fmt.Errorf("failed to initialize generator: %w", err)
	}

	// Handle list commands
	if listAgents {
		return listAvailableAgents(generator)
	}

	if listTemplates {
		return listAvailableTemplates(generator)
	}

	// Validate required parameters
	if agentType == "" {
		return fmt.Errorf("agent type is required. Use --list-agents to see available options")
	}

	// Validate method
	validMethods := []string{"code", "auto", "ebpf"}
	if !contains(validMethods, method) {
		return fmt.Errorf("invalid method %s. Valid options: %s",
			method, strings.Join(validMethods, ", "))
	}

	req := codegen.GenerationRequest{
		CodebasePath: codebasePath,
		Language:     language,
		Method:       templates.InstallationMethod(method),
		AgentType:    agents.AgentType(agentType),
	}

	return generator.GenerateInstrumentation(ctx, req)
}

func listAvailableAgents(generator *codegen.Generator) error {
	agents := generator.ListAvailableAgents()

	if len(agents) == 0 {
		fmt.Println("No coding agents detected on your system")
		fmt.Println("\nTo use this feature, install one of the following:")
		fmt.Println("  - GitHub CLI: gh extension install github/gh-copilot")
		fmt.Println("  - Gemini CLI: Follow instructions at https://ai.google.dev/gemini-api/docs/cli")
		fmt.Println("  - Claude Code: Follow instructions at https://docs.anthropic.com/claude/docs")
		return nil
	}

	fmt.Println("Available coding agents:")
	for _, agent := range agents {
		fmt.Printf("  %s - %s (version: %s)\n",
			agent.Type, agent.Name, agent.Version)
	}

	return nil
}

func listAvailableTemplates(generator *codegen.Generator) error {
	templates := generator.ListAvailableTemplates()

	fmt.Println("Available templates:")
	for _, template := range templates {
		fmt.Printf("  %s\n", template)
	}

	return nil
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

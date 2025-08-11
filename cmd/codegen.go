package cmd

import (
	"fmt"
	"path/filepath"

	"github.com/getlawrence/cli/internal/codegen/generator"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/ui"
	"github.com/spf13/cobra"
)

var codegenCmd = &cobra.Command{
	Use:   "codegen",
	Short: "Generate OTEL instrumentation code using AI agents or templates",
	Long: `Analyze your codebase and generate OpenTelemetry instrumentation 
using AI agents or template-based code generation.

AI Mode (default if agents available):
- Uses coding agents like GitHub Copilot, Gemini CLI, or Claude Code
- Generates instructions and lets AI implement the code
- Requires an available coding agent

Template Mode:
- Directly generates instrumentation code using templates
- Creates ready-to-use code files based on detected opportunities
- Works without external dependencies
- Supports --dry-run to preview generated code

This command will:
1. Detect available coding agents and generation strategies
2. Analyze your codebase for instrumentation opportunities  
3. Generate code using your chosen strategy (AI or template-based)`,
	RunE: runCodegen,
}

var (
	language       string
	agentType      string
	listAgents     bool
	listTemplates  bool
	listStrategies bool
	generationMode string
	outputDir      string
	dryRun         bool
	showPrompt     bool
	savePrompt     string
)

func init() {
	rootCmd.AddCommand(codegenCmd)

	codegenCmd.Flags().StringVarP(&language, "language", "l", "",
		"Target language (go, javascript, python, java, dotnet, ruby, php)")
	codegenCmd.Flags().StringVarP(&agentType, "agent", "a", "",
		"Preferred coding agent (gemini, claude, openai, github)")
	codegenCmd.Flags().BoolVar(&listAgents, "list-agents", false,
		"List available coding agents")
	codegenCmd.Flags().BoolVar(&listTemplates, "list-templates", false,
		"List available templates")
	codegenCmd.Flags().BoolVar(&listStrategies, "list-strategies", false,
		"List available generation strategies")
	codegenCmd.Flags().StringVarP(&generationMode, "mode", "", "",
		"Generation mode (ai, template). Defaults to ai if agents available, otherwise template")
	codegenCmd.Flags().StringVarP(&outputDir, "output", "o", "",
		"Output directory for generated files (template mode only)")
	codegenCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be generated without writing files (template mode only)")
	// AI mode flags
	codegenCmd.Flags().BoolVar(&showPrompt, "show-prompt", false,
		"Print the generated agent prompt before execution (AI mode only)")
	codegenCmd.Flags().StringVar(&savePrompt, "save-prompt", "",
		"Save the generated agent prompt to the given file path (AI mode only)")
}

func runCodegen(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Create analysis engine
	codebaseAnalyzer := detector.NewCodebaseAnalyzer([]detector.IssueDetector{
		issues.NewMissingOTelDetector(),
	}, map[string]detector.Language{
		"go":         languages.NewGoDetector(),
		"javascript": languages.NewJavaScriptDetector(),
		"python":     languages.NewPythonDetector(),
		"java":       languages.NewJavaDetector(),
		"csharp":     languages.NewDotNetDetector(),
		"ruby":       languages.NewRubyDetector(),
		"php":        languages.NewPHPDetector(),
	})

	// Initialize generator with existing detector system
	codeGenerator, err := generator.NewGenerator(codebaseAnalyzer)
	if err != nil {
		return fmt.Errorf("failed to initialize generator: %w", err)
	}

	// Handle list commands
	if listAgents {
		return listAvailableAgents(codeGenerator)
	}

	if listTemplates {
		return listAvailableTemplates(codeGenerator)
	}

	if listStrategies {
		return listAvailableStrategies(codeGenerator)
	}

	// Determine generation mode
	mode := types.GenerationMode(generationMode)
	if mode == "" {
		mode = codeGenerator.GetDefaultStrategy()
	}

	// Validate mode
	if mode != types.AgentMode && mode != types.TemplateMode {
		return fmt.Errorf("invalid generation mode %s. Valid options: ai, template", mode)
	}

	// Validate mode-specific requirements
	if mode == types.AgentMode && agentType == "" {
		return fmt.Errorf("agent type is required for AI mode. Use --list-agents to see available options")
	}

	req := types.GenerationRequest{
		CodebasePath: absPath,
		Language:     language,
		AgentType:    agentType,
		Config: types.StrategyConfig{
			Mode:            mode,
			AgentType:       agentType,
			OutputDirectory: outputDir,
			DryRun:          dryRun,
			ShowPrompt:      showPrompt,
			SavePrompt:      savePrompt,
		},
	}

	// Always show spinner for generation
	return ui.RunSpinner(ctx, "Generating code...", func() error {
		return codeGenerator.Generate(ctx, req)
	})
}

func listAvailableAgents(generator *generator.Generator) error {
	agents := generator.ListAvailableAgents()

	if len(agents) == 0 {
		ui.Log("No coding agents detected on your system")
		ui.Log("\nTo use this feature, install one of the following:")
		ui.Log("  - GitHub CLI: gh extension install github/gh-copilot")
		ui.Log("  - Gemini CLI: Follow instructions at https://ai.google.dev/gemini-api/docs/cli")
		ui.Log("  - Claude Code: Follow instructions at https://docs.anthropic.com/claude/docs")
		return nil
	}

	ui.Log("Available coding agents:")
	for _, agent := range agents {
		ui.Logf("  %s - %s (version: %s)\n",
			agent.Type, agent.Name, agent.Version)
	}

	return nil
}

func listAvailableTemplates(generator *generator.Generator) error {
	templates := generator.ListAvailableTemplates()

	ui.Log("Available templates:")
	for _, template := range templates {
		ui.Logf("  %s\n", template)
	}

	return nil
}

func listAvailableStrategies(generator *generator.Generator) error {
	strategies := generator.ListAvailableStrategies()

	ui.Log("Available generation strategies:")
	for mode, available := range strategies {
		status := "available"
		if !available {
			status = "not available"
		}
		ui.Logf("  %s - %s\n", mode, status)
	}

	ui.Logf("\nDefault strategy: %s\n", generator.GetDefaultStrategy())

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

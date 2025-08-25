package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/codegen/generator"
	"github.com/getlawrence/cli/internal/codegen/types"
	internalconfig "github.com/getlawrence/cli/internal/config"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/spf13/cobra"
)

var genCmd = &cobra.Command{
	Use:   "gen",
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
	RunE: runGen,
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
	configPath     string
)

func init() {
	rootCmd.AddCommand(genCmd)

	genCmd.Flags().StringVarP(&language, "language", "l", "",
		"Target language (go, javascript, python, java, dotnet, ruby, php)")
	genCmd.Flags().StringVarP(&agentType, "agent", "a", "",
		"Preferred coding agent (gemini, claude, openai, github)")
	genCmd.Flags().BoolVar(&listAgents, "list-agents", false,
		"List available coding agents")
	genCmd.Flags().BoolVar(&listTemplates, "list-templates", false,
		"List available templates")
	genCmd.Flags().BoolVar(&listStrategies, "list-strategies", false,
		"List available generation strategies")
	genCmd.Flags().StringVarP(&generationMode, "mode", "", "",
		"Generation mode (agent, template). Defaults to agent if agents available, otherwise template")
	genCmd.Flags().StringVarP(&outputDir, "output", "o", "",
		"Output directory for generated files (template mode only)")
	genCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be generated without writing files (template mode only)")
	// AI mode flags
	genCmd.Flags().BoolVar(&showPrompt, "show-prompt", false,
		"Print the generated agent prompt before execution (AI mode only)")
	genCmd.Flags().StringVar(&savePrompt, "save-prompt", "",
		"Save the generated agent prompt to the given file path (AI mode only)")
	// Advanced config
	genCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to advanced OpenTelemetry config YAML")
}

func runGen(cmd *cobra.Command, args []string) error {
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

	ui := logger.NewUILogger()

	// Create storage client for knowledge base
	config := cmd.Context().Value(ConfigKey).(*AppConfig)
	storageClient, err := storage.NewStorageWithEmbedded("knowledge.db", config.EmbeddedDB, ui)
	if err != nil {
		return fmt.Errorf("failed to create knowledge storage: %w", err)
	}
	defer storageClient.Close()

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
	}, storageClient, ui)

	kb := knowledge.NewKnowledge(*storageClient, ui)
	codeGenerator, err := generator.NewGenerator(codebaseAnalyzer, ui, kb)
	if err != nil {
		return err
	}

	// Handle list commands
	if listAgents {
		return listAvailableAgents(codeGenerator, ui)
	}

	if listTemplates {
		return listAvailableTemplates(codeGenerator, ui)
	}

	if listStrategies {
		return listAvailableStrategies(codeGenerator, ui)
	}

	// If we're using the standard generator, continue with the original logic
	mode := types.GenerationMode(generationMode)
	if mode == "" {
		mode = codeGenerator.GetDefaultStrategy()
	}

	// Validate mode
	if mode != types.AgentMode && mode != types.TemplateMode {
		return fmt.Errorf("invalid generation mode %s. Valid options: agent, template", mode)
	}

	// Validate mode-specific requirements
	if mode == types.AgentMode && agentType == "" {
		return fmt.Errorf("agent type is required for agent mode. Use --list-agents to see available options")
	}

	// Optionally load advanced OTEL config from YAML
	var otelCfg *types.OTELConfig
	if configPath != "" {
		content, err := os.ReadFile(configPath)
		if err != nil {
			return fmt.Errorf("failed to read config file: %w", err)
		}
		parsed, err := internalconfig.LoadOTELConfig(content)
		if err != nil {
			return err
		}
		if err := parsed.Validate(); err != nil {
			return err
		}
		// Map to types.OTELConfig for now (keeps request stable)
		converted := &types.OTELConfig{
			ServiceName:      parsed.ServiceName,
			ServiceVersion:   parsed.ServiceVersion,
			Environment:      parsed.Environment,
			ResourceAttrs:    parsed.ResourceAttrs,
			Instrumentations: parsed.Instrumentations,
			Propagators:      parsed.Propagators,
			SpanProcessors:   parsed.SpanProcessors,
			SDK:              parsed.SDK,
		}
		converted.Sampler.Type = parsed.Sampler.Type
		converted.Sampler.Ratio = parsed.Sampler.Ratio
		converted.Sampler.Parent = parsed.Sampler.Parent
		converted.Sampler.Rules = parsed.Sampler.Rules
		// exporters
		converted.Exporters.Traces.Type = parsed.Exporters.Traces.Type
		converted.Exporters.Traces.Protocol = parsed.Exporters.Traces.Protocol
		converted.Exporters.Traces.Endpoint = parsed.Exporters.Traces.Endpoint
		converted.Exporters.Traces.Headers = parsed.Exporters.Traces.Headers
		converted.Exporters.Traces.Insecure = parsed.Exporters.Traces.Insecure
		converted.Exporters.Traces.TimeoutMs = parsed.Exporters.Traces.TimeoutMs
		converted.Exporters.Metrics.Type = parsed.Exporters.Metrics.Type
		converted.Exporters.Metrics.Protocol = parsed.Exporters.Metrics.Protocol
		converted.Exporters.Metrics.Endpoint = parsed.Exporters.Metrics.Endpoint
		converted.Exporters.Metrics.Insecure = parsed.Exporters.Metrics.Insecure
		converted.Exporters.Logs.Type = parsed.Exporters.Logs.Type
		converted.Exporters.Logs.Protocol = parsed.Exporters.Logs.Protocol
		converted.Exporters.Logs.Endpoint = parsed.Exporters.Logs.Endpoint
		converted.Exporters.Logs.Insecure = parsed.Exporters.Logs.Insecure
		otelCfg = converted
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
		OTEL: otelCfg,
	}

	err = codeGenerator.Generate(ctx, req)
	if err != nil {
		return err
	}
	return nil
}

func listAvailableAgents(generator *generator.Generator, logger logger.Logger) error {
	agents := generator.ListAvailableAgents()

	if len(agents) == 0 {
		logger.Log("No coding agents detected on your system")
		logger.Log("\nTo use this feature, install one of the following:")
		logger.Log("  - GitHub CLI: gh extension install github/gh-copilot")
		logger.Log("  - Gemini CLI: Follow instructions at https://ai.google.dev/gemini-api/docs/cli")
		logger.Log("  - Claude Code: Follow instructions at https://docs.anthropic.com/claude/docs")
		return nil
	}

	logger.Log("Available coding agents:")
	for _, agent := range agents {
		logger.Logf("  %s - %s (version: %s)\n",
			agent.Type, agent.Name, agent.Version)
	}

	return nil
}

func listAvailableTemplates(generator *generator.Generator, logger logger.Logger) error {
	templates := generator.ListAvailableTemplates()

	logger.Log("Available templates:")
	for _, template := range templates {
		logger.Logf("  %s\n", template)
	}

	return nil
}

func listAvailableStrategies(generator *generator.Generator, logger logger.Logger) error {
	strategies := generator.ListAvailableStrategies()

	logger.Log("Available generation strategies:")
	for mode, available := range strategies {
		status := "available"
		if !available {
			status = "not available"
		}
		logger.Logf("  %s - %s\n", mode, status)
	}

	logger.Logf("\nDefault strategy: %s\n", generator.GetDefaultStrategy())

	return nil
}

package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/codegen/generator"
	"github.com/getlawrence/cli/internal/codegen/types"
	cfg "github.com/getlawrence/cli/internal/config"
	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/logger"
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
	configPath     string
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
		"Generation mode (agent, template). Defaults to agent if agents available, otherwise template")
	codegenCmd.Flags().StringVarP(&outputDir, "output", "o", "",
		"Output directory for generated files (template mode only)")
	codegenCmd.Flags().BoolVar(&dryRun, "dry-run", false,
		"Show what would be generated without writing files (template mode only)")
	// AI mode flags
	codegenCmd.Flags().BoolVar(&showPrompt, "show-prompt", false,
		"Print the generated agent prompt before execution (AI mode only)")
	codegenCmd.Flags().StringVar(&savePrompt, "save-prompt", "",
		"Save the generated agent prompt to the given file path (AI mode only)")
	// Advanced config
	codegenCmd.Flags().StringVarP(&configPath, "config", "c", "", "Path to advanced OpenTelemetry config YAML")
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

	ui := logger.NewUILogger()

	// Create analysis engine
	codebaseAnalyzer := detector.NewCodebaseAnalyzer([]detector.IssueDetector{
		issues.NewMissingOTelDetector(),
		// Add knowledge-based detector if available
		func() detector.IssueDetector {
			if detector, err := issues.NewKnowledgeBasedDetector(); err == nil {
				return detector
			}
			return nil
		}(),
	}, map[string]detector.Language{
		"go":         languages.NewGoDetector(),
		"javascript": languages.NewJavaScriptDetector(),
		"python":     languages.NewPythonDetector(),
		"java":       languages.NewJavaDetector(),
		"csharp":     languages.NewDotNetDetector(),
		"ruby":       languages.NewRubyDetector(),
		"php":        languages.NewPHPDetector(),
	}, ui)

	// Initialize generator with existing detector system
	codeGenerator, err := generator.NewGenerator(codebaseAnalyzer, ui)
	if err != nil {
		return fmt.Errorf("failed to initialize generator: %w", err)
	}

	// Try to create knowledge-enhanced generator if available
	enhancedGenerator, err := generator.NewKnowledgeEnhancedGenerator(codeGenerator)
	if err == nil {
		defer enhancedGenerator.Close()
		// Use enhanced generator for better recommendations
		ui.Logf("Using knowledge-enhanced generator for better recommendations\n")

		// Use the enhanced generator's GenerateWithKnowledge method
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
			parsed, err := cfg.LoadOTELConfig(content)
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

		return enhancedGenerator.GenerateWithKnowledge(ctx, req)
	} else {
		ui.Logf("Knowledge base not available, using standard generator\n")
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
		parsed, err := cfg.LoadOTELConfig(content)
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

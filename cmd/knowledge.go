package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/pipeline"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:     "knowledge [command]",
	Aliases: []string{"kb"},
	Short:   "Manage OpenTelemetry knowledge base",
	Long: `Knowledge base management for OpenTelemetry components.

This command provides tools to discover, update, and query OpenTelemetry components
across multiple languages and package managers.`,
	RunE: runKnowledge,
}

var updateCmd = &cobra.Command{
	Use:   "update [language]",
	Short: "Update the knowledge base for a specific language or all languages",
	Long: `Update the knowledge base by reading from the local OpenTelemetry registry.

Examples:
  lawrence knowledge update                    # Update all supported languages
  lawrence knowledge update go                # Update Go language only
  lawrence knowledge update --output data.db  # Save to specific file
  lawrence knowledge update --force           # Force update even if file exists
  lawrence knowledge update --registry .registry  # Use specific local registry path`,
	Args: cobra.MaximumNArgs(1),
	RunE: runUpdate,
}

func init() {
	knowledgeCmd.AddCommand(updateCmd)

	// Add flags for update command
	updateCmd.Flags().StringP("registry", "", "", "Path to local registry (if not specified, will try to find .registry)")
	updateCmd.Flags().StringP("output", "o", "knowledge.db", "Output file path")
	updateCmd.Flags().BoolP("force", "f", false, "Force update even if recent data exists")
	updateCmd.Flags().StringP("token", "", "", "GitHub personal access token for API authentication (needed for changelog and package data)")

	rootCmd.AddCommand(knowledgeCmd)
}

func runKnowledge(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runUpdate(cmd *cobra.Command, args []string) error {
	languageStr := ""
	if len(args) > 0 {
		languageStr = args[0]
	}
	outputPath, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	registryPath, _ := cmd.Flags().GetString("registry")
	githubToken, _ := cmd.Flags().GetString("token")

	cmdLogger := logger.NewUILogger()

	// Determine registry path
	if registryPath == "" {
		// Try to find registry in common locations
		possiblePaths := []string{
			".registry",
			"registry",
			"../registry",
		}

		for _, path := range possiblePaths {
			if absPath, err := filepath.Abs(path); err == nil {
				if _, err := os.Stat(filepath.Join(absPath, "data", "registry")); err == nil {
					registryPath = absPath
					break
				}
			}
		}

		if registryPath == "" {
			return fmt.Errorf("local registry not found. Please run 'lawrence registry sync' first or specify --registry path")
		}
	}

	// Check if registry exists
	absRegistryPath, err := filepath.Abs(registryPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute registry path: %w", err)
	}

	if _, err := os.Stat(filepath.Join(absRegistryPath, "data", "registry")); os.IsNotExist(err) {
		return fmt.Errorf("registry not found at %s. Please run 'lawrence registry sync' first", absRegistryPath)
	}

	var languages []types.ComponentLanguage

	// If no language was specified, update all supported languages
	if languageStr == "" {
		languages = []types.ComponentLanguage{
			types.ComponentLanguageJavaScript,
			types.ComponentLanguagePython,
			types.ComponentLanguageGo,
			types.ComponentLanguageJava,
			types.ComponentLanguageCSharp,
			types.ComponentLanguagePHP,
			types.ComponentLanguageRuby,
		}
	} else {
		language, err := parseLanguage(languageStr)
		if err != nil {
			return fmt.Errorf("invalid language: %s. Supported languages: %s", languageStr, getSupportedLanguages())
		}
		languages = append(languages, language)
	}

	// Check if output file exists and is recent (unless force is specified)
	if !force {
		if exists, recent := checkFileRecency(outputPath); exists && recent {
			cmdLogger.Logf("Knowledge base file %s exists and is recent. Use --force to update anyway.\n", outputPath)
			return nil
		}
	}

	storageClient, err := storage.NewStorage(outputPath, cmdLogger)
	if err != nil {
		return fmt.Errorf("failed to create storage client: %w", err)
	}

	providerFactory := providers.NewProviderFactory(absRegistryPath, cmdLogger)
	pipeline := pipeline.NewPipeline(providerFactory, cmdLogger, githubToken, storageClient)

	for _, language := range languages {
		err := pipeline.UpdateKnowledgeBase(language)
		if err != nil {
			return fmt.Errorf("failed to update knowledge base: %w", err)
		}
	}
	return nil
}

func parseLanguage(languageStr string) (types.ComponentLanguage, error) {
	switch strings.ToLower(languageStr) {
	case "javascript", "js":
		return types.ComponentLanguageJavaScript, nil
	case "python", "py":
		return types.ComponentLanguagePython, nil
	case "go":
		return types.ComponentLanguageGo, nil
	case "java":
		return types.ComponentLanguageJava, nil
	case "csharp", "c#":
		return types.ComponentLanguageCSharp, nil
	case "php":
		return types.ComponentLanguagePHP, nil
	case "ruby":
		return types.ComponentLanguageRuby, nil
	default:
		return "", fmt.Errorf("unsupported language: %s", languageStr)
	}
}

func getSupportedLanguages() string {
	languages := []string{"javascript", "python", "go", "java", "csharp", "php", "ruby"}
	return strings.Join(languages, ", ")
}

func checkFileRecency(filePath string) (bool, bool) {
	info, err := os.Stat(filePath)
	if err != nil {
		return false, false
	}

	// Consider file recent if it's less than 24 hours old
	recent := time.Since(info.ModTime()) < 24*time.Hour
	return true, recent
}

package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/pipeline"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
	"github.com/spf13/cobra"
)

var knowledgeCmd = &cobra.Command{
	Use:   "knowledge [command]",
	Short: "Manage OpenTelemetry knowledge base",
	Long: `Knowledge base management for OpenTelemetry components.

This command provides tools to discover, update, and query OpenTelemetry components
across multiple languages and package managers.`,
	RunE: runKnowledge,
}

var updateCmd = &cobra.Command{
	Use:   "update [language]",
	Short: "Update the knowledge base for a specific language",
	Long: `Update the OpenTelemetry knowledge base by fetching fresh data
from the registry and package manager for the specified language.

Supported languages: javascript, python, go, java, csharp, php, ruby

Examples:
  lawrence knowledge update javascript
  lawrence knowledge update python
  lawrence knowledge update go`,
	Args: cobra.ExactArgs(1),
	RunE: runUpdate,
}

var queryCmd = &cobra.Command{
	Use:   "query [query]",
	Short: "Query the knowledge base",
	Long: `Query the knowledge base for components matching specific criteria.

Examples:
  lawrence knowledge query --language javascript --type Instrumentation
  lawrence knowledge query --name express
  lawrence knowledge query --status stable
  lawrence knowledge query --category EXPERIMENTAL
  lawrence knowledge query --support-level official
  lawrence knowledge query --framework express`,
	RunE: runQuery,
}

var infoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show knowledge base information",
	Long: `Display information about the current knowledge base including
statistics, supported languages, and metadata.`,
	RunE: runInfo,
}

var providersCmd = &cobra.Command{
	Use:   "providers",
	Short: "List available providers",
	Long: `List all available language and registry providers.

This command shows which languages and package managers are supported
by the knowledge base system.`,
	RunE: runProviders,
}

func init() {
	knowledgeCmd.AddCommand(updateCmd)
	knowledgeCmd.AddCommand(queryCmd)
	knowledgeCmd.AddCommand(infoCmd)
	knowledgeCmd.AddCommand(providersCmd)

	// Add flags for update command
	updateCmd.Flags().StringP("output", "o", "pkg/knowledge/otel_packages.json", "Output file path")
	updateCmd.Flags().BoolP("force", "f", false, "Force update even if recent data exists")

	// Add flags for query command
	queryCmd.Flags().StringP("language", "l", "", "Filter by language")
	queryCmd.Flags().StringP("type", "t", "", "Filter by component type")
	queryCmd.Flags().StringP("category", "c", "", "Filter by component category")
	queryCmd.Flags().StringP("status", "s", "", "Filter by component status")
	queryCmd.Flags().StringP("support-level", "", "", "Filter by support level")
	queryCmd.Flags().StringP("name", "n", "", "Filter by component name (partial match)")
	queryCmd.Flags().String("version", "", "Filter by version")
	queryCmd.Flags().StringP("framework", "", "", "Filter by instrumentation target framework")
	queryCmd.Flags().StringP("output", "o", "text", "Output format (text, json)")
	queryCmd.Flags().StringP("file", "f", "pkg/knowledge/otel_packages.json", "Knowledge base file path")

	// Add flags for info command
	infoCmd.Flags().StringP("file", "f", "pkg/knowledge/otel_packages.json", "Knowledge base file path")
	infoCmd.Flags().StringP("output", "o", "text", "Output format (text, json)")

	rootCmd.AddCommand(knowledgeCmd)
}

func runKnowledge(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runUpdate(cmd *cobra.Command, args []string) error {
	languageStr := args[0]
	outputPath, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")

	// Parse language
	language, err := parseLanguage(languageStr)
	if err != nil {
		return fmt.Errorf("invalid language: %s. Supported languages: %s", languageStr, getSupportedLanguages())
	}

	// Check if output file exists and is recent (unless force is specified)
	if !force {
		if exists, recent := checkFileRecency(outputPath); exists && recent {
			fmt.Printf("Knowledge base file %s exists and is recent. Use --force to update anyway.\n", outputPath)
			return nil
		}
	}

	// Create pipeline
	p := pipeline.NewPipeline()

	// Update knowledge base
	kb, err := p.UpdateKnowledgeBase(language)
	if err != nil {
		return fmt.Errorf("failed to update knowledge base: %w", err)
	}

	// Save to file
	storageClient := storage.NewStorage("")
	if err := storageClient.SaveKnowledgeBase(kb, outputPath); err != nil {
		return fmt.Errorf("failed to save knowledge base: %w", err)
	}

	fmt.Printf("Successfully updated knowledge base for %s. Saved to %s\n", language, outputPath)
	fmt.Printf("Total components: %d, Total versions: %d\n", kb.Statistics.TotalComponents, kb.Statistics.TotalVersions)

	return nil
}

func runQuery(cmd *cobra.Command, args []string) error {
	language, _ := cmd.Flags().GetString("language")
	componentType, _ := cmd.Flags().GetString("type")
	category, _ := cmd.Flags().GetString("category")
	status, _ := cmd.Flags().GetString("status")
	supportLevel, _ := cmd.Flags().GetString("support-level")
	name, _ := cmd.Flags().GetString("name")
	version, _ := cmd.Flags().GetString("version")
	framework, _ := cmd.Flags().GetString("framework")
	outputFormat, _ := cmd.Flags().GetString("output")
	filePath, _ := cmd.Flags().GetString("file")

	// Load knowledge base
	storageClient := storage.NewStorage("")
	kb, err := storageClient.LoadKnowledgeBase(filePath)
	if err != nil {
		return fmt.Errorf("failed to load knowledge base: %w", err)
	}

	// Build query
	query := storage.Query{
		Language:     language,
		Type:         componentType,
		Category:     category,
		Status:       status,
		SupportLevel: supportLevel,
		Name:         name,
		Version:      version,
		Framework:    framework,
	}

	// Execute query
	result := storageClient.QueryKnowledgeBase(kb, query)

	// Output results
	switch outputFormat {
	case "json":
		return outputKnowledgeJSON(result)
	case "text":
		return outputQueryText(result)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

func runInfo(cmd *cobra.Command, args []string) error {
	filePath, _ := cmd.Flags().GetString("file")
	outputFormat, _ := cmd.Flags().GetString("output")

	// Load knowledge base
	storageClient := storage.NewStorage("")
	kb, err := storageClient.LoadKnowledgeBase(filePath)
	if err != nil {
		return fmt.Errorf("failed to load knowledge base: %w", err)
	}

	// Output information
	switch outputFormat {
	case "json":
		return outputKnowledgeJSON(kb.Statistics)
	case "text":
		return outputInfoText(kb)
	default:
		return fmt.Errorf("unsupported output format: %s", outputFormat)
	}
}

func runProviders(cmd *cobra.Command, args []string) error {
	// Create provider factory to list available providers
	factory := providers.NewProviderFactory()

	fmt.Printf("Available Providers\n")
	fmt.Printf("==================\n\n")

	// List supported languages
	languages := factory.ListSupportedLanguages()
	fmt.Printf("Supported Languages (%d):\n", len(languages))
	for _, lang := range languages {
		provider, err := factory.GetProvider(lang)
		if err != nil {
			fmt.Printf("  %s: Error getting provider\n", lang)
			continue
		}

		registryProvider, _ := factory.GetRegistryProvider(lang)
		packageManagerProvider, _ := factory.GetPackageManagerProvider(lang)

		fmt.Printf("  %s:\n", lang)
		fmt.Printf("    Provider: %s\n", provider.GetName())
		if registryProvider != nil {
			fmt.Printf("    Registry: %s (%s)\n", registryProvider.GetName(), registryProvider.GetRegistryType())
		}
		if packageManagerProvider != nil {
			fmt.Printf("    Package Manager: %s (%s)\n", packageManagerProvider.GetName(), packageManagerProvider.GetPackageManagerType())
		}
		fmt.Printf("    Status: %s\n", getProviderStatus(provider))
		fmt.Printf("\n")
	}

	return nil
}

// Helper functions

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

func getProviderStatus(provider providers.Provider) string {
	ctx := context.Background()
	if provider.IsHealthy(ctx) {
		return "Healthy"
	}
	return "Unhealthy"
}

func outputKnowledgeJSON(data interface{}) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

func outputQueryText(result *storage.QueryResult) error {
	fmt.Printf("Query Results (%d components found):\n\n", result.Total)

	for i, component := range result.Components {
		fmt.Printf("%d. %s (%s)\n", i+1, component.Name, component.Type)
		fmt.Printf("   Language: %s\n", component.Language)
		if component.Category != "" {
			fmt.Printf("   Category: %s\n", component.Category)
		}
		if component.Status != "" {
			fmt.Printf("   Status: %s\n", component.Status)
		}
		if component.SupportLevel != "" {
			fmt.Printf("   Support Level: %s\n", component.SupportLevel)
		}
		fmt.Printf("   Repository: %s\n", component.Repository)
		fmt.Printf("   Versions: %d\n", len(component.Versions))
		if len(component.InstrumentationTargets) > 0 {
			fmt.Printf("   Instrumentation Targets: ")
			for j, target := range component.InstrumentationTargets {
				if j > 0 {
					fmt.Printf(", ")
				}
				fmt.Printf("%s %s", target.Framework, target.VersionRange)
			}
			fmt.Printf("\n")
		}
		if len(component.Tags) > 0 {
			fmt.Printf("   Tags: %v\n", component.Tags)
		}
		fmt.Println()
	}

	return nil
}

func outputInfoText(kb *types.KnowledgeBase) error {
	fmt.Printf("Knowledge Base Information\n")
	fmt.Printf("==========================\n\n")
	fmt.Printf("Schema Version: %s\n", kb.SchemaVersion)
	fmt.Printf("Generated At: %s\n", kb.GeneratedAt.Format("2006-01-02 15:04:05"))
	fmt.Printf("Total Components: %d\n", kb.Statistics.TotalComponents)
	fmt.Printf("Total Versions: %d\n", kb.Statistics.TotalVersions)
	fmt.Printf("Last Update: %s\n", kb.Statistics.LastUpdate.Format("2006-01-02 15:04:05"))
	fmt.Printf("Source: %s\n\n", kb.Statistics.Source)

	// Show metadata if available
	if len(kb.Metadata) > 0 {
		fmt.Printf("Metadata:\n")
		for key, value := range kb.Metadata {
			fmt.Printf("  %s: %v\n", key, value)
		}
		fmt.Printf("\n")
	}

	fmt.Printf("Components by Language:\n")
	for lang, count := range kb.Statistics.ByLanguage {
		fmt.Printf("  %s: %d\n", lang, count)
	}

	fmt.Printf("\nComponents by Type:\n")
	for compType, count := range kb.Statistics.ByType {
		fmt.Printf("  %s: %d\n", compType, count)
	}

	if len(kb.Statistics.ByCategory) > 0 {
		fmt.Printf("\nComponents by Category:\n")
		for category, count := range kb.Statistics.ByCategory {
			fmt.Printf("  %s: %d\n", category, count)
		}
	}

	if len(kb.Statistics.ByStatus) > 0 {
		fmt.Printf("\nComponents by Status:\n")
		for status, count := range kb.Statistics.ByStatus {
			fmt.Printf("  %s: %d\n", status, count)
		}
	}

	if len(kb.Statistics.BySupportLevel) > 0 {
		fmt.Printf("\nComponents by Support Level:\n")
		for supportLevel, count := range kb.Statistics.BySupportLevel {
			fmt.Printf("  %s: %d\n", supportLevel, count)
		}
	}

	return nil
}

func getKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

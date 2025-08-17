package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"runtime"
	"sync"

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
	Short: "Update the knowledge base for a specific language or all languages",
	Long: `Update the knowledge base by fetching the latest information from the OpenTelemetry registry.

Examples:
  lawrence knowledge update                    # Update all supported languages in parallel
  lawrence knowledge update go                # Update Go language only
  lawrence knowledge update --output data.db  # Save to specific file
  lawrence knowledge update --force           # Force update even if file exists
  lawrence knowledge update --workers 4      # Use specific number of parallel workers
  lawrence knowledge update --rate-limit 50  # Limit API requests per second per worker`,
	Args: cobra.MaximumNArgs(1),
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
	updateCmd.Flags().IntP("workers", "w", 0, "Number of parallel workers (0 = auto-detect based on CPU cores)")
	updateCmd.Flags().IntP("rate-limit", "r", 100, "Rate limit for API requests per second per worker")

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
	languageStr := ""
	if len(args) > 0 {
		languageStr = args[0]
	}
	outputPath, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	workerCount, _ := cmd.Flags().GetInt("workers")
	rateLimit, _ := cmd.Flags().GetInt("rate-limit")

	// If no language was specified, update all supported languages
	if languageStr == "" {
		fmt.Println("Updating all supported languages in parallel...")
		supportedLanguages := []types.ComponentLanguage{
			types.ComponentLanguageJavaScript,
			types.ComponentLanguagePython,
			types.ComponentLanguageGo,
			types.ComponentLanguageJava,
			types.ComponentLanguageCSharp,
			types.ComponentLanguagePHP,
			types.ComponentLanguageRuby,
		}

		// Use parallel processing for better performance
		return runUpdateAllLanguagesParallel(supportedLanguages, outputPath, workerCount, rateLimit)
	}

	// Validate language if one was specified
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
	storageClient, err := storage.NewStorage("")
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storageClient.Close()

	if err := storageClient.SaveKnowledgeBase(kb, outputPath); err != nil {
		return fmt.Errorf("failed to save knowledge base: %w", err)
	}

	fmt.Printf("Successfully updated knowledge base for %s. Saved to %s\n", language, outputPath)
	fmt.Printf("Total components: %d, Total versions: %d\n", kb.Statistics.TotalComponents, kb.Statistics.TotalVersions)

	return nil
}

// runUpdateAllLanguagesParallel processes all languages in parallel using channels and goroutines
func runUpdateAllLanguagesParallel(supportedLanguages []types.ComponentLanguage, outputPath string, workerCount int, rateLimit int) error {
	startTime := time.Now()

	// Determine optimal number of workers
	var numWorkers int
	if workerCount > 0 {
		// Use user-specified worker count
		numWorkers = workerCount
		if numWorkers > len(supportedLanguages) {
			numWorkers = len(supportedLanguages) // Don't create more workers than languages
		}
	} else {
		// Auto-detect based on CPU cores
		numWorkers = runtime.NumCPU()
		if numWorkers > len(supportedLanguages) {
			numWorkers = len(supportedLanguages) // Don't create more workers than languages
		}
		if numWorkers > 8 {
			numWorkers = 8 // Cap at 8 to avoid overwhelming external APIs
		}
	}

	fmt.Printf("Using %d parallel workers for %d languages\n", numWorkers, len(supportedLanguages))

	// Create channels for parallel processing
	languageChan := make(chan types.ComponentLanguage, len(supportedLanguages))
	resultChan := make(chan *languageUpdateResult, len(supportedLanguages))
	errorChan := make(chan error, 1)
	stopChan := make(chan struct{})

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go languageUpdateWorker(languageChan, resultChan, errorChan, stopChan, &wg, rateLimit)
	}

	// Send languages to workers
	go func() {
		defer close(languageChan)
		for _, lang := range supportedLanguages {
			select {
			case languageChan <- lang:
			case <-stopChan:
				return
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Process results and build combined knowledge base
	var allComponents []types.Component
	var totalVersions int
	var successfulLanguages []types.ComponentLanguage
	var failedLanguages []string

	// Collect results from all workers
	for result := range resultChan {
		if result.err != nil {
			failedLanguages = append(failedLanguages, fmt.Sprintf("%s (%v)", result.language, result.err))
			continue
		}

		successfulLanguages = append(successfulLanguages, result.language)
		allComponents = append(allComponents, result.kb.Components...)
		totalVersions += result.kb.Statistics.TotalVersions

		fmt.Printf("✓ %s: %d components, %d versions\n", result.language, len(result.kb.Components), result.kb.Statistics.TotalVersions)
	}

	// Check for any errors that occurred during processing
	select {
	case err := <-errorChan:
		close(stopChan)
		wg.Wait()
		return fmt.Errorf("parallel processing error: %w", err)
	default:
		// No errors, continue
	}

	// Wait for all workers to finish
	wg.Wait()

	// Report results
	fmt.Printf("\n📊 Parallel processing completed in %v\n", time.Since(startTime))
	fmt.Printf("✅ Successful languages: %d/%d\n", len(successfulLanguages), len(supportedLanguages))

	if len(failedLanguages) > 0 {
		fmt.Printf("❌ Failed languages: %d/%d\n", len(failedLanguages), len(supportedLanguages))
		for _, failed := range failedLanguages {
			fmt.Printf("   - %s\n", failed)
		}
	}

	if len(allComponents) == 0 {
		return fmt.Errorf("no components were successfully updated")
	}

	// Create combined knowledge base
	combinedKB := &types.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components:    allComponents,
		Metadata: map[string]interface{}{
			"source":           "OpenTelemetry Registry (Global Update)",
			"languages":        successfulLanguages,
			"update_timestamp": time.Now().Unix(),
			"processing_mode":  "parallel",
			"workers_used":     numWorkers,
			"rate_limit":       rateLimit,
			"total_languages":  len(supportedLanguages),
			"successful":       len(successfulLanguages),
			"failed":           len(failedLanguages),
		},
		Statistics: types.Statistics{
			TotalComponents: len(allComponents),
			TotalVersions:   totalVersions,
			LastUpdate:      time.Now(),
			Source:          "OpenTelemetry Registry (Global Update)",
			ByLanguage:      make(map[string]int),
			ByType:          make(map[string]int),
			ByCategory:      make(map[string]int),
			ByStatus:        make(map[string]int),
			BySupportLevel:  make(map[string]int),
		},
	}

	// Generate statistics
	for _, component := range allComponents {
		// Count by language
		lang := string(component.Language)
		combinedKB.Statistics.ByLanguage[lang]++

		// Count by type
		compType := string(component.Type)
		combinedKB.Statistics.ByType[compType]++

		// Count by category
		if component.Category != "" {
			category := string(component.Category)
			combinedKB.Statistics.ByCategory[category]++
		}

		// Count by status
		if component.Status != "" {
			status := string(component.Status)
			combinedKB.Statistics.ByStatus[status]++
		}

		// Count by support level
		if component.SupportLevel != "" {
			supportLevel := string(component.SupportLevel)
			combinedKB.Statistics.BySupportLevel[supportLevel]++
		}
	}

	// Save to file
	storageClient, err := storage.NewStorage("")
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storageClient.Close()

	if err := storageClient.SaveKnowledgeBase(combinedKB, outputPath); err != nil {
		return fmt.Errorf("failed to save combined knowledge base: %w", err)
	}

	fmt.Printf("\n🎉 All supported languages updated successfully!\n")
	fmt.Printf("📁 Saved to: %s\n", outputPath)
	fmt.Printf("📊 Total components: %d\n", combinedKB.Statistics.TotalComponents)
	fmt.Printf("📦 Total versions: %d\n", combinedKB.Statistics.TotalVersions)

	// Show breakdown by language
	fmt.Printf("\n📈 Breakdown by language:\n")
	for lang, count := range combinedKB.Statistics.ByLanguage {
		fmt.Printf("  %s: %d components\n", lang, count)
	}

	return nil
}

// languageUpdateResult represents the result of updating a single language
type languageUpdateResult struct {
	language types.ComponentLanguage
	kb       *types.KnowledgeBase
	err      error
}

// languageUpdateWorker processes language updates in parallel
func languageUpdateWorker(languageChan <-chan types.ComponentLanguage, resultChan chan<- *languageUpdateResult, errorChan chan<- error, stopChan <-chan struct{}, wg *sync.WaitGroup, rateLimit int) {
	defer wg.Done()

	// Create rate limiter for this worker
	var rateLimiter *time.Ticker
	if rateLimit > 0 {
		rateLimiter = time.NewTicker(time.Second / time.Duration(rateLimit))
		defer rateLimiter.Stop()
	}

	for language := range languageChan {
		// Check if we should stop
		select {
		case <-stopChan:
			return
		default:
		}

		// Apply rate limiting if configured
		if rateLimiter != nil {
			select {
			case <-rateLimiter.C:
			case <-stopChan:
				return
			}
		}

		// Update knowledge base for this language
		p := pipeline.NewPipeline()
		kb, err := p.UpdateKnowledgeBase(language)

		// Send result
		result := &languageUpdateResult{
			language: language,
			kb:       kb,
			err:      err,
		}

		select {
		case resultChan <- result:
		case <-stopChan:
			return
		}

		// If there was an error, try to send it to the error channel
		if err != nil {
			select {
			case errorChan <- fmt.Errorf("failed to update %s knowledge base: %w", language, err):
			default:
				// Error channel is full, continue processing other languages
			}
		}
	}
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
	storageClient, err := storage.NewStorage("")
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storageClient.Close()

	kb, err := storageClient.LoadKnowledgeBase(filePath)
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
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
	storageClient, err := storage.NewStorage("")
	if err != nil {
		return fmt.Errorf("failed to create storage: %w", err)
	}
	defer storageClient.Close()

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

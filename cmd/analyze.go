package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/detector/types"
	"github.com/spf13/cobra"
)

// analyzeCmd represents the analyze command
var analyzeCmd = &cobra.Command{
	Use:   "analyze [path]",
	Short: "Analyze a codebase for OpenTelemetry usage and issues",
	Long: `Analyze analyzes the specified codebase (or current directory) to:
- Detect programming languages in use
- Find OpenTelemetry libraries and instrumentation
- Identify potential issues and improvements
- Provide recommendations for better observability

Example usage:
  lawrence analyze                    # Analyze current directory
  lawrence analyze /path/to/project   # Analyze specific directory
  lawrence analyze --output json      # Output results as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: runAnalyze,
}

func init() {
	rootCmd.AddCommand(analyzeCmd)

	// Add analyze-specific flags
	analyzeCmd.Flags().BoolP("detailed", "d", false, "Show detailed analysis including file-level information")
	analyzeCmd.Flags().StringSliceP("languages", "l", []string{}, "Limit analysis to specific languages (go, python, java, etc.)")
	analyzeCmd.Flags().StringSliceP("categories", "", []string{}, "Limit issues to specific categories (missing_library, configuration, etc.)")
}

func runAnalyze(cmd *cobra.Command, args []string) error {
	// Get target path
	targetPath := "."
	if len(args) > 0 {
		targetPath = args[0]
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if path exists
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Get flags
	verbose, _ := cmd.Flags().GetBool("verbose")
	detailed, _ := cmd.Flags().GetBool("detailed")
	outputFormat, _ := cmd.Flags().GetString("output")
	languageFilter, _ := cmd.Flags().GetStringSlice("languages")
	categoryFilter, _ := cmd.Flags().GetStringSlice("categories")

	if verbose {
		fmt.Printf("Analyzing codebase at: %s\n", absPath)
	}

	// Create detection manager
	manager := detector.NewManager([]detector.IssueDetector{
		issues.NewMissingOTelDetector(),
	}, map[string]detector.Language{
		"go":     languages.NewGoDetector(),
		"python": languages.NewPythonDetector(),
	})

	// Run analysis
	ctx := context.Background()
	analysis, detectedIssues, err := manager.AnalyzeCodebase(ctx, absPath)
	if err != nil {
		return fmt.Errorf("analysis failed: %w", err)
	}

	// Filter results if requested
	if len(languageFilter) > 0 {
		analysis = filterAnalysisByLanguages(analysis, languageFilter)
		detectedIssues = filterIssuesByLanguages(detectedIssues, languageFilter)
	}

	if len(categoryFilter) > 0 {
		detectedIssues = filterIssuesByCategories(detectedIssues, categoryFilter)
	}

	// Output results
	switch outputFormat {
	case "json":
		return outputJSON(analysis, detectedIssues)
	case "yaml":
		return outputYAML(analysis, detectedIssues)
	default:
		return outputText(analysis, detectedIssues, detailed, verbose)
	}
}

func outputText(analysis *detector.Analysis, issues []types.Issue, detailed, verbose bool) error {
	fmt.Printf("📊 OpenTelemetry Analysis Results\n")
	fmt.Printf("=================================\n\n")

	// Summary
	fmt.Printf("📂 Project Path: %s\n", analysis.RootPath)
	fmt.Printf("🗣️  Languages Detected: %v\n", analysis.DetectedLanguages)
	fmt.Printf("📦 OpenTelemetry Libraries: %d\n", len(analysis.Libraries))
	fmt.Printf("📥 All Packages: %d\n", len(analysis.Packages))
	fmt.Printf("🔧 Available Instrumentations: %d\n", len(analysis.AvailableInstrumentations))
	fmt.Printf("⚠️  Issues Found: %d\n\n", len(issues))

	// Libraries
	if len(analysis.Libraries) > 0 {
		fmt.Printf("📦 OpenTelemetry Libraries Found:\n")
		fmt.Printf("---------------------------------\n")
		for _, lib := range analysis.Libraries {
			if lib.Version != "" {
				fmt.Printf("  • %s (%s) - %s\n", lib.Name, lib.Version, lib.Language)
			} else {
				fmt.Printf("  • %s - %s\n", lib.Name, lib.Language)
			}
			if detailed && lib.PackageFile != "" {
				fmt.Printf("    📄 Found in: %s\n", lib.PackageFile)
			}
		}
		fmt.Println()
	}

	// Available Instrumentations
	if len(analysis.AvailableInstrumentations) > 0 {
		fmt.Printf("🔧 Available OpenTelemetry Instrumentations:\n")
		fmt.Printf("-------------------------------------------\n")
		for _, instrumentation := range analysis.AvailableInstrumentations {
			status := "🔧"
			if instrumentation.IsFirstParty {
				status = "✅"
			}

			fmt.Printf("  %s %s (%s)\n", status, instrumentation.Package.Name, instrumentation.Language)
			if instrumentation.Title != "" && instrumentation.Title != instrumentation.Package.Name {
				fmt.Printf("    📝 %s\n", instrumentation.Title)
			}
			if detailed && instrumentation.Description != "" {
				fmt.Printf("    💬 %s\n", instrumentation.Description)
			}
			if detailed && len(instrumentation.Tags) > 0 {
				fmt.Printf("    🏷️  Tags: %s\n", strings.Join(instrumentation.Tags, ", "))
			}
		}
		fmt.Println()
	}

	// Issues
	if len(issues) > 0 {
		fmt.Printf("⚠️  Issues and Recommendations:\n")
		fmt.Printf("-------------------------------\n")

		// Group issues by severity
		errors := filterIssuesBySeverity(issues, types.SeverityError)
		warnings := filterIssuesBySeverity(issues, types.SeverityWarning)
		infos := filterIssuesBySeverity(issues, types.SeverityInfo)

		printIssuesByCategory("🚨 Errors", errors, detailed)
		printIssuesByCategory("⚠️  Warnings", warnings, detailed)
		printIssuesByCategory("ℹ️  Information", infos, detailed)
	} else {
		fmt.Printf("✅ No issues found! Your OpenTelemetry setup looks good.\n")
	}

	return nil
}

func printIssuesByCategory(title string, issues []types.Issue, detailed bool) {
	if len(issues) == 0 {
		return
	}

	fmt.Printf("%s (%d):\n", title, len(issues))
	for i, issue := range issues {
		fmt.Printf("  %d. %s\n", i+1, issue.Title)
		fmt.Printf("     %s\n", issue.Description)

		if issue.Suggestion != "" {
			fmt.Printf("     💡 %s\n", issue.Suggestion)
		}

		if detailed {
			if issue.File != "" {
				location := issue.File
				if issue.Line > 0 {
					location = fmt.Sprintf("%s:%d", location, issue.Line)
				}
				fmt.Printf("     📍 %s\n", location)
			}
			if len(issue.References) > 0 {
				fmt.Printf("     🔗 References: %v\n", issue.References)
			}
		}
		fmt.Println()
	}
}

func outputJSON(analysis *detector.Analysis, issues []types.Issue) error {
	result := map[string]interface{}{
		"analysis": analysis,
		"issues":   issues,
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func outputYAML(analysis *detector.Analysis, issues []types.Issue) error {
	// For simplicity, output as JSON for now
	// In a real implementation, you'd use a YAML library
	return outputJSON(analysis, issues)
}

// Filter functions

func filterAnalysisByLanguages(analysis *detector.Analysis, languages []string) *detector.Analysis {
	filtered := *analysis
	filtered.DetectedLanguages = filterStringSlice(analysis.DetectedLanguages, languages)

	var filteredLibs []types.Library
	for _, lib := range analysis.Libraries {
		if containsString(languages, lib.Language) {
			filteredLibs = append(filteredLibs, lib)
		}
	}
	filtered.Libraries = filteredLibs

	return &filtered
}

func filterIssuesByLanguages(issues []types.Issue, languages []string) []types.Issue {
	var filtered []types.Issue
	for _, issue := range issues {
		// Include issues with no language specified (general issues)
		if issue.Language == "" || containsString(languages, issue.Language) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func filterIssuesByCategories(issues []types.Issue, categories []string) []types.Issue {
	var filtered []types.Issue
	for _, issue := range issues {
		if containsString(categories, string(issue.Category)) {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

func filterIssuesBySeverity(issues []types.Issue, severity types.Severity) []types.Issue {
	var filtered []types.Issue
	for _, issue := range issues {
		if issue.Severity == severity {
			filtered = append(filtered, issue)
		}
	}
	return filtered
}

// Helper functions

func filterStringSlice(slice, filter []string) []string {
	var result []string
	for _, item := range slice {
		if containsString(filter, item) {
			result = append(result, item)
		}
	}
	return result
}

func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

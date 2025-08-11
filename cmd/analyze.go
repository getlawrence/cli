package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/ui"
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
	absPath, pathErr := filepath.Abs(targetPath)
	if pathErr != nil {
		return fmt.Errorf("failed to resolve path: %w", pathErr)
	}

	// Check if path exists
	if _, statErr := os.Stat(absPath); os.IsNotExist(statErr) {
		return fmt.Errorf("path does not exist: %s", absPath)
	}

	// Get flags
	verbose, _ := cmd.Flags().GetBool("verbose")
	detailed, _ := cmd.Flags().GetBool("detailed")
	outputFormat, _ := cmd.Flags().GetString("output")

	if verbose {
		fmt.Printf("Analyzing codebase at: %s\n", absPath)
	}

	// Create analysis engine
	codebaseAnalyzer := detector.NewCodebaseAnalyzer([]detector.IssueDetector{
		issues.NewMissingOTelDetector(),
	}, map[string]detector.Language{
		"go":         languages.NewGoDetector(),
		"python":     languages.NewPythonDetector(),
		"javascript": languages.NewJavaScriptDetector(),
		"java":       languages.NewJavaDetector(),
		"csharp":     languages.NewDotNetDetector(),
		"ruby":       languages.NewRubyDetector(),
		"php":        languages.NewPHPDetector(),
	})

	// Run analysis
	ctx := cmd.Context()

	var analysis *detector.Analysis
	// Always use spinner TUI while analyzing
	runErr := ui.RunSpinner(ctx, "Analyzing codebase...", func() error {
		var e error
		analysis, e = codebaseAnalyzer.AnalyzeCodebase(ctx, absPath)
		return e
	})
	if runErr != nil {
		return runErr
	}

	switch outputFormat {
	case "json":
		return outputJSON(analysis)
	default:
		return outputText(analysis, detailed)
	}
}

func outputText(analysis *detector.Analysis, detailed bool) error {
	fmt.Print(ui.RenderAnalysis(analysis, detailed))
	return nil
}

func outputJSON(analysis *detector.Analysis) error {
	// Aggregate data from all directories for backward compatibility
	var allIssues []interface{}
	var allLibraries []interface{}
	var allPackages []interface{}
	var allInstrumentations []interface{}
	detectedLanguages := make(map[string]bool)

	for _, dirAnalysis := range analysis.DirectoryAnalyses {
		for _, it := range dirAnalysis.Issues {
			allIssues = append(allIssues, it)
		}
		for _, it := range dirAnalysis.Libraries {
			allLibraries = append(allLibraries, it)
		}
		for _, it := range dirAnalysis.Packages {
			allPackages = append(allPackages, it)
		}
		for _, it := range dirAnalysis.AvailableInstrumentations {
			allInstrumentations = append(allInstrumentations, it)
		}
		if dirAnalysis.Language != "" {
			detectedLanguages[dirAnalysis.Language] = true
		}
	}

	// Convert detected languages map to slice
	var languageSlice []string
	for lang := range detectedLanguages {
		languageSlice = append(languageSlice, lang)
	}

	result := map[string]interface{}{
		"analysis":   analysis,
		"all_issues": allIssues,
		"summary": map[string]interface{}{
			"total_directories":      len(analysis.DirectoryAnalyses),
			"total_languages":        len(languageSlice),
			"total_libraries":        len(allLibraries),
			"total_packages":         len(allPackages),
			"total_instrumentations": len(allInstrumentations),
			"total_issues":           len(allIssues),
		},
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

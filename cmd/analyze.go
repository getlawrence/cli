package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/issues"
	"github.com/getlawrence/cli/internal/detector/languages"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
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

	uiLogger := logger.NewUILogger()

	if verbose {
		uiLogger.Logf("Analyzing codebase at: %s\n", absPath)
	}

	// Create storage client for knowledge base
	storageClient, err := storage.NewStorageWithEmbedded("knowledge.db", uiLogger)
	if err != nil {
		return fmt.Errorf("failed to create knowledge storage: %w", err)
	}
	defer storageClient.Close()

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
	}, storageClient, uiLogger)

	analysis, err := codebaseAnalyzer.AnalyzeCodebase(cmd.Context(), absPath)
	if err != nil {
		return err
	}

	switch outputFormat {
	case "json":
		return outputJSON(analysis)
	default:
		return outputText(analysis, detailed, uiLogger)
	}
}

func outputText(analysis *detector.Analysis, detailed bool, logger logger.Logger) error {
	if analysis == nil || len(analysis.DirectoryAnalyses) == 0 {
		logger.Logf("No analysis results to display.\n")
		return nil
	}

	// Stable ordering of directories
	directories := make([]string, 0, len(analysis.DirectoryAnalyses))
	for dir := range analysis.DirectoryAnalyses {
		directories = append(directories, dir)
	}
	sort.Strings(directories)

	// Totals for summary
	var totalLibraries, totalPackages, totalInstrumentations, totalIssues int
	detectedLanguages := make(map[string]bool)

	// Helpers
	joinNonEmpty := func(parts ...string) string {
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			if strings.TrimSpace(p) != "" {
				out = append(out, p)
			}
		}
		return strings.Join(out, " ")
	}

	for _, dir := range directories {
		dirAnalysis := analysis.DirectoryAnalyses[dir]
		if dirAnalysis == nil {
			continue
		}
		detectedLanguages[strings.ToLower(dirAnalysis.Language)] = true
		totalLibraries += len(dirAnalysis.Libraries)
		totalPackages += len(dirAnalysis.Packages)
		totalInstrumentations += len(dirAnalysis.AvailableInstrumentations)
		totalIssues += len(dirAnalysis.Issues)

		// Header
		logger.Logf("Directory: %s\n", dirAnalysis.Directory)
		logger.Logf("Language: %s\n", dirAnalysis.Language)

		// Libraries
		if detailed {
			logger.Logf("Libraries:\n")
			if len(dirAnalysis.Libraries) == 0 {
				logger.Logf("  - none\n")
			} else {
				for _, lib := range dirAnalysis.Libraries {
					name := lib.Name
					ver := lib.Version
					file := lib.PackageFile
					label := name
					if ver != "" {
						label = joinNonEmpty(label, fmt.Sprintf("(%s)", ver))
					}
					if file != "" {
						label = joinNonEmpty(label, fmt.Sprintf("[%s]", file))
					}
					logger.Logf("  - %s\n", label)
				}
			}
		} else {
			logger.Logf("Libraries: %d\n", len(dirAnalysis.Libraries))
		}

		// Packages
		if detailed {
			logger.Logf("Packages:\n")
			if len(dirAnalysis.Packages) == 0 {
				logger.Logf("  - none\n")
			} else {
				for _, pkg := range dirAnalysis.Packages {
					name := pkg.Name
					ver := pkg.Version
					file := pkg.PackageFile
					label := name
					if ver != "" {
						label = joinNonEmpty(label, fmt.Sprintf("(%s)", ver))
					}
					if file != "" {
						label = joinNonEmpty(label, fmt.Sprintf("[%s]", file))
					}
					logger.Logf("  - %s\n", label)
				}
			}
		} else {
			logger.Logf("Packages: %d\n", len(dirAnalysis.Packages))
		}

		// Instrumentations
		if detailed {
			logger.Logf("Instrumentations:\n")
			if len(dirAnalysis.AvailableInstrumentations) == 0 {
				logger.Logf("  - none\n")
			} else {
				for _, inst := range dirAnalysis.AvailableInstrumentations {
					tags := make([]string, 0, 3)
					if inst.IsFirstParty {
						tags = append(tags, "first-party")
					}
					if inst.IsAvailable {
						tags = append(tags, "available")
					} else {
						tags = append(tags, "unavailable")
					}
					if inst.RegistryType != "" {
						tags = append(tags, inst.RegistryType)
					}
					meta := ""
					if len(tags) > 0 {
						meta = fmt.Sprintf(" (%s)", strings.Join(tags, ", "))
					}
					link := inst.URLs.Repo
					suffix := meta
					if link != "" {
						suffix = joinNonEmpty(suffix, fmt.Sprintf("- %s", link))
					}
					logger.Logf("  - %s: %s%s\n", inst.Package.Name, inst.Title, suffix)
				}
			}
		} else {
			logger.Logf("Instrumentations: %d\n", len(dirAnalysis.AvailableInstrumentations))
		}

		// Issues
		if len(dirAnalysis.Issues) > 0 {
			logger.Logf("Issues (%d):\n", len(dirAnalysis.Issues))
			for _, issue := range dirAnalysis.Issues {
				header := fmt.Sprintf("[%s][%s] %s", strings.ToUpper(string(issue.Severity)), string(issue.Category), issue.Title)
				logger.Logf("  - %s\n", header)
				if strings.TrimSpace(issue.Description) != "" {
					logger.Logf("    Description: %s\n", issue.Description)
				}
				if strings.TrimSpace(issue.Suggestion) != "" {
					logger.Logf("    Suggestion: %s\n", issue.Suggestion)
				}
				if len(issue.References) > 0 {
					logger.Logf("    References:\n")
					for _, ref := range issue.References {
						logger.Logf("      - %s\n", ref)
					}
				}
				locParts := make([]string, 0, 2)
				if strings.TrimSpace(issue.File) != "" {
					locParts = append(locParts, issue.File)
				}
				if issue.Line > 0 {
					locParts = append(locParts, fmt.Sprintf("line %d", issue.Line))
				}
				if len(locParts) > 0 {
					logger.Logf("    Location: %s\n", strings.Join(locParts, ": "))
				}
			}
		} else {
			logger.Logf("Issues: 0\n")
		}

		// Spacer between directories
		logger.Logf("\n")
	}

	// Summary footer
	languages := make([]string, 0, len(detectedLanguages))
	for lang := range detectedLanguages {
		languages = append(languages, lang)
	}
	sort.Strings(languages)
	logger.Logf("Summary: %d directories, %d languages [%s], %d libraries, %d packages, %d instrumentations, %d issues\n",
		len(analysis.DirectoryAnalyses), len(languages), strings.Join(languages, ", "), totalLibraries, totalPackages, totalInstrumentations, totalIssues,
	)

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

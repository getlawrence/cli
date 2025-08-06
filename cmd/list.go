package cmd

import (
	"fmt"

	"github.com/getlawrence/cli/internal/detector/types"
	"github.com/spf13/cobra"
)

// listCmd represents the list command
var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List supported languages and issue categories",
	Long: `List displays information about supported programming languages,
issue categories, and detection capabilities.

Available subcommands:
  languages   List supported programming languages
  categories  List available issue categories
  detectors   List all available issue detectors`,
}

var listLanguagesCmd = &cobra.Command{
	Use:   "languages",
	Short: "List supported programming languages",
	Long:  `List all programming languages that Lawrence can analyze for OpenTelemetry usage.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🗣️  Supported Programming Languages:\n")
		fmt.Printf("===================================\n\n")

		languages := []struct {
			name        string
			description string
			files       []string
		}{
			{
				name:        "Go",
				description: "Go programming language with go.mod support",
				files:       []string{"*.go", "go.mod", "go.sum"},
			},
			{
				name:        "Python",
				description: "Python with pip, poetry, and setuptools support",
				files:       []string{"*.py", "requirements.txt", "pyproject.toml", "setup.py"},
			},
		}

		for _, lang := range languages {
			fmt.Printf("📦 %s\n", lang.name)
			fmt.Printf("   %s\n", lang.description)
			fmt.Printf("   File patterns: %v\n\n", lang.files)
		}

		fmt.Printf("💡 More languages coming soon! Contributions welcome.\n")
	},
}

var listCategoriesCmd = &cobra.Command{
	Use:   "categories",
	Short: "List available issue categories",
	Long:  `List all issue categories that Lawrence can detect and analyze.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("📂 Issue Categories:\n")
		fmt.Printf("===================\n\n")

		categories := []struct {
			name        types.Category
			description string
			examples    []string
		}{
			{
				name:        types.CategoryMissingOtel,
				description: "Missing OpenTelemetry libraries or dependencies",
				examples:    []string{"No OTel libraries found", "Missing core instrumentation"},
			},
			{
				name:        types.CategoryConfiguration,
				description: "Configuration issues and misconfigurations",
				examples:    []string{"Invalid endpoint URLs", "Missing environment variables"},
			},
			{
				name:        types.CategoryInstrumentation,
				description: "Instrumentation coverage and completeness",
				examples:    []string{"Missing traces", "Incomplete metrics", "No logging correlation"},
			},
			{
				name:        types.CategoryPerformance,
				description: "Performance-related issues and optimizations",
				examples:    []string{"High sampling rates", "Excessive metric cardinality"},
			},
			{
				name:        types.CategorySecurity,
				description: "Security concerns and vulnerabilities",
				examples:    []string{"Exposed sensitive data", "Insecure endpoints"},
			},
			{
				name:        types.CategoryBestPractice,
				description: "Best practice violations and recommendations",
				examples:    []string{"Inconsistent naming", "Missing resource attributes"},
			},
			{
				name:        types.CategoryDeprecated,
				description: "Deprecated features and outdated libraries",
				examples:    []string{"Old library versions", "Deprecated APIs"},
			},
		}

		for _, cat := range categories {
			fmt.Printf("🏷️  %s\n", cat.name)
			fmt.Printf("   %s\n", cat.description)
			fmt.Printf("   Examples: %v\n\n", cat.examples)
		}
	},
}

var listDetectorsCmd = &cobra.Command{
	Use:   "detectors",
	Short: "List all available issue detectors",
	Long:  `List all issue detectors with their descriptions and capabilities.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("🔍 Available Issue Detectors:\n")
		fmt.Printf("============================\n\n")

		detectors := []struct {
			id          string
			name        string
			description string
			category    types.Category
			languages   []string
		}{
			{
				id:          "missing_otel_libraries",
				name:        "Missing OpenTelemetry Libraries",
				description: "Detects when no OpenTelemetry libraries are found in the codebase",
				category:    types.CategoryMissingOtel,
				languages:   []string{"all"},
			},
			{
				id:          "incomplete_instrumentation",
				name:        "Incomplete Instrumentation",
				description: "Detects when only partial OpenTelemetry instrumentation is present",
				category:    types.CategoryInstrumentation,
				languages:   []string{"all"},
			},
			{
				id:          "outdated_libraries",
				name:        "Outdated Libraries",
				description: "Detects outdated OpenTelemetry library versions",
				category:    types.CategoryDeprecated,
				languages:   []string{"all"},
			},
		}

		for _, det := range detectors {
			fmt.Printf("🔧 %s\n", det.name)
			fmt.Printf("   ID: %s\n", det.id)
			fmt.Printf("   Category: %s\n", det.category)
			fmt.Printf("   Languages: %v\n", det.languages)
			fmt.Printf("   Description: %s\n\n", det.description)
		}

		fmt.Printf("💡 Want to add more detectors? Check the documentation for contributing guidelines.\n")
	},
}

func init() {
	rootCmd.AddCommand(listCmd)
	listCmd.AddCommand(listLanguagesCmd)
	listCmd.AddCommand(listCategoriesCmd)
	listCmd.AddCommand(listDetectorsCmd)
}

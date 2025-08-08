package cmd

import (
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/languages"
	"github.com/spf13/cobra"
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "lawrence",
	Short: "OpenTelemetry codebase analyzer and troubleshooter",
	Long: `Lawrence is a CLI tool for analyzing codebases to detect OpenTelemetry 
deployments and troubleshoot common issues across multiple programming languages.

The tool provides modular detection of libraries and issues, making it easy
to extend support for new languages and problem patterns.`,
	Version: Version,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Register all language plugins and their injectors explicitly
	goPlugin := languages.NewGoPlugin()
	pyPlugin := languages.NewPythonPlugin()

	languages.DefaultRegistry.Register(goPlugin)
	languages.DefaultRegistry.Register(pyPlugin)

	// Register language injectors for code modification
	inj.RegisterLanguageInjector(goPlugin.DisplayName(), goPlugin.Injector())
	inj.RegisterLanguageInjector(pyPlugin.DisplayName(), pyPlugin.Injector())

	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "output format (text, json, yaml)")
}

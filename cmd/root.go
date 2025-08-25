package cmd

import (
	"context"
	"embed"

	"github.com/spf13/cobra"
)

// Context key for configuration
const ConfigKey = "config"

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
func Execute(embeddedDB embed.FS) error {
	config := NewAppConfig(embeddedDB, nil) // Logger will be created per command
	ctx := context.WithValue(context.Background(), ConfigKey, config)
	return rootCmd.ExecuteContext(ctx)
}

func init() {
	// Global flags
	rootCmd.PersistentFlags().BoolP("verbose", "v", false, "verbose output")
	rootCmd.PersistentFlags().StringP("output", "o", "text", "output format (text, json, yaml)")
}

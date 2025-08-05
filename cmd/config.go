package cmd

import (
	"fmt"
	"os"

	"github.com/getlawrence/cli/internal/config"
	"github.com/spf13/cobra"
)

// configCmd represents the config command
var configCmd = &cobra.Command{
	Use:   "config",
	Short: "Manage Lawrence configuration",
	Long: `Manage Lawrence configuration files and settings.

Available subcommands:
  init     Create a new configuration file with default settings
  show     Display current configuration
  path     Show configuration file path`,
}

var configInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Create a new configuration file",
	Long: `Create a new configuration file with default settings.
	
By default, creates .lawrence.json in the current directory.
Use --global to create in the home directory.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		global, _ := cmd.Flags().GetBool("global")
		force, _ := cmd.Flags().GetBool("force")

		var configPath string
		if global {
			homeDir, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			configPath = fmt.Sprintf("%s/.lawrence.json", homeDir)
		} else {
			configPath = ".lawrence.json"
		}

		// Check if config file already exists
		if _, err := os.Stat(configPath); err == nil && !force {
			return fmt.Errorf("configuration file already exists at %s (use --force to overwrite)", configPath)
		}

		// Create default config
		defaultConfig := config.DefaultConfig()

		// Save to file
		if err := config.SaveConfig(defaultConfig, configPath); err != nil {
			return fmt.Errorf("failed to save configuration: %w", err)
		}

		fmt.Printf("✅ Created configuration file at: %s\n", configPath)
		fmt.Printf("💡 Edit this file to customize Lawrence's behavior\n")

		return nil
	},
}

var configShowCmd = &cobra.Command{
	Use:   "show",
	Short: "Display current configuration",
	Long:  `Display the current configuration that Lawrence will use.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		configPath, _ := cmd.Flags().GetString("config")

		// Load configuration
		cfg, err := config.LoadConfig(configPath)
		if err != nil {
			return fmt.Errorf("failed to load configuration: %w", err)
		}

		fmt.Printf("🔧 Current Configuration:\n")
		fmt.Printf("========================\n\n")

		// Display analysis settings
		fmt.Printf("📊 Analysis Settings:\n")
		fmt.Printf("  Exclude Paths: %v\n", cfg.Analysis.ExcludePaths)
		fmt.Printf("  Max Depth: %d\n", cfg.Analysis.MaxDepth)
		fmt.Printf("  Follow Symlinks: %t\n", cfg.Analysis.FollowSymlinks)
		fmt.Printf("  Min Severity: %s\n\n", cfg.Analysis.MinSeverity)

		// Display output settings
		fmt.Printf("📤 Output Settings:\n")
		fmt.Printf("  Format: %s\n", cfg.Output.Format)
		fmt.Printf("  Detailed: %t\n", cfg.Output.Detailed)
		fmt.Printf("  Color: %t\n\n", cfg.Output.Color)

		// Display language settings
		fmt.Printf("🗣️  Language Settings:\n")
		for lang, langConfig := range cfg.Languages {
			fmt.Printf("  %s:\n", lang)
			fmt.Printf("    Enabled: %t\n", langConfig.Enabled)
			fmt.Printf("    File Patterns: %v\n", langConfig.FilePatterns)
			fmt.Printf("    Package Files: %v\n", langConfig.PackageFiles)
			fmt.Printf("    OTel Patterns: %v\n", langConfig.OTelPatterns)
		}
		fmt.Println()

		// Display custom detectors
		if len(cfg.CustomDetectors) > 0 {
			fmt.Printf("🔧 Custom Detectors (%d):\n", len(cfg.CustomDetectors))
			for _, detector := range cfg.CustomDetectors {
				fmt.Printf("  %s (%s):\n", detector.Name, detector.ID)
				fmt.Printf("    Category: %s\n", detector.Category)
				fmt.Printf("    Languages: %v\n", detector.Languages)
				fmt.Printf("    Severity: %s\n", detector.Severity)
			}
		} else {
			fmt.Printf("🔧 Custom Detectors: None configured\n")
		}

		return nil
	},
}

var configPathCmd = &cobra.Command{
	Use:   "path",
	Short: "Show configuration file path",
	Long:  `Show the path to the configuration file that Lawrence is using or would use.`,
	Run: func(cmd *cobra.Command, args []string) {
		configPath, _ := cmd.Flags().GetString("config")

		if configPath == "" {
			configPath = config.GetConfigPath("")
		}

		// Check if file exists
		if _, err := os.Stat(configPath); err == nil {
			fmt.Printf("📄 Active configuration file: %s\n", configPath)
		} else {
			fmt.Printf("📄 Configuration file would be created at: %s\n", configPath)
			fmt.Printf("💡 Run 'lawrence config init' to create it\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(configCmd)
	configCmd.AddCommand(configInitCmd)
	configCmd.AddCommand(configShowCmd)
	configCmd.AddCommand(configPathCmd)

	// Add flags to config init
	configInitCmd.Flags().BoolP("global", "g", false, "Create config in home directory")
	configInitCmd.Flags().BoolP("force", "f", false, "Overwrite existing config file")
}

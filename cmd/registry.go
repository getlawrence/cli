package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/spf13/cobra"
)

var registryCmd = &cobra.Command{
	Use:   "registry [command]",
	Short: "Manage OpenTelemetry registry",
	Long: `Registry management for OpenTelemetry components.

This command provides tools to sync and manage the local OpenTelemetry registry.`,
	RunE: runRegistry,
}

var syncCmd = &cobra.Command{
	Use:   "sync",
	Short: "Sync the OpenTelemetry registry locally",
	Long: `Sync the OpenTelemetry registry by cloning it locally.

This command clones the OpenTelemetry registry repository to a local directory,
allowing fast access to component information without making API calls.`,
	RunE: runSync,
}

var registryInfoCmd = &cobra.Command{
	Use:   "info",
	Short: "Show registry information",
	Long: `Show information about the local OpenTelemetry registry.

This command displays statistics and metadata about the locally synced registry.`,
	RunE: runRegistryInfo,
}

func init() {
	registryCmd.AddCommand(syncCmd)
	registryCmd.AddCommand(registryInfoCmd)

	// Add flags for sync command
	syncCmd.Flags().StringP("path", "p", ".registry", "Local path to store the registry")
	syncCmd.Flags().BoolP("force", "f", false, "Force re-clone even if directory exists")
	syncCmd.Flags().StringP("branch", "b", "main", "Git branch to clone")

	// Add flags for info command
	registryInfoCmd.Flags().StringP("registry", "r", "", "Path to local registry")

	rootCmd.AddCommand(registryCmd)
}

func runRegistry(cmd *cobra.Command, args []string) error {
	return cmd.Help()
}

func runSync(cmd *cobra.Command, args []string) error {
	registryPath, _ := cmd.Flags().GetString("path")
	force, _ := cmd.Flags().GetBool("force")
	branch, _ := cmd.Flags().GetString("branch")

	ui := logger.NewUILogger()

	// Check if git is available
	if _, err := exec.LookPath("git"); err != nil {
		return fmt.Errorf("git is not available: %w", err)
	}

	// Convert to absolute path
	absPath, err := filepath.Abs(registryPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Check if directory already exists
	if _, err := os.Stat(absPath); err == nil {
		if !force {
			ui.Logf("Registry directory already exists at: %s", absPath)
			ui.Log("Use --force to re-clone")
			return nil
		}

		ui.Logf("Removing existing registry directory: %s", absPath)
		if err := os.RemoveAll(absPath); err != nil {
			return fmt.Errorf("failed to remove existing directory: %w", err)
		}
	}

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(absPath)
	if err := os.MkdirAll(parentDir, 0755); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Clone the registry
	ui.Logf("Cloning OpenTelemetry registry to: %s", absPath)
	ui.Logf("Using branch: %s", branch)

	cloneCmd := exec.Command("git", "clone", "--depth", "1", "--branch", branch,
		"https://github.com/open-telemetry/opentelemetry.io.git", absPath)
	cloneCmd.Stdout = os.Stdout
	cloneCmd.Stderr = os.Stderr

	if err := cloneCmd.Run(); err != nil {
		return fmt.Errorf("failed to clone registry: %w", err)
	}

	// Verify the clone was successful
	registryDataPath := filepath.Join(absPath, "data", "registry")
	if _, err := os.Stat(registryDataPath); os.IsNotExist(err) {
		return fmt.Errorf("registry data directory not found after clone: %s", registryDataPath)
	}

	ui.Log("✅ Registry synced successfully!")
	ui.Logf("📁 Location: %s", absPath)
	ui.Logf("📊 Data path: %s", registryDataPath)

	// Show some basic stats
	if err := showRegistryStats(absPath, ui); err != nil {
		ui.Logf("Warning: could not show registry stats: %v", err)
	}

	return nil
}

func runRegistryInfo(cmd *cobra.Command, args []string) error {
	ui := logger.NewUILogger()

	// Get registry path from flags or try common locations
	registryPath, _ := cmd.Flags().GetString("registry")

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
	}

	ui.Logf("📁 Registry location: %s", registryPath)

	if err := showRegistryStats(registryPath, ui); err != nil {
		return fmt.Errorf("failed to show registry stats: %w", err)
	}

	return nil
}

func showRegistryStats(registryPath string, ui logger.Logger) error {
	registryDataPath := filepath.Join(registryPath, "data", "registry")

	// Count YAML files
	yamlCount := 0
	err := filepath.Walk(registryDataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (filepath.Ext(path) == ".yml" || filepath.Ext(path) == ".yaml") {
			yamlCount++
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed to count registry files: %w", err)
	}

	ui.Logf("📊 Total YAML files: %d", yamlCount)

	// Show git info
	gitCmd := exec.Command("git", "-C", registryPath, "log", "-1", "--format=%H %s %ad", "--date=short")
	output, err := gitCmd.Output()
	if err == nil {
		ui.Logf("🔗 Git info: %s", string(output))
	}

	return nil
}

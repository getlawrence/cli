package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// ComposerInstaller installs PHP packages using composer or edits composer.json
type ComposerInstaller struct {
	commander types.Commander
}

// NewComposerInstaller creates a new composer installer
func NewComposerInstaller(commander types.Commander) Installer {
	return &ComposerInstaller{commander: commander}
}

// Install installs PHP dependencies
func (i *ComposerInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	composerPath := filepath.Join(projectPath, "composer.json")
	if _, err := os.Stat(composerPath); os.IsNotExist(err) {
		return fmt.Errorf("composer.json not found in %s", projectPath)
	}

	if dryRun {
		return nil
	}

	// Check if composer is available
	if _, err := i.commander.LookPath("composer"); err == nil {
		// Use composer require
		args := append([]string{"require"}, dependencies...)
		args = append(args, "--no-interaction")

		if out, err := i.commander.Run(ctx, "composer", args, projectPath); err != nil {
			return fmt.Errorf("composer require failed: %w\nOutput: %s", err, out)
		}
		return nil
	}

	// Fallback: edit composer.json directly
	return i.editComposerJSON(composerPath, dependencies)
}

// editComposerJSON adds dependencies to composer.json
func (i *ComposerInstaller) editComposerJSON(composerPath string, dependencies []string) error {
	content, err := os.ReadFile(composerPath)
	if err != nil {
		return err
	}

	var composer map[string]interface{}
	if err := json.Unmarshal(content, &composer); err != nil {
		return err
	}

	// Ensure require section exists
	require, ok := composer["require"].(map[string]interface{})
	if !ok {
		require = make(map[string]interface{})
		composer["require"] = require
	}

	// Add dependencies
	for _, dep := range dependencies {
		// For now, use * for version
		require[dep] = "*"
	}

	// Write back
	output, err := json.MarshalIndent(composer, "", "    ")
	if err != nil {
		return err
	}

	return os.WriteFile(composerPath, append(output, '\n'), 0644)
}

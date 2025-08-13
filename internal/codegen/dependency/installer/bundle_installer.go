package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// BundleInstaller installs Ruby gems using bundle or edits Gemfile
type BundleInstaller struct {
	commander types.Commander
}

// NewBundleInstaller creates a new bundle installer
func NewBundleInstaller(commander types.Commander) Installer {
	return &BundleInstaller{commander: commander}
}

// Install installs Ruby dependencies
func (i *BundleInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	gemfilePath := filepath.Join(projectPath, "Gemfile")
	if _, err := os.Stat(gemfilePath); os.IsNotExist(err) {
		return fmt.Errorf("Gemfile not found in %s", projectPath)
	}

	if dryRun {
		return nil
	}

	// Check if bundle is available
	if _, err := i.commander.LookPath("bundle"); err == nil {
		// Use bundle add
		for _, dep := range dependencies {
			args := []string{"add", dep}
			if out, err := i.commander.Run(ctx, "bundle", args, projectPath); err != nil {
				return fmt.Errorf("bundle add %s failed: %w\nOutput: %s", dep, err, out)
			}
		}
		return nil
	}

	// Fallback: edit Gemfile directly
	return i.editGemfile(gemfilePath, dependencies)
}

// editGemfile adds gem entries to Gemfile
func (i *BundleInstaller) editGemfile(gemfilePath string, dependencies []string) error {
	// Read existing content
	content, err := os.ReadFile(gemfilePath)
	if err != nil {
		return err
	}

	// Check which gems already exist
	contentStr := string(content)
	existing := make(map[string]bool)
	lines := strings.Split(contentStr, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "gem ") {
			// Extract gem name
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				gemName := strings.Trim(parts[1], `"'`)
				existing[gemName] = true
			}
		}
	}

	// Add new gems
	var toAdd []string
	for _, dep := range dependencies {
		if !existing[dep] {
			toAdd = append(toAdd, fmt.Sprintf("gem '%s'", dep))
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Append to file
	f, err := os.OpenFile(gemfilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure newline before additions
	if !strings.HasSuffix(contentStr, "\n") {
		if _, err := f.WriteString("\n"); err != nil {
			return err
		}
	}

	for _, gem := range toAdd {
		if _, err := f.WriteString(gem + "\n"); err != nil {
			return err
		}
	}

	return nil
}

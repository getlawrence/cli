package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// PipInstaller installs Python packages using pip or edits requirements.txt
type PipInstaller struct {
	commander types.Commander
}

// NewPipInstaller creates a new pip installer
func NewPipInstaller(commander types.Commander) Installer {
	return &PipInstaller{commander: commander}
}

// Install installs Python dependencies
func (i *PipInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	reqPath := filepath.Join(projectPath, "requirements.txt")

	// Resolve versions
	resolved, err := i.resolveVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	// Check if pip is available
	pipCmd := "pip"
	if _, err := i.commander.LookPath(pipCmd); err != nil {
		// Try pip3
		if _, err := i.commander.LookPath("pip3"); err == nil {
			pipCmd = "pip3"
		} else if _, err := i.commander.LookPath("python"); err == nil {
			// Try python -m pip
			pipCmd = "python"
		} else {
			// Fallback to editing requirements.txt
			return i.editRequirements(reqPath, resolved)
		}
	}

	// Use pip install
	for _, dep := range resolved {
		var args []string
		if pipCmd == "python" {
			args = []string{"-m", "pip", "install", "-U", dep}
		} else {
			args = []string{"install", "-U", dep}
		}

		if _, err := i.commander.Run(ctx, pipCmd, args, projectPath); err != nil {
			return i.updateRequirements(reqPath, resolved)
		}
	}

	// Update requirements.txt
	return i.updateRequirements(reqPath, resolved)
}

// resolveVersions adds versions to dependencies
func (i *PipInstaller) resolveVersions(ctx context.Context, projectPath string, deps []string) ([]string, error) {
	var resolved []string

	for _, dep := range deps {
		// Check if already has version specifier
		hasVersion := false
		for _, op := range []string{"==", ">=", "<=", ">", "<", "~=", "!="} {
			if strings.Contains(dep, op) {
				hasVersion = true
				break
			}
		}

		if hasVersion {
			resolved = append(resolved, dep)
			continue
		}

		// For now, use no version constraint (latest)
		resolved = append(resolved, dep)
	}

	return resolved, nil
}

// editRequirements adds dependencies to requirements.txt
func (i *PipInstaller) editRequirements(reqPath string, dependencies []string) error {
	// Read existing requirements
	existing := make(map[string]bool)
	if content, err := os.ReadFile(reqPath); err == nil {
		for _, line := range strings.Split(string(content), "\n") {
			line = strings.TrimSpace(line)
			if line != "" && !strings.HasPrefix(line, "#") {
				// Extract package name
				pkg := line
				for _, op := range []string{"==", ">=", "<=", ">", "<", "~=", "!="} {
					if idx := strings.Index(pkg, op); idx != -1 {
						pkg = pkg[:idx]
						break
					}
				}
				existing[strings.ToLower(strings.TrimSpace(pkg))] = true
			}
		}
	}

	// Append new dependencies
	var toAdd []string
	for _, dep := range dependencies {
		pkg := dep
		for _, op := range []string{"==", ">=", "<=", ">", "<", "~=", "!="} {
			if idx := strings.Index(pkg, op); idx != -1 {
				pkg = pkg[:idx]
				break
			}
		}

		if !existing[strings.ToLower(strings.TrimSpace(pkg))] {
			toAdd = append(toAdd, dep)
		}
	}

	if len(toAdd) == 0 {
		return nil
	}

	// Append to file
	f, err := os.OpenFile(reqPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	// Ensure we start on a new line if the file doesn't end with one
	if content, err := os.ReadFile(reqPath); err == nil && len(content) > 0 {
		// Check if the file ends with a newline
		if content[len(content)-1] != '\n' {
			if _, err := f.WriteString("\n"); err != nil {
				return err
			}
		}
	}

	for _, dep := range toAdd {
		if _, err := f.WriteString(dep + "\n"); err != nil {
			return err
		}
	}

	return nil
}

// updateRequirements updates requirements.txt after pip install
func (i *PipInstaller) updateRequirements(reqPath string, dependencies []string) error {
	// For now, just ensure they're in the file
	return i.editRequirements(reqPath, dependencies)
}

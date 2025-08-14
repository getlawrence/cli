package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// GoInstaller installs go modules using `go get` or edits go.mod
type GoInstaller struct {
	commander types.Commander
}

// NewGoInstaller creates a new Go installer
func NewGoInstaller(commander types.Commander) Installer {
	return &GoInstaller{commander: commander}
}

// Install installs Go dependencies
func (i *GoInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	goModPath := filepath.Join(projectPath, "go.mod")
	if _, err := os.Stat(goModPath); os.IsNotExist(err) {
		return fmt.Errorf("go.mod not found in %s", projectPath)
	}

	if dryRun {
		return nil // Caller handles logging
	}

	// Resolve versions for dependencies that need them
	resolved, err := i.resolveVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}

	// Check if go command is available
	if _, err := i.commander.LookPath("go"); err == nil {
		// Use go get
		for _, dep := range resolved {
			args := []string{"get", dep}
			if out, err := i.commander.Run(ctx, "go", args, projectPath); err != nil {
				return fmt.Errorf("go get %s failed: %w\nOutput: %s", dep, err, out)
			}
		}

		// Do not run as the otel init code has not been added yet and this will remove the new dependencies
		// Best-effort tidy
		// _, _ = i.commander.Run(ctx, "go", []string{"mod", "tidy"}, projectPath)
		return nil
	}

	// Fallback: edit go.mod directly
	return i.editGoMod(goModPath, resolved)
}

// resolveVersions adds versions to dependencies that need them
func (i *GoInstaller) resolveVersions(ctx context.Context, projectPath string, deps []string) ([]string, error) {
	var resolved []string

	for _, dep := range deps {
		// Check if dependency already has a version
		if strings.Contains(dep, "@") {
			resolved = append(resolved, dep)
			continue
		}

		// Check if path encodes version (e.g., semconv/v1.34.0)
		if hasPathEncodedVersion(dep) {
			resolved = append(resolved, dep)
			continue
		}

		// Try to resolve latest version
		version, err := i.resolveLatestVersion(ctx, projectPath, dep)
		if err != nil {
			// Use "latest" as fallback
			version = "latest"
		}

		resolved = append(resolved, dep+"@"+version)
	}

	return resolved, nil
}

// hasPathEncodedVersion checks if module path has version in last segment
func hasPathEncodedVersion(importPath string) bool {
	lastSlash := strings.LastIndex(importPath, "/")
	seg := importPath
	if lastSlash != -1 {
		seg = importPath[lastSlash+1:]
	}
	matched, _ := regexp.MatchString(`^v\d+\.\d+\.\d+$`, seg)
	return matched
}

// resolveLatestVersion attempts to find the latest version for a module
func (i *GoInstaller) resolveLatestVersion(ctx context.Context, projectPath string, module string) (string, error) {
	// Try go list if available
	if _, err := i.commander.LookPath("go"); err == nil {
		args := []string{"list", "-m", "-json", module + "@latest"}
		out, err := i.commander.Run(ctx, "go", args, projectPath)
		if err == nil {
			// Parse JSON to extract version
			if m := regexp.MustCompile(`"Version":\s*"([^"]+)"`).FindStringSubmatch(out); len(m) > 1 {
				return m[1], nil
			}
		}
	}

	return "", fmt.Errorf("could not resolve version for %s", module)
}

// editGoMod adds require entries to go.mod
func (i *GoInstaller) editGoMod(goModPath string, dependencies []string) error {
	content, err := os.ReadFile(goModPath)
	if err != nil {
		return err
	}

	// Parse dependencies to extract module and version
	var lines []string
	for _, dep := range dependencies {
		var module, version string

		// Handle path-encoded versions specially
		if hasPathEncodedVersion(dep) && !strings.Contains(dep, "@") {
			module = dep
			// Extract version from path
			lastSlash := strings.LastIndex(module, "/")
			if lastSlash != -1 {
				version = module[lastSlash+1:]
			}
		} else {
			parts := strings.Split(dep, "@")
			if len(parts) != 2 {
				return fmt.Errorf("invalid dependency format: %s", dep)
			}
			module, version = parts[0], parts[1]

			// Handle path-encoded versions with @latest
			if version == "latest" && hasPathEncodedVersion(module) {
				lastSlash := strings.LastIndex(module, "/")
				if lastSlash != -1 {
					version = module[lastSlash+1:]
				}
			}
		}

		lines = append(lines, fmt.Sprintf("\t%s %s", module, version))
	}

	// Insert into require block or create new one
	contentStr := string(content)
	if strings.Contains(contentStr, "require (") {
		// Find last closing parenthesis of require block
		idx := strings.LastIndex(contentStr, ")")
		if idx == -1 {
			return fmt.Errorf("malformed go.mod: missing closing parenthesis")
		}
		insert := "\n" + strings.Join(lines, "\n") + "\n"
		newContent := contentStr[:idx] + insert + contentStr[idx:]
		return os.WriteFile(goModPath, []byte(newContent), 0644)
	}

	// Append new require block
	var b strings.Builder
	b.WriteString(contentStr)
	if !strings.HasSuffix(contentStr, "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\nrequire (\n")
	b.WriteString(strings.Join(lines, "\n"))
	b.WriteString("\n)\n")

	return os.WriteFile(goModPath, []byte(b.String()), 0644)
}

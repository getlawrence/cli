package installer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// NpmInstaller installs npm packages using `npm install` or edits package.json
type NpmInstaller struct {
	commander types.Commander
}

// NewNpmInstaller creates a new npm installer
func NewNpmInstaller(commander types.Commander) Installer {
	return &NpmInstaller{commander: commander}
}

// Install installs npm dependencies
func (i *NpmInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	pkgPath := filepath.Join(projectPath, "package.json")
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		return fmt.Errorf("package.json not found in %s", projectPath)
	}

	// Resolve versions for dependencies
	resolved, err := i.resolveVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}

	if dryRun {
		return nil // Caller handles logging
	}

	// Check if npm command is available
	if _, err := i.commander.LookPath("npm"); err == nil {
		// Use npm install
		args := append([]string{"install"}, resolved...)
		if out, err := i.commander.Run(ctx, "npm", args, projectPath); err != nil {
			return fmt.Errorf("npm install failed: %w\nOutput: %s", err, out)
		}
		return nil
	}

	// Fallback: edit package.json directly
	return i.editPackageJSON(pkgPath, resolved)
}

// resolveVersions adds versions to dependencies that need them
func (i *NpmInstaller) resolveVersions(ctx context.Context, projectPath string, deps []string) ([]string, error) {
	var resolved []string

	for _, dep := range deps {
		// Check if dependency already has a version
		if strings.HasPrefix(dep, "@") {
			// Scoped package - check for version after last @
			idx := strings.LastIndex(dep, "@")
			if idx > 0 && idx < len(dep)-1 {
				// Already has version
				resolved = append(resolved, dep)
				continue
			}
		} else if strings.Contains(dep, "@") {
			// Regular package with version
			resolved = append(resolved, dep)
			continue
		}

		// Try to resolve latest version
		version, err := i.resolveLatestVersion(ctx, projectPath, dep)
		if err != nil {
			version = "latest"
		}

		resolved = append(resolved, dep+"@"+version)
	}

	return resolved, nil
}

// resolveLatestVersion attempts to find the latest version for a package
func (i *NpmInstaller) resolveLatestVersion(ctx context.Context, projectPath string, pkg string) (string, error) {
	// Try npm view if available
	if _, err := i.commander.LookPath("npm"); err == nil {
		args := []string{"view", pkg, "version", "--json"}
		out, err := i.commander.Run(ctx, "npm", args, projectPath)
		if err == nil {
			// Parse JSON response
			var version string
			if err := json.Unmarshal([]byte(out), &version); err == nil {
				return version, nil
			}
			// Fallback to plain text
			return strings.TrimSpace(out), nil
		}
	}

	return "", fmt.Errorf("could not resolve version for %s", pkg)
}

// editPackageJSON adds dependencies to package.json
func (i *NpmInstaller) editPackageJSON(pkgPath string, dependencies []string) error {
	content, err := os.ReadFile(pkgPath)
	if err != nil {
		return err
	}

	var pkg map[string]interface{}
	if err := json.Unmarshal(content, &pkg); err != nil {
		return err
	}

	// Ensure dependencies section exists
	deps, ok := pkg["dependencies"].(map[string]interface{})
	if !ok {
		deps = make(map[string]interface{})
		pkg["dependencies"] = deps
	}

	// Add new dependencies
	for _, dep := range dependencies {
		// Handle scoped packages like @opentelemetry/api@1.8.0
		var name, version string
		if strings.HasPrefix(dep, "@") {
			// Scoped package
			idx := strings.LastIndex(dep, "@")
			if idx <= 0 {
				return fmt.Errorf("invalid dependency format: %s", dep)
			}
			name = dep[:idx]
			version = dep[idx+1:]
		} else {
			// Regular package
			parts := strings.SplitN(dep, "@", 2)
			if len(parts) != 2 {
				return fmt.Errorf("invalid dependency format: %s", dep)
			}
			name, version = parts[0], parts[1]
		}
		deps[name] = version
	}

	// Write back to file
	output, err := json.MarshalIndent(pkg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(pkgPath, append(output, '\n'), 0644)
}

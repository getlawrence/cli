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

// DotNetInstaller installs .NET packages using dotnet CLI or edits .csproj
type DotNetInstaller struct {
	commander types.Commander
}

// NewDotNetInstaller creates a new .NET installer
func NewDotNetInstaller(commander types.Commander) Installer {
	return &DotNetInstaller{commander: commander}
}

// Install installs .NET dependencies
func (i *DotNetInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	// Find .csproj file
	csprojPath, err := i.findCsproj(projectPath)
	if err != nil {
		return err
	}

	// Resolve versions
	resolved, err := i.resolveVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	// Check if dotnet CLI is available
	if _, err := i.commander.LookPath("dotnet"); err == nil {
		// Use dotnet add package
		for _, dep := range resolved {
			parts := strings.Split(dep, "@")
			args := []string{"add", csprojPath, "package", parts[0]}
			if len(parts) > 1 && parts[1] != "" {
				args = append(args, "--version", parts[1])
			}

			if out, err := i.commander.Run(ctx, "dotnet", args, projectPath); err != nil {
				return fmt.Errorf("dotnet add package %s failed: %w\nOutput: %s", parts[0], err, out)
			}
		}
		return nil
	}

	// Fallback: edit .csproj directly
	return i.editCsproj(csprojPath, resolved)
}

// findCsproj finds the first .csproj file in the project
func (i *DotNetInstaller) findCsproj(projectPath string) (string, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", err
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csproj") {
			return filepath.Join(projectPath, entry.Name()), nil
		}
	}

	return "", fmt.Errorf("no .csproj file found in %s", projectPath)
}

// resolveVersions adds versions to dependencies
func (i *DotNetInstaller) resolveVersions(ctx context.Context, projectPath string, deps []string) ([]string, error) {
	var resolved []string

	for _, dep := range deps {
		// Check if already has version
		if strings.Contains(dep, "@") {
			resolved = append(resolved, dep)
			continue
		}

		// For now, use empty version (latest)
		resolved = append(resolved, dep+"@")
	}

	return resolved, nil
}

// editCsproj adds PackageReference entries to .csproj
func (i *DotNetInstaller) editCsproj(csprojPath string, dependencies []string) error {
	content, err := os.ReadFile(csprojPath)
	if err != nil {
		return err
	}

	// Get existing packages
	existing := make(map[string]bool)
	re := regexp.MustCompile(`(?i)<PackageReference\s+Include="([^"]+)"`)
	for _, match := range re.FindAllStringSubmatch(string(content), -1) {
		if len(match) > 1 {
			existing[strings.ToLower(match[1])] = true
		}
	}

	// Build new PackageReference lines
	var lines []string
	for _, dep := range dependencies {
		parts := strings.Split(dep, "@")
		pkgName := parts[0]
		version := "*"
		if len(parts) > 1 && parts[1] != "" {
			version = parts[1]
		}

		if !existing[strings.ToLower(pkgName)] {
			lines = append(lines, fmt.Sprintf("    <PackageReference Include=\"%s\" Version=\"%s\" />", pkgName, version))
		}
	}

	if len(lines) == 0 {
		return nil
	}

	// Insert into .csproj
	contentStr := string(content)
	itemGroup := "  <ItemGroup>\n" + strings.Join(lines, "\n") + "\n  </ItemGroup>\n"

	// Find insertion point (before </Project>)
	idx := strings.LastIndex(strings.ToLower(contentStr), "</project>")
	if idx == -1 {
		return fmt.Errorf("malformed .csproj: missing </Project>")
	}

	newContent := contentStr[:idx] + itemGroup + contentStr[idx:]
	return os.WriteFile(csprojPath, []byte(newContent), 0644)
}

package dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/getlawrence/cli/internal/logger"
)

// PHPInjector implements DependencyHandler for PHP projects (Composer)
type PHPInjector struct {
	logger logger.Logger
}

// NewPHPInjector creates a new PHP dependency handler
func NewPHPInjector(logger logger.Logger) *PHPInjector {
	return &PHPInjector{logger: logger}
}

// GetLanguage returns the language this handler supports
func (h *PHPInjector) GetLanguage() string { return "php" }

// AddDependencies adds the specified dependencies to composer.json (creates it if missing)
func (h *PHPInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}
	// Resolve explicit versions for all composer packages
	deps, err := h.resolveLatestVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}

	composerPath := filepath.Join(projectPath, "composer.json")

	// Load or initialize composer.json
	composer := make(map[string]any)
	if _, err := os.Stat(composerPath); err == nil {
		content, rerr := os.ReadFile(composerPath)
		if rerr != nil {
			return fmt.Errorf("failed to read composer.json: %w", rerr)
		}
		if len(content) > 0 {
			if jerr := json.Unmarshal(content, &composer); jerr != nil {
				return fmt.Errorf("failed to parse composer.json: %w", jerr)
			}
		}
	} else {
		composer["name"] = "getlawrence/generated-php"
		composer["type"] = "project"
	}

	// Ensure require map exists
	requireAny, ok := composer["require"].(map[string]any)
	if !ok {
		requireAny = make(map[string]any)
		composer["require"] = requireAny
	}

	// Determine which dependencies are new
	var toAdd []Dependency
	existing := make(map[string]bool)
	for k := range requireAny {
		existing[strings.ToLower(k)] = true
	}
	for _, dep := range deps {
		key := strings.ToLower(dep.ImportPath)
		if key == "" {
			continue
		}
		if !existing[key] {
			toAdd = append(toAdd, dep)
		}
	}
	if len(toAdd) == 0 {
		return nil
	}

	if dryRun {
		h.logger.Logf("Would add the following PHP dependencies to composer.json in %s:\n", projectPath)
		for _, dep := range toAdd {
			h.logger.Logf("  - %s: %s\n", dep.ImportPath, dep.Version)
		}
		return nil
	}

	// Add into require map
	for _, dep := range toAdd {
		requireAny[dep.ImportPath] = dep.Version
	}

	// Write back pretty-printed JSON with stable key order
	// Rebuild require map in sorted order for nicer diffs
	ordered := make(map[string]any)
	for k, v := range composer {
		if k != "require" {
			ordered[k] = v
		}
	}
	reqKeys := make([]string, 0, len(requireAny))
	for k := range requireAny {
		reqKeys = append(reqKeys, k)
	}
	sort.Strings(reqKeys)
	orderedRequire := make(map[string]any)
	for _, k := range reqKeys {
		orderedRequire[k] = requireAny[k]
	}
	ordered["require"] = orderedRequire

	output, werr := json.MarshalIndent(ordered, "", "  ")
	if werr != nil {
		return fmt.Errorf("failed to serialize composer.json: %w", werr)
	}
	output = append(output, '\n')
	if err := os.WriteFile(composerPath, output, 0644); err != nil {
		return fmt.Errorf("failed to write composer.json: %w", err)
	}
	h.logger.Logf("Updated %s with %d dependencies\n", composerPath, len(toAdd))
	return nil
}

// resolveLatestVersions queries composer to resolve latest versions for any missing versions
func (h *PHPInjector) resolveLatestVersions(ctx context.Context, projectPath string, deps []Dependency) ([]Dependency, error) {
	resolved := make([]Dependency, 0, len(deps))
	for _, d := range deps {
		if strings.TrimSpace(d.Version) != "" {
			resolved = append(resolved, d)
			continue
		}
		// Use: composer show <package> --all --no-interaction
		cmd := exec.CommandContext(ctx, "composer", "show", d.ImportPath, "--all", "--no-interaction")
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			// If composer is not available on host, fallback to default version constraint that will allow docker build to resolve latest
			// We still need an explicit version; fallback to "*" is not allowed per requirement, so return an error
			return nil, fmt.Errorf("failed to resolve latest version for %s: %w", d.ImportPath, err)
		}
		latest := ""
		lines := strings.Split(string(out), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(strings.ToLower(line), "versions :") || strings.HasPrefix(strings.ToLower(line), "versions:") {
				// e.g., versions : * 1.0.2, 1.0.1
				// strip until '*' and take token after it
				idx := strings.Index(line, "*")
				if idx != -1 && idx+1 < len(line) {
					rest := strings.TrimSpace(line[idx+1:])
					parts := strings.Split(rest, ",")
					if len(parts) > 0 {
						latest = strings.TrimSpace(parts[0])
					}
				}
				break
			}
		}
		if latest == "" {
			return nil, fmt.Errorf("could not parse latest version for %s from composer output", d.ImportPath)
		}
		d.Version = latest
		resolved = append(resolved, d)
	}
	return resolved, nil
}

// GetCoreDependencies returns essential OpenTelemetry packages for PHP
func (h *PHPInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{
			Name:        "OpenTelemetry SDK",
			Language:    "php",
			ImportPath:  "open-telemetry/opentelemetry",
			Category:    "core",
			Description: "OpenTelemetry PHP SDK",
			Required:    true,
		},
	}
}

// GetInstrumentationDependency returns instrumentation dependency for a specific package (none for minimal)
func (h *PHPInjector) GetInstrumentationDependency(instrumentation string) *Dependency { return nil }

// GetComponentDependency returns component dependencies (none for minimal)
func (h *PHPInjector) GetComponentDependency(componentType, component string) *Dependency { return nil }

// ResolveInstrumentationPrerequisites for PHP currently returns the list unchanged.
func (h *PHPInjector) ResolveInstrumentationPrerequisites(instrumentations []string) []string {
	return instrumentations
}

// ValidateProjectStructure checks for composer.json presence and warns if missing
func (h *PHPInjector) ValidateProjectStructure(projectPath string) error {
	composerPath := filepath.Join(projectPath, "composer.json")
	if _, err := os.Stat(composerPath); os.IsNotExist(err) {
		h.logger.Logf("No composer.json found in %s, will create one if needed.\n", projectPath)
	}
	return nil
}

// GetDependencyFiles returns paths to dependency files
func (h *PHPInjector) GetDependencyFiles(projectPath string) []string {
	path := filepath.Join(projectPath, "composer.json")
	if _, err := os.Stat(path); err == nil {
		return []string{path}
	}
	return []string{}
}

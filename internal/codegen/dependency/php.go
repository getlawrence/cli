package dependency

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// PHPInjector implements DependencyHandler for PHP projects (Composer)
type PHPInjector struct{}

// NewPHPInjector creates a new PHP dependency handler
func NewPHPInjector() *PHPInjector { return &PHPInjector{} }

// GetLanguage returns the language this handler supports
func (h *PHPInjector) GetLanguage() string { return "php" }

// AddDependencies adds the specified dependencies to composer.json (creates it if missing)
func (h *PHPInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
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
	for _, dep := range dependencies {
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
		fmt.Printf("Would add the following PHP dependencies to composer.json in %s:\n", projectPath)
		for _, dep := range toAdd {
			version := dep.Version
			if version == "" {
				version = "*"
			}
			fmt.Printf("  - %s: %s\n", dep.ImportPath, version)
		}
		return nil
	}

	// Add into require map
	for _, dep := range toAdd {
		version := dep.Version
		if version == "" {
			version = "*"
		}
		requireAny[dep.ImportPath] = version
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
	fmt.Printf("Updated %s with %d dependencies\n", composerPath, len(toAdd))
	return nil
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

// ValidateProjectStructure checks for composer.json presence and warns if missing
func (h *PHPInjector) ValidateProjectStructure(projectPath string) error {
	composerPath := filepath.Join(projectPath, "composer.json")
	if _, err := os.Stat(composerPath); os.IsNotExist(err) {
		fmt.Printf("No composer.json found in %s, will create one if needed.\n", projectPath)
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

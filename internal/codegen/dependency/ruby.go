package dependency

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// RubyHandler implements DependencyHandler for Ruby projects (Bundler)
type RubyHandler struct{}

// NewRubyHandler creates a new Ruby dependency handler
func NewRubyHandler() *RubyHandler { return &RubyHandler{} }

// GetLanguage returns the language this handler supports
func (h *RubyHandler) GetLanguage() string { return "ruby" }

// AddDependencies adds the specified dependencies using Bundler
func (h *RubyHandler) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	gemfile := filepath.Join(projectPath, "Gemfile")
	if _, err := os.Stat(gemfile); os.IsNotExist(err) {
		return fmt.Errorf("Gemfile not found in %s", projectPath)
	}

	if len(dependencies) == 0 {
		return nil
	}

	// Add gems to Gemfile if they are not already present
	needed, err := h.filterExistingDependencies(gemfile, dependencies)
	if err != nil {
		return err
	}
	if len(needed) == 0 {
		return nil
	}

	if dryRun {
		fmt.Printf("Would add the following Ruby gems to %s and run bundle install:\n", gemfile)
		for _, dep := range needed {
			if dep.Version != "" {
				fmt.Printf("  - gem '%s', '%s'\n", dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  - gem '%s'\n", dep.ImportPath)
			}
		}
		return nil
	}

	if err := h.appendGemsToGemfile(gemfile, needed); err != nil {
		return err
	}

	// Run bundle install
	cmd := exec.CommandContext(ctx, "bundle", "install")
	cmd.Dir = projectPath
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to run bundle install: %w\nOutput: %s", err, string(output))
	}
	return nil
}

// GetCoreDependencies returns the core OpenTelemetry dependencies for Ruby
func (h *RubyHandler) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry API", Language: "ruby", ImportPath: "opentelemetry-api", Category: "core", Required: true},
		{Name: "OpenTelemetry SDK", Language: "ruby", ImportPath: "opentelemetry-sdk", Category: "core", Required: true},
		{Name: "OTLP Exporter", Language: "ruby", ImportPath: "opentelemetry-exporter-otlp", Category: "exporter", Required: true},
	}
}

// GetInstrumentationDependency maps common instrumentation names to Ruby gems
func (h *RubyHandler) GetInstrumentationDependency(instrumentation string) *Dependency {
	m := map[string]Dependency{
		"rails":   {Name: "Rails Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-rails", Category: "instrumentation"},
		"rack":    {Name: "Rack Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-rack", Category: "instrumentation"},
		"http":    {Name: "HTTP (Net::HTTP) Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-net_http", Category: "instrumentation"},
		"pg":      {Name: "Postgres (pg) Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-pg", Category: "instrumentation"},
		"mysql2":  {Name: "MySQL2 Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-mysql2", Category: "instrumentation"},
		"redis":   {Name: "Redis Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-redis", Category: "instrumentation"},
		"sidekiq": {Name: "Sidekiq Instrumentation", Language: "ruby", ImportPath: "opentelemetry-instrumentation-sidekiq", Category: "instrumentation"},
	}
	if dep, ok := m[instrumentation]; ok {
		return &dep
	}
	return nil
}

// GetComponentDependency returns exporter/propagator components if needed
func (h *RubyHandler) GetComponentDependency(componentType, component string) *Dependency { return nil }

// ValidateProjectStructure checks for Gemfile
func (h *RubyHandler) ValidateProjectStructure(projectPath string) error {
	if _, err := os.Stat(filepath.Join(projectPath, "Gemfile")); err != nil {
		return fmt.Errorf("Gemfile not found in %s", projectPath)
	}
	return nil
}

// GetDependencyFiles returns Ruby dependency files
func (h *RubyHandler) GetDependencyFiles(projectPath string) []string {
	return []string{filepath.Join(projectPath, "Gemfile"), filepath.Join(projectPath, "Gemfile.lock")}
}

// filterExistingDependencies filters out gems already in Gemfile
func (h *RubyHandler) filterExistingDependencies(gemfile string, dependencies []Dependency) ([]Dependency, error) {
	existing, err := h.getExistingGems(gemfile)
	if err != nil {
		return nil, err
	}
	var needed []Dependency
	for _, dep := range dependencies {
		if !existing[dep.ImportPath] {
			needed = append(needed, dep)
		}
	}
	return needed, nil
}

func (h *RubyHandler) getExistingGems(gemfile string) (map[string]bool, error) {
	file, err := os.Open(gemfile)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	gems := make(map[string]bool)
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`gem\s+["']([^"']+)["']`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			gems[m[1]] = true
		}
	}
	return gems, scanner.Err()
}

func (h *RubyHandler) appendGemsToGemfile(gemfile string, dependencies []Dependency) error {
	f, err := os.OpenFile(gemfile, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()
	for _, dep := range dependencies {
		line := fmt.Sprintf("\n# Added by lawrence\n")
		if _, err := f.WriteString(line); err != nil {
			return err
		}
		if dep.Version != "" {
			if _, err := f.WriteString(fmt.Sprintf("gem '%s', '%s'\n", dep.ImportPath, dep.Version)); err != nil {
				return err
			}
		} else {
			if _, err := f.WriteString(fmt.Sprintf("gem '%s'\n", dep.ImportPath)); err != nil {
				return err
			}
		}
	}
	return nil
}

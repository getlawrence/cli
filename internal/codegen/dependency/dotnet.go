package dependency

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/logger"
)

// DotNetInjector implements DependencyHandler for .NET projects
type DotNetInjector struct {
	logger logger.Logger
}

func NewDotNetInjector(logger logger.Logger) *DotNetInjector {
	return &DotNetInjector{logger: logger}
}

func (h *DotNetInjector) GetLanguage() string { return "csharp" }

func (h *DotNetInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}
	// Resolve NuGet package versions if missing
	deps, err := h.resolveLatestVersions(ctx, projectPath, dependencies)
	if err != nil {
		return err
	}
	// Prefer direct csproj edits to avoid requiring dotnet CLI on host
	csproj, err := h.findCsproj(projectPath)
	if err != nil {
		return fmt.Errorf("no .csproj found in %s: %w", projectPath, err)
	}

	if dryRun {
		h.logger.Logf("Would add PackageReference entries to %s:\n", filepath.Base(csproj))
		for _, dep := range deps {
			h.logger.Logf("  <PackageReference Include=\"%s\" Version=\"%s\" />\n", dep.ImportPath, dep.Version)
		}
		return nil
	}

	if err := h.editCsprojAddPackages(csproj, deps); err != nil {
		// Fallback: try dotnet CLI if available
		for _, dep := range deps {
			args := []string{"add", csproj, "package", dep.ImportPath}
			if dep.Version != "" {
				args = append(args, "--version", dep.Version)
			}
			cmd := exec.CommandContext(ctx, "dotnet", args...)
			cmd.Dir = projectPath
			if out, derr := cmd.CombinedOutput(); derr != nil {
				return fmt.Errorf("failed to modify csproj and dotnet add also failed: %w\nOutput: %s", err, string(out))
			}
		}
	}
	return nil
}

func (h *DotNetInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry", Language: "csharp", ImportPath: "OpenTelemetry", Category: "core", Required: true},
		{Name: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Language: "csharp", ImportPath: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Category: "exporter", Required: true},
		{Name: "OpenTelemetry.Extensions.Hosting", Language: "csharp", ImportPath: "OpenTelemetry.Extensions.Hosting", Category: "core", Required: true},
		{Name: "OpenTelemetry.Instrumentation.AspNetCore", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.AspNetCore", Category: "instrumentation", Required: true},
		{Name: "OpenTelemetry.Instrumentation.Http", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Http", Category: "instrumentation", Required: true},
		{Name: "OpenTelemetry.Instrumentation.Runtime", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Runtime", Category: "instrumentation", Required: true},
		// Process instrumentation is pre-release; skip to avoid NuGet restore failures
	}
}

func (h *DotNetInjector) GetInstrumentationDependency(instrumentation string) *Dependency {
	m := map[string]Dependency{
		"aspnetcore": {Name: "ASP.NET Core", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.AspNetCore", Category: "instrumentation"},
		"httpclient": {Name: "HttpClient", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Http", Category: "instrumentation"},
		// grpc and redis instrumentations are optional; not pulled by default templates
		"runtime": {Name: "Runtime", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Runtime", Category: "instrumentation"},
	}
	if dep, ok := m[instrumentation]; ok {
		return &dep
	}
	return nil
}

func (h *DotNetInjector) GetComponentDependency(componentType, component string) *Dependency {
	switch componentType {
	case "instrumentation":
		// no auto instrumentation component support
	}
	return nil
}

// ResolveInstrumentationPrerequisites for .NET currently returns the list unchanged.
func (h *DotNetInjector) ResolveInstrumentationPrerequisites(instrumentations []string) []string {
	return instrumentations
}

func (h *DotNetInjector) ValidateProjectStructure(projectPath string) error {
	if _, err := h.findCsproj(projectPath); err != nil {
		return fmt.Errorf(".csproj not found in %s", projectPath)
	}
	return nil
}

func (h *DotNetInjector) GetDependencyFiles(projectPath string) []string {
	if csproj, err := h.findCsproj(projectPath); err == nil {
		return []string{csproj}
	}
	return nil
}

func (h *DotNetInjector) findCsproj(projectPath string) (string, error) {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".csproj" {
			return filepath.Join(projectPath, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .csproj found")
}

// Minimal XML types for csproj
type csprojProject struct {
	XMLName    xml.Name          `xml:"Project"`
	ItemGroups []csprojItemGroup `xml:"ItemGroup"`
}

// csprojItemGroup is kept for future XML binding; currently unused
//
//nolint:unused // kept for future XML binding
type csprojItemGroup struct{}

type csprojPackageReference struct{}

func (h *DotNetInjector) editCsprojAddPackages(csprojPath string, deps []Dependency) error {
	raw, err := os.ReadFile(csprojPath)
	if err != nil {
		return err
	}
	content := string(raw)
	// Collect existing package includes via regex
	existing := make(map[string]bool)
	re := regexp.MustCompile(`(?i)<PackageReference\s+Include=\"([^\"]+)\"`)
	for _, m := range re.FindAllStringSubmatch(content, -1) {
		if len(m) > 1 {
			existing[strings.ToLower(m[1])] = true
		}
	}
	// Build new PackageReference lines for missing packages
	var b strings.Builder
	for _, d := range deps {
		key := strings.ToLower(d.ImportPath)
		if existing[key] {
			continue
		}
		ver := d.Version
		if ver == "" {
			ver = "*"
		}
		b.WriteString(fmt.Sprintf("    <PackageReference Include=\"%s\" Version=\"%s\" />\n", d.ImportPath, ver))
	}
	lines := b.String()
	if lines == "" {
		return nil
	}
	// Insert an ItemGroup before closing </Project>
	itemGroup := "  <ItemGroup>\n" + lines + "  </ItemGroup>\n"
	if idx := strings.LastIndex(strings.ToLower(content), strings.ToLower("</Project>")); idx != -1 {
		newContent := content[:idx] + itemGroup + content[idx:]
		return os.WriteFile(csprojPath, []byte(newContent), 0644)
	}
	// Fallback to naive append creating minimal project wrapper
	return h.naiveAppendPackageRefs(csprojPath, content, deps)
}

func (h *DotNetInjector) naiveAppendPackageRefs(path, original string, deps []Dependency) error {
	var b strings.Builder
	b.WriteString("  <ItemGroup>\n")
	for _, d := range deps {
		b.WriteString(fmt.Sprintf("    <PackageReference Include=\"%s\" Version=\"%s\" />\n", d.ImportPath, d.Version))
	}
	b.WriteString("  </ItemGroup>\n")
	insert := b.String()
	if idx := strings.LastIndex(original, "</Project>"); idx != -1 {
		newContent := original[:idx] + insert + original[idx:]
		return os.WriteFile(path, []byte(newContent), 0644)
	}
	// fallback minimal
	minimal := fmt.Sprintf("<Project Sdk=\"Microsoft.NET.Sdk\">\n%s</Project>\n", insert)
	return os.WriteFile(path, []byte(minimal), 0644)
}

// resolveLatestVersions determines latest version for each NuGet package when not provided
func (h *DotNetInjector) resolveLatestVersions(ctx context.Context, projectPath string, deps []Dependency) ([]Dependency, error) {
	resolved := make([]Dependency, 0, len(deps))
	for _, d := range deps {
		if strings.TrimSpace(d.Version) != "" {
			resolved = append(resolved, d)
			continue
		}
		// Use: dotnet list package --outdated won't work without project context; instead use `dotnet nuget list source`+`dotnet tool` is heavy.
		// Prefer: `dotnet package search <id> --prerelease false` is not available everywhere; fallback to NuGet CLI if present.
		// Use `dotnet nuget search <id> --take 1 --format short` (available in newer SDKs). If fails, error out.
		cmd := exec.CommandContext(ctx, "dotnet", "nuget", "search", d.ImportPath, "--take", "1", "--format", "short")
		cmd.Dir = projectPath
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("failed to resolve latest version for %s: %w\nOutput: %s", d.ImportPath, err, string(out))
		}
		// Output example: OpenTelemetry 1.8.1
		fields := strings.Fields(string(out))
		if len(fields) >= 2 {
			d.Version = strings.TrimSpace(fields[len(fields)-1])
		}
		if strings.TrimSpace(d.Version) == "" {
			return nil, fmt.Errorf("could not parse latest version for %s from dotnet output: %s", d.ImportPath, string(out))
		}
		resolved = append(resolved, d)
	}
	return resolved, nil
}

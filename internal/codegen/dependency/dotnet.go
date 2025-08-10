package dependency

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DotNetInjector implements DependencyHandler for .NET projects
type DotNetInjector struct{}

func NewDotNetInjector() *DotNetInjector { return &DotNetInjector{} }

func (h *DotNetInjector) GetLanguage() string { return "csharp" }

func (h *DotNetInjector) AddDependencies(ctx context.Context, projectPath string, dependencies []Dependency, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}
	// Best-effort: run `dotnet add package` for each dependency when a .csproj exists in the folder.
	csproj, err := h.findCsproj(projectPath)
	if err != nil {
		return fmt.Errorf("no .csproj found in %s: %w", projectPath, err)
	}

	if dryRun {
		fmt.Printf("Would run dotnet add package in %s for:\n", projectPath)
		for _, dep := range dependencies {
			if dep.Version != "" {
				fmt.Printf("  dotnet add %s package %s --version %s\n", filepath.Base(csproj), dep.ImportPath, dep.Version)
			} else {
				fmt.Printf("  dotnet add %s package %s\n", filepath.Base(csproj), dep.ImportPath)
			}
		}
		return nil
	}

	for _, dep := range dependencies {
		args := []string{"add", csproj, "package", dep.ImportPath}
		if dep.Version != "" {
			args = append(args, "--version", dep.Version)
		}
		cmd := exec.CommandContext(ctx, "dotnet", args...)
		cmd.Dir = projectPath
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("failed: dotnet %v\nError: %w\nOutput: %s", args, err, string(out))
		}
	}
	return nil
}

func (h *DotNetInjector) GetCoreDependencies() []Dependency {
	return []Dependency{
		{Name: "OpenTelemetry", Language: "csharp", ImportPath: "OpenTelemetry", Category: "core", Required: true},
		{Name: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Language: "csharp", ImportPath: "OpenTelemetry.Exporter.OpenTelemetryProtocol", Category: "exporter", Required: true},
		{Name: "OpenTelemetry.Extensions.Hosting", Language: "csharp", ImportPath: "OpenTelemetry.Extensions.Hosting", Category: "core", Required: true},
	}
}

func (h *DotNetInjector) GetInstrumentationDependency(instrumentation string) *Dependency {
	m := map[string]Dependency{
		"aspnetcore": {Name: "ASP.NET Core", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.AspNetCore", Category: "instrumentation"},
		"httpclient": {Name: "HttpClient", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Http", Category: "instrumentation"},
		"grpc":       {Name: "gRPC", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.GrpcNetClient", Category: "instrumentation"},
		"redis":      {Name: "Redis", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.StackExchangeRedis", Category: "instrumentation"},
		"runtime":    {Name: "Runtime", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Runtime", Category: "instrumentation"},
		"process":    {Name: "Process", Language: "csharp", ImportPath: "OpenTelemetry.Instrumentation.Process", Category: "instrumentation"},
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

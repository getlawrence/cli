package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
)

func TestDotNetInstaller_EditCsproj(t *testing.T) {
	// Create a temporary directory
	tmpDir := t.TempDir()

	// Create a test .csproj file
	csprojContent := `<Project Sdk="Microsoft.NET.Sdk.Web">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
    <Nullable>enable</Nullable>
    <ImplicitUsings>enable</ImplicitUsings>
  </PropertyGroup>
</Project>`

	csprojPath := filepath.Join(tmpDir, "test.csproj")
	if err := os.WriteFile(csprojPath, []byte(csprojContent), 0644); err != nil {
		t.Fatalf("Failed to create test .csproj file: %v", err)
	}

	// Create installer with mock commander
	commander := &commander.Mock{}
	installer := NewDotNetInstaller(commander)

	// Test adding dependencies
	dependencies := []string{"OpenTelemetry", "OpenTelemetry.Sdk"}

	if err := installer.Install(context.Background(), tmpDir, dependencies, false); err != nil {
		t.Fatalf("Failed to install dependencies: %v", err)
	}

	// Read the modified .csproj file
	content, err := os.ReadFile(csprojPath)
	if err != nil {
		t.Fatalf("Failed to read modified .csproj file: %v", err)
	}

	contentStr := string(content)

	// Check that the dependencies were added
	if !strings.Contains(contentStr, `<PackageReference Include="OpenTelemetry"`) {
		t.Error("OpenTelemetry package reference not found in .csproj")
	}

	if !strings.Contains(contentStr, `<PackageReference Include="OpenTelemetry.Sdk"`) {
		t.Error("OpenTelemetry.Sdk package reference not found in .csproj")
	}

	t.Logf("Modified .csproj content:\n%s", contentStr)
}

package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
)

func TestGoInstaller(t *testing.T) {
	ctx := context.Background()

	t.Run("with go command available", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["go"] = true
		mock.Responses["go list -m -json go.opentelemetry.io/otel@latest"] = `{"Version": "v1.24.0"}`

		installer := NewGoInstaller(mock)

		// Create test project
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"go.opentelemetry.io/otel"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that go get was called
		if len(mock.RecordedCalls) < 2 {
			t.Fatal("Expected at least two command calls (list + get)")
		}

		// First call should be go list to resolve version
		listCall := mock.RecordedCalls[0]
		if listCall.Name != "go" || listCall.Args[0] != "list" {
			t.Errorf("Expected 'go list', got %s %v", listCall.Name, listCall.Args)
		}

		// Second call should be go get
		getCall := mock.RecordedCalls[1]
		if getCall.Name != "go" || getCall.Args[0] != "get" {
			t.Errorf("Expected 'go get', got %s %v", getCall.Name, getCall.Args)
		}
		if !strings.Contains(getCall.Args[1], "go.opentelemetry.io/otel@") {
			t.Errorf("Expected module with version, got %s", getCall.Args[1])
		}
	})

	t.Run("without go command - edit go.mod", func(t *testing.T) {
		mock := commander.NewMock()
		// go command not available

		installer := NewGoInstaller(mock)

		// Create test project
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"go.opentelemetry.io/otel@v1.24.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that go.mod was edited
		content, err := os.ReadFile(goModPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(content), "require") {
			t.Error("Expected require block in go.mod")
		}
		if !strings.Contains(string(content), "go.opentelemetry.io/otel v1.24.0") {
			t.Error("Expected dependency to be added to go.mod")
		}
	})

	t.Run("path-encoded version", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewGoInstaller(mock)

		// Create test project
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatal(err)
		}

		// Dependency with version in path
		deps := []string{"go.opentelemetry.io/otel/semconv/v1.34.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that go.mod was edited with correct version
		content, err := os.ReadFile(goModPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(content), "go.opentelemetry.io/otel/semconv/v1.34.0 v1.34.0") {
			t.Error("Expected path-encoded version to be handled correctly")
		}
	})

	t.Run("dry run", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewGoInstaller(mock)

		// Create test project
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		originalContent := "module test\n\ngo 1.21\n"
		if err := os.WriteFile(goModPath, []byte(originalContent), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"go.opentelemetry.io/otel"}
		err := installer.Install(ctx, dir, deps, true) // dry run
		if err != nil {
			t.Fatal(err)
		}

		// Check that no commands were executed
		if len(mock.RecordedCalls) != 0 {
			t.Error("Expected no commands in dry run")
		}

		// Check that go.mod was not modified
		content, err := os.ReadFile(goModPath)
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != originalContent {
			t.Error("go.mod should not be modified in dry run")
		}
	})
}

func TestNpmInstaller(t *testing.T) {
	ctx := context.Background()

	t.Run("with npm command available", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["npm"] = true
		mock.Responses["npm view @opentelemetry/api version --json"] = `"1.8.0"`

		installer := NewNpmInstaller(mock)

		// Create test project
		dir := t.TempDir()
		pkgPath := filepath.Join(dir, "package.json")
		if err := os.WriteFile(pkgPath, []byte(`{"name":"test","version":"1.0.0"}`), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"@opentelemetry/api"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that npm install was called
		if len(mock.RecordedCalls) < 2 {
			t.Fatal("Expected at least two command calls")
		}

		// First call should be npm view
		viewCall := mock.RecordedCalls[0]
		if viewCall.Name != "npm" || viewCall.Args[0] != "view" {
			t.Errorf("Expected 'npm view', got %s %v", viewCall.Name, viewCall.Args)
		}

		// Second call should be npm install
		installCall := mock.RecordedCalls[1]
		if installCall.Name != "npm" || installCall.Args[0] != "install" {
			t.Errorf("Expected 'npm install', got %s %v", installCall.Name, installCall.Args)
		}
		if !strings.Contains(installCall.Args[1], "@opentelemetry/api@") {
			t.Errorf("Expected package with version, got %s", installCall.Args[1])
		}
	})

	t.Run("without npm command - edit package.json", func(t *testing.T) {
		mock := commander.NewMock()
		// npm command not available

		installer := NewNpmInstaller(mock)

		// Create test project
		dir := t.TempDir()
		pkgPath := filepath.Join(dir, "package.json")
		if err := os.WriteFile(pkgPath, []byte(`{"name":"test","version":"1.0.0"}`), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"@opentelemetry/api@1.8.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that package.json was edited
		content, err := os.ReadFile(pkgPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(content), `"dependencies"`) {
			t.Error("Expected dependencies section in package.json")
		}
		if !strings.Contains(string(content), `"@opentelemetry/api"`) || !strings.Contains(string(content), `"1.8.0"`) {
			t.Errorf("Expected dependency to be added to package.json, got: %s", string(content))
		}
	})
}

func TestPipInstaller(t *testing.T) {
	ctx := context.Background()

	t.Run("with pip command available", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["pip"] = true

		installer := NewPipInstaller(mock)

		// Create test project
		dir := t.TempDir()
		reqPath := filepath.Join(dir, "requirements.txt")
		if err := os.WriteFile(reqPath, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"opentelemetry-api"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that pip install was called
		if len(mock.RecordedCalls) < 1 {
			t.Fatal("Expected at least one command call")
		}

		installCall := mock.RecordedCalls[0]
		if installCall.Name != "pip" || installCall.Args[0] != "install" {
			t.Errorf("Expected 'pip install', got %s %v", installCall.Name, installCall.Args)
		}
	})

	t.Run("without pip - edit requirements.txt", func(t *testing.T) {
		mock := commander.NewMock()
		// No pip/python available

		installer := NewPipInstaller(mock)

		// Create test project
		dir := t.TempDir()
		reqPath := filepath.Join(dir, "requirements.txt")
		if err := os.WriteFile(reqPath, []byte("flask==2.0.0\n"), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"opentelemetry-api", "opentelemetry-sdk"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that requirements.txt was edited
		content, err := os.ReadFile(reqPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(content), "opentelemetry-api") {
			t.Error("Expected opentelemetry-api in requirements.txt")
		}
		if !strings.Contains(string(content), "opentelemetry-sdk") {
			t.Error("Expected opentelemetry-sdk in requirements.txt")
		}
		// Original content should still be there
		if !strings.Contains(string(content), "flask==2.0.0") {
			t.Error("Expected original flask dependency to remain")
		}
	})
}

func TestDotNetInstaller(t *testing.T) {
	ctx := context.Background()

	t.Run("with dotnet CLI available", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["dotnet"] = true

		installer := NewDotNetInstaller(mock)

		// Create test project
		dir := t.TempDir()
		csprojPath := filepath.Join(dir, "test.csproj")
		if err := os.WriteFile(csprojPath, []byte(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"OpenTelemetry"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that dotnet add package was called
		if len(mock.RecordedCalls) < 1 {
			t.Fatal("Expected at least one command call")
		}

		addCall := mock.RecordedCalls[0]
		if addCall.Name != "dotnet" || addCall.Args[0] != "add" {
			t.Errorf("Expected 'dotnet add', got %s %v", addCall.Name, addCall.Args)
		}
		if addCall.Args[2] != "package" || addCall.Args[3] != "OpenTelemetry" {
			t.Errorf("Expected 'package OpenTelemetry', got %v", addCall.Args)
		}
	})

	t.Run("without dotnet CLI - edit csproj", func(t *testing.T) {
		mock := commander.NewMock()
		// dotnet command not available

		installer := NewDotNetInstaller(mock)

		// Create test project
		dir := t.TempDir()
		csprojPath := filepath.Join(dir, "test.csproj")
		if err := os.WriteFile(csprojPath, []byte(`<Project Sdk="Microsoft.NET.Sdk">
  <PropertyGroup>
    <TargetFramework>net8.0</TargetFramework>
  </PropertyGroup>
</Project>`), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"OpenTelemetry@1.8.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Check that .csproj was edited
		content, err := os.ReadFile(csprojPath)
		if err != nil {
			t.Fatal(err)
		}

		if !strings.Contains(string(content), "<ItemGroup>") {
			t.Error("Expected ItemGroup in .csproj")
		}
		if !strings.Contains(string(content), `<PackageReference Include="OpenTelemetry" Version="1.8.0"`) {
			t.Error("Expected PackageReference to be added")
		}
	})
}

func TestInstallerErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("missing dependency file", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewGoInstaller(mock)

		// Empty directory
		dir := t.TempDir()

		err := installer.Install(ctx, dir, []string{"some-dep"}, false)
		if err == nil {
			t.Error("Expected error for missing go.mod")
		}
		if !strings.Contains(err.Error(), "go.mod not found") {
			t.Errorf("Expected 'go.mod not found' error, got: %v", err)
		}
	})

	t.Run("command failure", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["go"] = true
		mock.Errors["go get"] = fmt.Errorf("network error")

		installer := NewGoInstaller(mock)

		// Create test project
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatal(err)
		}

		deps := []string{"go.opentelemetry.io/otel@latest"}
		err := installer.Install(ctx, dir, deps, false)
		if err == nil {
			t.Error("Expected error from failed command")
		}
		if !strings.Contains(err.Error(), "network error") {
			t.Errorf("Expected network error, got: %v", err)
		}
	})
}

package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

func TestOrchestrator(t *testing.T) {
	ctx := context.Background()

	// Create test knowledge base
	kb := &knowledge.KnowledgeBase{
		Languages: map[string]knowledge.LanguagePackages{
			"go": {
				Core: []string{
					"go.opentelemetry.io/otel",
					"go.opentelemetry.io/otel/sdk",
				},
				Instrumentations: map[string]string{
					"http": "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
				},
			},
			"javascript": {
				Core: []string{
					"@opentelemetry/api",
					"@opentelemetry/sdk-node",
				},
				Instrumentations: map[string]string{
					"express": "@opentelemetry/instrumentation-express",
					"http":    "@opentelemetry/instrumentation-http",
				},
				Prerequisites: []knowledge.PrerequisiteRule{
					{
						If:       []string{"express"},
						Requires: []string{"http"},
					},
				},
			},
		},
	}

	t.Run("successful installation flow", func(t *testing.T) {
		// Create mock commander
		mock := commander.NewMock()
		mock.Commands["go"] = true

		// Create registry and orchestrator
		reg := registry.New(mock)
		orch := New(reg, kb)

		// Create test project with go.mod
		dir := t.TempDir()
		createGoProject(t, dir)

		// Create install plan
		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		// Run orchestrator
		installed, err := orch.Run(ctx, dir, plan, false)
		if err != nil {
			t.Fatal(err)
		}

		// Should have installed 2 core packages
		if len(installed) != 2 {
			t.Errorf("Expected 2 packages installed, got %d", len(installed))
		}

		// Check that commands were called
		if len(mock.RecordedCalls) == 0 {
			t.Error("Expected commands to be called")
		}
	})

	t.Run("no missing dependencies", func(t *testing.T) {
		mock := commander.NewMock()
		reg := registry.New(mock)
		orch := New(reg, kb)

		// Create test project with existing dependencies
		dir := t.TempDir()
		createGoProjectWithDeps(t, dir)

		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		installed, err := orch.Run(ctx, dir, plan, false)
		if err != nil {
			t.Fatal(err)
		}

		// Should install nothing
		if len(installed) != 0 {
			t.Errorf("Expected no packages installed, got %d", len(installed))
		}

		// No commands should be called
		if len(mock.RecordedCalls) != 0 {
			t.Error("Expected no commands to be called")
		}
	})

	t.Run("dry run", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["go"] = true

		reg := registry.New(mock)
		orch := New(reg, kb)

		dir := t.TempDir()
		createGoProject(t, dir)

		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		installed, err := orch.Run(ctx, dir, plan, true) // dry run
		if err != nil {
			t.Fatal(err)
		}

		// Should return what would be installed
		if len(installed) != 2 {
			t.Errorf("Expected 2 packages in dry run, got %d", len(installed))
		}

		// But no actual commands should be executed
		if len(mock.RecordedCalls) != 0 {
			t.Error("Expected no commands in dry run")
		}
	})

	t.Run("prerequisites expansion", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["npm"] = true
		mock.Responses["npm view"] = `"1.0.0"`

		reg := registry.New(mock)
		orch := New(reg, kb)

		dir := t.TempDir()
		createNodeProject(t, dir)

		plan := types.InstallPlan{
			Language:                "javascript",
			InstallInstrumentations: []string{"express"},
		}

		installed, err := orch.Run(ctx, dir, plan, false)
		if err != nil {
			t.Fatal(err)
		}

		// Should install both express and http (prerequisite)
		if len(installed) != 2 {
			t.Errorf("Expected 2 packages installed (express + http), got %d", len(installed))
		}

		expectedPackages := map[string]bool{
			"@opentelemetry/instrumentation-express": true,
			"@opentelemetry/instrumentation-http":    true,
		}
		for _, pkg := range installed {
			if !expectedPackages[pkg] {
				t.Errorf("Unexpected package installed: %s", pkg)
			}
		}
	})

	t.Run("scanner not found", func(t *testing.T) {
		mock := commander.NewMock()
		reg := registry.New(mock)
		orch := New(reg, kb)

		dir := t.TempDir()

		plan := types.InstallPlan{
			Language: "unknown",
		}

		_, err := orch.Run(ctx, dir, plan, false)
		if err == nil {
			t.Error("Expected error for unknown language")
		}
		if !contains(err.Error(), "no scanner for language") {
			t.Errorf("Expected scanner error, got: %v", err)
		}
	})

	t.Run("no dependency file", func(t *testing.T) {
		mock := commander.NewMock()
		reg := registry.New(mock)
		orch := New(reg, kb)

		dir := t.TempDir() // Empty directory

		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		_, err := orch.Run(ctx, dir, plan, false)
		if err == nil {
			t.Error("Expected error for missing dependency file")
		}
		if !contains(err.Error(), "no dependency file found") {
			t.Errorf("Expected dependency file error, got: %v", err)
		}
	})

	t.Run("installer error", func(t *testing.T) {
		mock := commander.NewMock()
		mock.Commands["go"] = true
		mock.Errors["go get"] = fmt.Errorf("network timeout")

		reg := registry.New(mock)
		orch := New(reg, kb)

		dir := t.TempDir()
		createGoProject(t, dir)

		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		_, err := orch.Run(ctx, dir, plan, false)
		if err == nil {
			t.Error("Expected error from installer")
		}
		if !contains(err.Error(), "network timeout") {
			t.Errorf("Expected network timeout error, got: %v", err)
		}
	})
}

// Helper functions

func createGoProject(t *testing.T, dir string) {
	goMod := `module test

go 1.21
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
}

func createGoProjectWithDeps(t *testing.T, dir string) {
	goMod := `module test

go 1.21

require (
	go.opentelemetry.io/otel v1.24.0
	go.opentelemetry.io/otel/sdk v1.24.0
)
`
	if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte(goMod), 0644); err != nil {
		t.Fatal(err)
	}
}

func createNodeProject(t *testing.T, dir string) {
	packageJSON := `{
  "name": "test",
  "version": "1.0.0"
}`
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(packageJSON), 0644); err != nil {
		t.Fatal(err)
	}
}

func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

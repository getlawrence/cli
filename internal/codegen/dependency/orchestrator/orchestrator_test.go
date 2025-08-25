package orchestrator

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/client"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

func createTestKnowledgeClientForOrchestrator(t *testing.T) *client.KnowledgeClient {
	// Create a temporary database file
	dbPath := "test_orchestrator.db"
	t.Cleanup(func() {
		os.Remove(dbPath)
		os.Remove(dbPath + "-journal")
	})

	// Create new storage
	logger := &logger.StdoutLogger{}
	store, err := storage.NewStorage(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}

	// Create test knowledge base with required components
	components := []kbtypes.Component{
		// Go core packages
		{
			Name:         "go.opentelemetry.io/otel",
			Type:         kbtypes.ComponentTypeAPI,
			Category:     kbtypes.ComponentCategoryAPI,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageGo,
			Description:  "OpenTelemetry API for Go",
			Repository:   "https://github.com/open-telemetry/opentelemetry-go",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		{
			Name:         "go.opentelemetry.io/otel/sdk",
			Type:         kbtypes.ComponentTypeSDK,
			Category:     kbtypes.ComponentCategoryCore,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageGo,
			Description:  "OpenTelemetry SDK for Go",
			Repository:   "https://github.com/open-telemetry/opentelemetry-go",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		// Go instrumentation
		{
			Name:         "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp",
			Type:         kbtypes.ComponentTypeInstrumentation,
			Category:     kbtypes.ComponentCategoryContrib,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageGo,
			Description:  "HTTP instrumentation for Go",
			Repository:   "https://github.com/open-telemetry/opentelemetry-go-contrib",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		// JavaScript core packages
		{
			Name:         "@opentelemetry/api",
			Type:         kbtypes.ComponentTypeAPI,
			Category:     kbtypes.ComponentCategoryAPI,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageJavaScript,
			Description:  "OpenTelemetry API for JavaScript",
			Repository:   "https://github.com/open-telemetry/opentelemetry-js",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		{
			Name:         "@opentelemetry/sdk-node",
			Type:         kbtypes.ComponentTypeSDK,
			Category:     kbtypes.ComponentCategoryCore,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageJavaScript,
			Description:  "OpenTelemetry SDK for Node.js",
			Repository:   "https://github.com/open-telemetry/opentelemetry-js",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		// JavaScript instrumentations
		{
			Name:         "@opentelemetry/instrumentation-express",
			Type:         kbtypes.ComponentTypeInstrumentation,
			Category:     kbtypes.ComponentCategoryContrib,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageJavaScript,
			Description:  "Express instrumentation for JavaScript",
			Repository:   "https://github.com/open-telemetry/opentelemetry-js-contrib",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
		{
			Name:         "@opentelemetry/instrumentation-http",
			Type:         kbtypes.ComponentTypeInstrumentation,
			Category:     kbtypes.ComponentCategoryContrib,
			Status:       kbtypes.ComponentStatusStable,
			SupportLevel: kbtypes.SupportLevelOfficial,
			Language:     kbtypes.ComponentLanguageJavaScript,
			Description:  "HTTP instrumentation for JavaScript",
			Repository:   "https://github.com/open-telemetry/opentelemetry-js-contrib",
			LastUpdated:  time.Now(),
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusLatest,
				},
			},
		},
	}

	// Save the test knowledge base
	err = store.SaveKnowledgeBase(components)
	if err != nil {
		t.Fatalf("Failed to save test knowledge base: %v", err)
	}

	// Create and return the knowledge client
	kc := client.NewKnowledgeClient(store, logger)

	t.Cleanup(func() {
		kc.Close()
	})

	return kc
}

func TestOrchestrator(t *testing.T) {
	ctx := context.Background()

	// Create test knowledge client
	kb := createTestKnowledgeClientForOrchestrator(t)

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
			Language:          "javascript",
			InstallComponents: map[string][]string{"instrumentation": {"express"}},
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
			"@opentelemetry/instrumentation-express@1.0.0": true,
			"@opentelemetry/instrumentation-http@1.0.0":    true,
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

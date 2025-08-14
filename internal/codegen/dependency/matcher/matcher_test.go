package matcher

import (
	"testing"

	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

func TestPlanMatcher(t *testing.T) {
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
					"gin":  "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin",
				},
				Components: map[string]map[string]string{
					"exporter": {
						"jaeger": "go.opentelemetry.io/otel/exporters/jaeger",
					},
				},
			},
			"javascript": {
				Core: []string{
					"@opentelemetry/api",
					"@opentelemetry/sdk-node",
				},
				Instrumentations: map[string]string{
					"http":    "@opentelemetry/instrumentation-http",
					"express": "@opentelemetry/instrumentation-express",
					"auto":    "@opentelemetry/auto-instrumentations-node",
				},
				Prerequisites: []knowledge.PrerequisiteRule{
					{
						If:       []string{"express"},
						Requires: []string{"http"},
						Unless:   []string{"auto"},
					},
				},
			},
		},
	}

	matcher := NewPlanMatcher()

	t.Run("no missing dependencies", func(t *testing.T) {
		existing := []string{
			"go.opentelemetry.io/otel",
			"go.opentelemetry.io/otel/sdk",
		}
		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 0 {
			t.Errorf("Expected no missing dependencies, got %v", missing)
		}
	})

	t.Run("missing core dependencies", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 2 {
			t.Errorf("Expected 2 missing dependencies, got %d", len(missing))
		}

		// Check specific packages
		expectedMissing := map[string]bool{
			"go.opentelemetry.io/otel":     true,
			"go.opentelemetry.io/otel/sdk": true,
		}
		for _, pkg := range missing {
			if !expectedMissing[pkg] {
				t.Errorf("Unexpected missing package: %s", pkg)
			}
		}
	})

	t.Run("missing instrumentation", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:                "go",
			InstallInstrumentations: []string{"http"},
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 1 {
			t.Errorf("Expected 1 missing dependency, got %d", len(missing))
		}
		if missing[0] != "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp" {
			t.Errorf("Expected http instrumentation, got %s", missing[0])
		}
	})

	t.Run("missing component", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language: "go",
			InstallComponents: map[string][]string{
				"exporter": {"jaeger"},
			},
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 1 {
			t.Errorf("Expected 1 missing dependency, got %d", len(missing))
		}
		if missing[0] != "go.opentelemetry.io/otel/exporters/jaeger" {
			t.Errorf("Expected jaeger exporter, got %s", missing[0])
		}
	})

	t.Run("prerequisites expansion", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:                "javascript",
			InstallInstrumentations: []string{"express"},
		}

		missing := matcher.Match(existing, plan, kb)
		// Should include both express and http (prerequisite)
		if len(missing) != 2 {
			t.Errorf("Expected 2 missing dependencies (express + http), got %d", len(missing))
		}

		expectedMissing := map[string]bool{
			"@opentelemetry/instrumentation-http":    true,
			"@opentelemetry/instrumentation-express": true,
		}
		for _, pkg := range missing {
			if !expectedMissing[pkg] {
				t.Errorf("Unexpected missing package: %s", pkg)
			}
		}
	})

	t.Run("prerequisites with unless condition", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:                "javascript",
			InstallInstrumentations: []string{"express", "auto"},
		}

		missing := matcher.Match(existing, plan, kb)
		// Should NOT include http because auto is present
		expectedMissing := map[string]bool{
			"@opentelemetry/instrumentation-express":    true,
			"@opentelemetry/auto-instrumentations-node": true,
		}

		if len(missing) != 2 {
			t.Errorf("Expected 2 missing dependencies, got %d: %v", len(missing), missing)
		}

		for _, pkg := range missing {
			if !expectedMissing[pkg] {
				t.Errorf("Unexpected missing package: %s", pkg)
			}
		}
	})

	t.Run("case insensitive matching", func(t *testing.T) {
		existing := []string{
			"GO.OPENTELEMETRY.IO/OTEL", // uppercase
		}
		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		missing := matcher.Match(existing, plan, kb)
		// Should only be missing the SDK, not the API
		if len(missing) != 1 {
			t.Errorf("Expected 1 missing dependency, got %d", len(missing))
		}
		if missing[0] != "go.opentelemetry.io/otel/sdk" {
			t.Errorf("Expected SDK to be missing, got %s", missing[0])
		}
	})
}

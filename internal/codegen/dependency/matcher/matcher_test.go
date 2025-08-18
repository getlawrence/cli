package matcher

import (
	"os"
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/client"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

func createTestKnowledgeClient(t *testing.T) *client.KnowledgeClient {
	// Create a temporary database file
	dbPath := "test_matcher.db"
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
	kb := &kbtypes.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components: []kbtypes.Component{
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
			// Go instrumentations
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
			// Go exporters
			{
				Name:         "go.opentelemetry.io/otel/exporters/jaeger",
				Type:         kbtypes.ComponentTypeExporter,
				Category:     kbtypes.ComponentCategoryCore,
				Status:       kbtypes.ComponentStatusStable,
				SupportLevel: kbtypes.SupportLevelOfficial,
				Language:     kbtypes.ComponentLanguageGo,
				Description:  "Jaeger exporter for Go",
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
				Name:         "@opentelemetry/auto-instrumentations-node",
				Type:         kbtypes.ComponentTypeInstrumentation,
				Category:     kbtypes.ComponentCategoryContrib,
				Status:       kbtypes.ComponentStatusStable,
				SupportLevel: kbtypes.SupportLevelOfficial,
				Language:     kbtypes.ComponentLanguageJavaScript,
				Description:  "Auto instrumentation for Node.js",
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
		},
	}

	// Save the test knowledge base
	err = store.SaveKnowledgeBase(kb, "test")
	if err != nil {
		t.Fatalf("Failed to save test knowledge base: %v", err)
	}

	// Create and return the knowledge client
	kc, err := client.NewKnowledgeClient(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create knowledge client: %v", err)
	}

	t.Cleanup(func() {
		kc.Close()
	})

	return kc
}

func TestPlanMatcher(t *testing.T) {
	// Create test knowledge client
	kb := createTestKnowledgeClient(t)

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

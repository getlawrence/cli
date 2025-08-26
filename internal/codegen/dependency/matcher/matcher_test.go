package matcher

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

func createTestKnowledgeClient(t *testing.T) *knowledge.Knowledge {
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
	}

	// Save the test knowledge base
	err = store.SaveKnowledgeBase(components)
	if err != nil {
		t.Fatalf("Failed to save test knowledge base: %v", err)
	}

	// Create and return the knowledge client
	kc := knowledge.NewKnowledge(*store, logger)

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
			Language:          "go",
			InstallComponents: map[string][]string{"instrumentation": {"http"}},
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
			Language:          "javascript",
			InstallComponents: map[string][]string{"instrumentation": {"express"}},
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
			Language:          "javascript",
			InstallComponents: map[string][]string{"instrumentation": {"express", "auto"}},
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

func TestKnowledgeEnhancedMatcher(t *testing.T) {
	// Create test knowledge client
	kb := createTestKnowledgeClient(t)

	// Create knowledge-enhanced matcher
	matcher := NewKnowledgeEnhancedMatcher(kb)

	t.Run("version resolution for core packages", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:    "go",
			InstallOTEL: true,
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 2 {
			t.Errorf("Expected 2 missing dependencies, got %d", len(missing))
		}

		// Check that packages have versions
		for _, pkg := range missing {
			if !strings.Contains(pkg, "@") {
				t.Errorf("Expected package with version, got: %s", pkg)
			}
		}

		// Check specific packages with versions
		expectedPackages := map[string]bool{
			"go.opentelemetry.io/otel@1.0.0":     true,
			"go.opentelemetry.io/otel/sdk@1.0.0": true,
		}
		for _, pkg := range missing {
			if !expectedPackages[pkg] {
				t.Errorf("Unexpected package: %s", pkg)
			}
		}
	})

	t.Run("version resolution for instrumentation", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:          "go",
			InstallComponents: map[string][]string{"instrumentation": {"http"}},
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 1 {
			t.Errorf("Expected 1 missing dependency, got %d", len(missing))
		}

		// Check that instrumentation has version
		pkg := missing[0]
		if !strings.Contains(pkg, "@") {
			t.Errorf("Expected instrumentation with version, got: %s", pkg)
		}
		if pkg != "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@1.0.0" {
			t.Errorf("Expected specific instrumentation with version, got: %s", pkg)
		}
	})

	t.Run("version resolution for components", func(t *testing.T) {
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

		// Check that component has version
		pkg := missing[0]
		if !strings.Contains(pkg, "@") {
			t.Errorf("Expected component with version, got: %s", pkg)
		}
		if pkg != "go.opentelemetry.io/otel/exporters/jaeger@1.0.0" {
			t.Errorf("Expected specific component with version, got: %s", pkg)
		}
	})

	t.Run("preserves existing version specifiers", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language:          "go",
			InstallComponents: map[string][]string{"instrumentation": {"http"}},
		}

		// Mock a package that already has a version
		originalMatcher := matcher
		defer func() { matcher = originalMatcher }()

		// Create a mock matcher that returns pre-versioned packages
		matcher = &MockKnowledgeEnhancedMatcher{
			PlanMatcher: &PlanMatcher{},
			kb:          kb,
			preVersionedPackages: map[string]string{
				"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp": "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@1.2.3",
			},
		}

		missing := matcher.Match(existing, plan, kb)
		if len(missing) != 1 {
			t.Errorf("Expected 1 missing dependency, got %d", len(missing))
		}

		// Check that the pre-versioned package is preserved
		pkg := missing[0]
		if pkg != "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp@1.2.3" {
			t.Errorf("Expected pre-versioned package to be preserved, got: %s", pkg)
		}
	})

	t.Run("handles packages not in knowledge base", func(t *testing.T) {
		existing := []string{}
		plan := types.InstallPlan{
			Language: "go",
			InstallComponents: map[string][]string{
				"exporter": {"unknown-exporter"},
			},
		}

		missing := matcher.Match(existing, plan, kb)
		// Unknown components are not included in missing dependencies
		// This is the correct behavior - only known components should be installed
		if len(missing) != 0 {
			t.Errorf("Expected 0 missing dependencies for unknown component, got %d: %v", len(missing), missing)
		}
	})

	t.Run("language-specific version formatting", func(t *testing.T) {
		// Test that the KnowledgeEnhancedMatcher formats versions correctly for different languages
		// This test verifies the formatPackageWithVersion method works correctly
		matcher := &KnowledgeEnhancedMatcher{
			PlanMatcher:     &PlanMatcher{},
			knowledgeClient: kb,
		}

		// Test the formatting logic directly
		testCases := []struct {
			language string
			expected string
		}{
			{"python", "opentelemetry-api==1.0.0"},
			{"javascript", "opentelemetry-api@1.0.0"},
			{"go", "opentelemetry-api@1.0.0"},
			{"java", "opentelemetry-api:1.0.0"},
			{"csharp", "opentelemetry-api@1.0.0"},
			{"php", "opentelemetry-api:1.0.0"},
			{"ruby", "opentelemetry-api:1.0.0"},
		}

		for _, tc := range testCases {
			t.Run(tc.language, func(t *testing.T) {
				// Test the formatting method directly
				formatted := matcher.FormatPackageWithVersion("opentelemetry-api", "1.0.0", tc.language)
				if formatted != tc.expected {
					t.Errorf("Expected %s, got %s", tc.expected, formatted)
				}
			})
		}
	})

	t.Run("filters out packages with deprecated versions", func(t *testing.T) {
		// Test that deprecated packages are filtered out
		// This test verifies the getLatestStableVersion method works correctly
		matcher := &KnowledgeEnhancedMatcher{
			PlanMatcher:     &PlanMatcher{},
			knowledgeClient: kb,
		}

		// Create a component with deprecated versions
		deprecatedComponent := &kbtypes.Component{
			Name:     "deprecated-package",
			Language: kbtypes.ComponentLanguageGo,
			Versions: []kbtypes.Version{
				{
					Name:        "1.0.0",
					ReleaseDate: time.Now(),
					Status:      kbtypes.VersionStatusDeprecated,
					Deprecated:  true,
				},
			},
		}

		// Test the version filtering logic directly
		latestVersion := matcher.GetLatestStableVersion(deprecatedComponent)
		if latestVersion != nil {
			t.Errorf("Expected no stable version for deprecated component, got: %v", latestVersion)
		}
	})
}

// Mock types for testing
type MockKnowledgeEnhancedMatcher struct {
	*PlanMatcher
	kb                   *knowledge.Knowledge
	preVersionedPackages map[string]string
}

func (m *MockKnowledgeEnhancedMatcher) Match(existingDeps []string, plan types.InstallPlan, kb *knowledge.Knowledge) []string {
	// First get the basic missing packages
	basicMissing := m.PlanMatcher.Match(existingDeps, plan, kb)
	if len(basicMissing) == 0 {
		return nil
	}

	// Enhance with specific versions and filter out packages with invalid versions
	var enhancedMissing []string
	for _, pkg := range basicMissing {
		// Check if package already has a version specifier
		if hasVersionSpecifier(pkg) {
			enhancedMissing = append(enhancedMissing, pkg)
			continue
		}

		// Check if we have a pre-versioned package for testing
		if versionedPkg, exists := m.preVersionedPackages[pkg]; exists {
			enhancedMissing = append(enhancedMissing, versionedPkg)
			continue
		}

		// Try to find the package in the knowledge base and get latest stable version
		if versionedPkg := m.getPackageWithVersion(pkg, plan.Language); versionedPkg != "" {
			enhancedMissing = append(enhancedMissing, versionedPkg)
		} else {
			// Check if this package has "unknown" version - if so, skip it entirely
			if m.hasUnknownVersion(pkg) {
				// Skip packages that exist in registry but not in package manager
				continue
			}
			// Fallback to package without version for packages not in knowledge base
			enhancedMissing = append(enhancedMissing, pkg)
		}
	}

	return enhancedMissing
}

func (m *MockKnowledgeEnhancedMatcher) getPackageWithVersion(packageName, language string) string {
	// Delegate to the real implementation
	realMatcher := &KnowledgeEnhancedMatcher{
		PlanMatcher:     &PlanMatcher{},
		knowledgeClient: m.kb,
	}
	return realMatcher.getPackageWithVersion(packageName, language)
}

func (m *MockKnowledgeEnhancedMatcher) hasUnknownVersion(packageName string) bool {
	// Delegate to the real implementation
	realMatcher := &KnowledgeEnhancedMatcher{
		PlanMatcher:     &PlanMatcher{},
		knowledgeClient: m.kb,
	}
	return realMatcher.hasUnknownVersion(packageName)
}

type MockKnowledgeClient struct {
	components map[string]*kbtypes.Component
}

func (m *MockKnowledgeClient) GetComponentByName(name string) (*kbtypes.Component, error) {
	if component, exists := m.components[name]; exists {
		return component, nil
	}
	return nil, fmt.Errorf("component not found: %s", name)
}

func (m *MockKnowledgeClient) GetCorePackages(language string) ([]string, error) {
	return []string{}, nil
}

func (m *MockKnowledgeClient) GetInstrumentationPackage(language, name string) (string, error) {
	return "", nil
}

func (m *MockKnowledgeClient) GetComponentPackage(language, compType, name string) (string, error) {
	return "", nil
}

func (m *MockKnowledgeClient) GetPrerequisites(language string) ([]knowledge.PrerequisiteRule, error) {
	return []knowledge.PrerequisiteRule{}, nil
}

func (m *MockKnowledgeClient) Close() error {
	return nil
}

// Additional methods to satisfy the interface
func (m *MockKnowledgeClient) GetComponentsByLanguage(language string, limit, offset int) (*knowledge.ComponentResult, error) {
	return &knowledge.ComponentResult{}, nil
}

func (m *MockKnowledgeClient) GetComponentsByType(componentType string, limit, offset int) (*knowledge.ComponentResult, error) {
	return &knowledge.ComponentResult{}, nil
}

func (m *MockKnowledgeClient) QueryComponents(query knowledge.ComponentQuery) (*knowledge.ComponentResult, error) {
	return &knowledge.ComponentResult{}, nil
}

type MockInstaller struct {
	receivedDeps []string
}

func (m *MockInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	m.receivedDeps = dependencies
	return nil
}

type MockCommander struct{}

func (m *MockCommander) LookPath(file string) (string, error) {
	return "", fmt.Errorf("command not found")
}

func (m *MockCommander) Run(ctx context.Context, name string, args []string, dir string) (string, error) {
	return "", fmt.Errorf("command not found")
}

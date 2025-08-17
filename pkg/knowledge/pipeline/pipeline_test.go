package pipeline

import (
	"testing"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/npm"
	"github.com/getlawrence/cli/pkg/knowledge/registry"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

func TestPipeline_UpdateKnowledgeBase(t *testing.T) {
	// Create mock clients
	mockRegistry := &MockRegistryClient{
		components: []registry.RegistryComponent{
			{
				Name:        "@opentelemetry/api",
				Type:        "API",
				Language:    "javascript",
				Description: "OpenTelemetry API for JavaScript",
				Repository:  "https://github.com/open-telemetry/opentelemetry-js",
				RegistryURL: "https://registry.opentelemetry.io/components/javascript/api",
				Homepage:    "https://opentelemetry.io/docs/languages/js/",
				Tags:        []string{"api", "core", "telemetry"},
				Maintainers: []string{"OpenTelemetry"},
				License:     "Apache-2.0",
				LastUpdated: time.Now(),
			},
			{
				Name:        "@opentelemetry/instrumentation-express",
				Type:        "instrumentation",
				Language:    "javascript",
				Description: "OpenTelemetry Express instrumentation",
				Repository:  "https://github.com/open-telemetry/opentelemetry-js-contrib",
				RegistryURL: "https://registry.opentelemetry.io/components/javascript/instrumentation-express",
				Homepage:    "https://opentelemetry.io/docs/languages/js/",
				Tags:        []string{"instrumentation", "express", "web"},
				Maintainers: []string{"OpenTelemetry"},
				License:     "Apache-2.0",
				LastUpdated: time.Now(),
			},
		},
	}

	mockNPM := &MockNPMClient{
		packages: map[string]*npm.Package{
			"@opentelemetry/api": {
				Name:        "@opentelemetry/api",
				Description: "OpenTelemetry API for JavaScript",
				License:     "Apache-2.0",
				DistTags: map[string]string{
					"latest": "1.7.0",
				},
				Time: map[string]time.Time{
					"1.7.0": time.Date(2024, 12, 1, 0, 0, 0, 0, time.UTC),
				},
				Versions: map[string]npm.Version{
					"1.7.0": {
						Version: "1.7.0",
						Dependencies: map[string]string{
							"typescript": "^4.8.0",
						},
						Engines: map[string]string{
							"node": ">=18.0.0",
						},
					},
				},
			},
			"@opentelemetry/instrumentation-express": {
				Name:        "@opentelemetry/instrumentation-express",
				Description: "OpenTelemetry Express instrumentation",
				License:     "Apache-2.0",
				DistTags: map[string]string{
					"latest": "1.5.0",
				},
				Time: map[string]time.Time{
					"1.5.0": time.Date(2024, 5, 15, 0, 0, 0, 0, time.UTC),
				},
				Versions: map[string]npm.Version{
					"1.5.0": {
						Version: "1.5.0",
						Dependencies: map[string]string{
							"typescript": "^4.8.0",
						},
						Engines: map[string]string{
							"node": ">=18.0.0",
						},
					},
				},
			},
		},
	}

	// Create pipeline with mock clients
	p := NewPipelineWithClients(mockRegistry, mockNPM)

	// Test knowledge base update
	kb, err := p.UpdateKnowledgeBase("javascript")
	if err != nil {
		t.Fatalf("Failed to update knowledge base: %v", err)
	}

	// Verify basic structure
	if kb.SchemaVersion != "1.0.0" {
		t.Errorf("Expected schema version 1.0.0, got %s", kb.SchemaVersion)
	}

	if len(kb.Components) != 2 {
		t.Errorf("Expected 2 components, got %d", len(kb.Components))
	}

	// Verify API component data
	apiComponent := kb.Components[0]
	if apiComponent.Name != "@opentelemetry/api" {
		t.Errorf("Expected component name @opentelemetry/api, got %s", apiComponent.Name)
	}

	if apiComponent.Type != types.ComponentTypeAPI {
		t.Errorf("Expected component type API, got %s", apiComponent.Type)
	}

	if apiComponent.Category != types.ComponentCategoryAPI {
		t.Errorf("Expected component category API, got %s", apiComponent.Category)
	}

	if apiComponent.Status != types.ComponentStatusStable {
		t.Errorf("Expected component status stable, got %s", apiComponent.Status)
	}

	if apiComponent.SupportLevel != types.SupportLevelOfficial {
		t.Errorf("Expected component support level official, got %s", apiComponent.SupportLevel)
	}

	// Verify instrumentation component data
	instrumentationComponent := kb.Components[1]
	if instrumentationComponent.Name != "@opentelemetry/instrumentation-express" {
		t.Errorf("Expected component name @opentelemetry/instrumentation-express, got %s", instrumentationComponent.Name)
	}

	if instrumentationComponent.Type != types.ComponentTypeInstrumentation {
		t.Errorf("Expected component type Instrumentation, got %s", instrumentationComponent.Type)
	}

	if instrumentationComponent.Category != types.ComponentCategoryContrib {
		t.Errorf("Expected component category CONTRIB, got %s", instrumentationComponent.Category)
	}

	if instrumentationComponent.Status != types.ComponentStatusExperimental {
		t.Errorf("Expected component status experimental, got %s", instrumentationComponent.Status)
	}

	if instrumentationComponent.SupportLevel != types.SupportLevelCommunity {
		t.Errorf("Expected component support level community, got %s", instrumentationComponent.SupportLevel)
	}

	// Verify instrumentation targets
	if len(instrumentationComponent.InstrumentationTargets) != 1 {
		t.Errorf("Expected 1 instrumentation target, got %d", len(instrumentationComponent.InstrumentationTargets))
	}

	target := instrumentationComponent.InstrumentationTargets[0]
	if target.Framework != "Express" {
		t.Errorf("Expected framework Express, got %s", target.Framework)
	}

	// Verify versions
	if len(apiComponent.Versions) != 1 {
		t.Errorf("Expected 1 version, got %d", len(apiComponent.Versions))
	}

	version := apiComponent.Versions[0]
	if version.Name != "1.7.0" {
		t.Errorf("Expected version name 1.7.0, got %s", version.Name)
	}

	if version.Status != types.VersionStatusLatest {
		t.Errorf("Expected version status latest, got %s", version.Status)
	}

	// Verify statistics
	if kb.Statistics.TotalComponents != 2 {
		t.Errorf("Expected 2 total components, got %d", kb.Statistics.TotalComponents)
	}

	if kb.Statistics.TotalVersions != 2 {
		t.Errorf("Expected 2 total versions, got %d", kb.Statistics.TotalVersions)
	}

	// Verify enhanced statistics
	if len(kb.Statistics.ByCategory) == 0 {
		t.Error("Expected category statistics to be populated")
	}

	if len(kb.Statistics.ByStatus) == 0 {
		t.Error("Expected status statistics to be populated")
	}

	if len(kb.Statistics.BySupportLevel) == 0 {
		t.Error("Expected support level statistics to be populated")
	}
}

// MockRegistryClient is a mock implementation of the registry client
type MockRegistryClient struct {
	components []registry.RegistryComponent
}

func (m *MockRegistryClient) GetJavaScriptComponents() ([]registry.RegistryComponent, error) {
	return m.components, nil
}

func (m *MockRegistryClient) GetComponentsByLanguage(language string) ([]registry.RegistryComponent, error) {
	return m.components, nil
}

func (m *MockRegistryClient) GetComponentByName(name string) (*registry.RegistryComponent, error) {
	for _, c := range m.components {
		if c.Name == name {
			return &c, nil
		}
	}
	return nil, nil
}

// MockNPMClient is a mock implementation of the npm client
type MockNPMClient struct {
	packages map[string]*npm.Package
}

func (m *MockNPMClient) GetPackage(name string) (*npm.Package, error) {
	if pkg, ok := m.packages[name]; ok {
		return pkg, nil
	}
	return nil, nil
}

func (m *MockNPMClient) GetPackageVersion(name, version string) (*npm.Version, error) {
	if pkg, ok := m.packages[name]; ok {
		if ver, ok := pkg.Versions[version]; ok {
			return &ver, nil
		}
	}
	return nil, nil
}

func (m *MockNPMClient) GetLatestVersion(name string) (*npm.Version, error) {
	if pkg, ok := m.packages[name]; ok {
		if latest, ok := pkg.DistTags["latest"]; ok {
			if ver, ok := pkg.Versions[latest]; ok {
				return &ver, nil
			}
		}
	}
	return nil, nil
}

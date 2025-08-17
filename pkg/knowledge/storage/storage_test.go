package storage

import (
	"os"
	"testing"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/types"
)

func TestStorage_SaveAndLoadKnowledgeBase(t *testing.T) {
	// Create a temporary directory for testing
	tempDir := t.TempDir()
	testFile := tempDir + "/test_kb.json"

	// Create test knowledge base
	kb := &types.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components: []types.Component{
			{
				Name:         "test-component",
				Type:         types.ComponentTypeAPI,
				Category:     types.ComponentCategoryAPI,
				Status:       types.ComponentStatusStable,
				SupportLevel: types.SupportLevelOfficial,
				Language:     types.ComponentLanguageJavaScript,
				Description:  "Test component",
				Repository:   "https://github.com/test/test",
				LastUpdated:  time.Now(),
				Versions: []types.Version{
					{
						Name:        "1.0.0",
						ReleaseDate: time.Now(),
						Status:      types.VersionStatusLatest,
					},
				},
			},
		},
		Statistics: types.Statistics{
			TotalComponents: 1,
			TotalVersions:   1,
			LastUpdate:      time.Now(),
			Source:          "Test",
			ByLanguage:      map[string]int{"javascript": 1},
			ByType:          map[string]int{"API": 1},
			ByCategory:      map[string]int{"API": 1},
			ByStatus:        map[string]int{"stable": 1},
			BySupportLevel:  map[string]int{"official": 1},
		},
	}

	// Create storage
	storage := NewStorage(tempDir)

	// Test save
	err := storage.SaveKnowledgeBase(kb, testFile)
	if err != nil {
		t.Fatalf("Failed to save knowledge base: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		t.Fatalf("Knowledge base file was not created")
	}

	// Test load
	loadedKB, err := storage.LoadKnowledgeBase(testFile)
	if err != nil {
		t.Fatalf("Failed to load knowledge base: %v", err)
	}

	// Verify loaded data
	if loadedKB.SchemaVersion != kb.SchemaVersion {
		t.Errorf("Expected schema version %s, got %s", kb.SchemaVersion, loadedKB.SchemaVersion)
	}

	if len(loadedKB.Components) != len(kb.Components) {
		t.Errorf("Expected %d components, got %d", len(kb.Components), len(loadedKB.Components))
	}

	if loadedKB.Statistics.TotalComponents != kb.Statistics.TotalComponents {
		t.Errorf("Expected %d total components, got %d", kb.Statistics.TotalComponents, loadedKB.Statistics.TotalComponents)
	}
}

func TestStorage_QueryKnowledgeBase(t *testing.T) {
	// Create test knowledge base
	kb := &types.KnowledgeBase{
		SchemaVersion: "1.0.0",
		GeneratedAt:   time.Now(),
		Components: []types.Component{
			{
				Name:         "test-api",
				Type:         types.ComponentTypeAPI,
				Category:     types.ComponentCategoryAPI,
				Status:       types.ComponentStatusStable,
				SupportLevel: types.SupportLevelOfficial,
				Language:     types.ComponentLanguageJavaScript,
				Repository:   "https://github.com/test/api",
				LastUpdated:  time.Now(),
				Versions: []types.Version{
					{
						Name:        "1.0.0",
						ReleaseDate: time.Now(),
						Status:      types.VersionStatusLatest,
					},
				},
			},
			{
				Name:         "test-instrumentation",
				Type:         types.ComponentTypeInstrumentation,
				Category:     types.ComponentCategoryContrib,
				Status:       types.ComponentStatusExperimental,
				SupportLevel: types.SupportLevelCommunity,
				Language:     types.ComponentLanguageJavaScript,
				Repository:   "https://github.com/test/instrumentation",
				LastUpdated:  time.Now(),
				InstrumentationTargets: []types.InstrumentationTarget{
					{
						Framework:    "Express",
						VersionRange: ">=4.0.0",
					},
				},
				Versions: []types.Version{
					{
						Name:        "1.0.0",
						ReleaseDate: time.Now(),
						Status:      types.VersionStatusStable,
					},
				},
			},
		},
		Statistics: types.Statistics{
			TotalComponents: 2,
			TotalVersions:   2,
			LastUpdate:      time.Now(),
			Source:          "Test",
		},
	}

	storage := NewStorage("")

	// Test query by type
	query := Query{Type: "API"}
	result := storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 API component, got %d", result.Total)
	}

	// Test query by category
	query = Query{Category: "API"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 API category component, got %d", result.Total)
	}

	// Test query by status
	query = Query{Status: "stable"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 stable status component, got %d", result.Total)
	}

	// Test query by support level
	query = Query{SupportLevel: "official"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 official support level component, got %d", result.Total)
	}

	// Test query by language
	query = Query{Language: "javascript"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 2 {
		t.Errorf("Expected 2 JavaScript components, got %d", result.Total)
	}

	// Test query by name
	query = Query{Name: "api"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 component with 'api' in name, got %d", result.Total)
	}

	// Test query by framework
	query = Query{Framework: "express"}
	result = storage.QueryKnowledgeBase(kb, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 component with express framework, got %d", result.Total)
	}
}

func TestStorage_GetComponentsByType(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:     "api-1",
				Type:     types.ComponentTypeAPI,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "api-2",
				Type:     types.ComponentTypeAPI,
				Language: types.ComponentLanguageGo,
			},
			{
				Name:     "instrumentation-1",
				Type:     types.ComponentTypeInstrumentation,
				Language: types.ComponentLanguageJavaScript,
			},
		},
	}

	storage := NewStorage("")

	// Test getting API components
	apiComponents := storage.GetComponentsByType(kb, types.ComponentTypeAPI)
	if len(apiComponents) != 2 {
		t.Errorf("Expected 2 API components, got %d", len(apiComponents))
	}

	// Test getting instrumentation components
	instrumentationComponents := storage.GetComponentsByType(kb, types.ComponentTypeInstrumentation)
	if len(instrumentationComponents) != 1 {
		t.Errorf("Expected 1 instrumentation component, got %d", len(instrumentationComponents))
	}
}

func TestStorage_GetComponentsByLanguage(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:     "js-1",
				Type:     types.ComponentTypeAPI,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "js-2",
				Type:     types.ComponentTypeSDK,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "go-1",
				Type:     types.ComponentTypeAPI,
				Language: types.ComponentLanguageGo,
			},
		},
	}

	storage := NewStorage("")

	// Test getting JavaScript components
	jsComponents := storage.GetComponentsByLanguage(kb, types.ComponentLanguageJavaScript)
	if len(jsComponents) != 2 {
		t.Errorf("Expected 2 JavaScript components, got %d", len(jsComponents))
	}

	// Test getting Go components
	goComponents := storage.GetComponentsByLanguage(kb, types.ComponentLanguageGo)
	if len(goComponents) != 1 {
		t.Errorf("Expected 1 Go component, got %d", len(goComponents))
	}
}

func TestStorage_GetComponentsByCategory(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:     "api-1",
				Type:     types.ComponentTypeAPI,
				Category: types.ComponentCategoryAPI,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "sdk-1",
				Type:     types.ComponentTypeSDK,
				Category: types.ComponentCategoryStableSDK,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "contrib-1",
				Type:     types.ComponentTypeInstrumentation,
				Category: types.ComponentCategoryContrib,
				Language: types.ComponentLanguageJavaScript,
			},
		},
	}

	storage := NewStorage("")

	// Test getting API category components
	apiComponents := storage.GetComponentsByCategory(kb, types.ComponentCategoryAPI)
	if len(apiComponents) != 1 {
		t.Errorf("Expected 1 API category component, got %d", len(apiComponents))
	}

	// Test getting StableSDK category components
	sdkComponents := storage.GetComponentsByCategory(kb, types.ComponentCategoryStableSDK)
	if len(sdkComponents) != 1 {
		t.Errorf("Expected 1 StableSDK category component, got %d", len(sdkComponents))
	}

	// Test getting Contrib category components
	contribComponents := storage.GetComponentsByCategory(kb, types.ComponentCategoryContrib)
	if len(contribComponents) != 1 {
		t.Errorf("Expected 1 Contrib category component, got %d", len(contribComponents))
	}
}

func TestStorage_GetComponentsByStatus(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:     "stable-1",
				Type:     types.ComponentTypeAPI,
				Status:   types.ComponentStatusStable,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "experimental-1",
				Type:     types.ComponentTypeSDK,
				Status:   types.ComponentStatusExperimental,
				Language: types.ComponentLanguageJavaScript,
			},
			{
				Name:     "deprecated-1",
				Type:     types.ComponentTypeInstrumentation,
				Status:   types.ComponentStatusDeprecated,
				Language: types.ComponentLanguageJavaScript,
			},
		},
	}

	storage := NewStorage("")

	// Test getting stable status components
	stableComponents := storage.GetComponentsByStatus(kb, types.ComponentStatusStable)
	if len(stableComponents) != 1 {
		t.Errorf("Expected 1 stable status component, got %d", len(stableComponents))
	}

	// Test getting experimental status components
	experimentalComponents := storage.GetComponentsByStatus(kb, types.ComponentStatusExperimental)
	if len(experimentalComponents) != 1 {
		t.Errorf("Expected 1 experimental status component, got %d", len(experimentalComponents))
	}

	// Test getting deprecated status components
	deprecatedComponents := storage.GetComponentsByStatus(kb, types.ComponentStatusDeprecated)
	if len(deprecatedComponents) != 1 {
		t.Errorf("Expected 1 deprecated status component, got %d", len(deprecatedComponents))
	}
}

func TestStorage_GetComponentsBySupportLevel(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:         "official-1",
				Type:         types.ComponentTypeAPI,
				SupportLevel: types.SupportLevelOfficial,
				Language:     types.ComponentLanguageJavaScript,
			},
			{
				Name:         "community-1",
				Type:         types.ComponentTypeSDK,
				SupportLevel: types.SupportLevelCommunity,
				Language:     types.ComponentLanguageJavaScript,
			},
			{
				Name:         "vendor-1",
				Type:         types.ComponentTypeInstrumentation,
				SupportLevel: types.SupportLevelVendor,
				Language:     types.ComponentLanguageJavaScript,
			},
		},
	}

	storage := NewStorage("")

	// Test getting official support level components
	officialComponents := storage.GetComponentsBySupportLevel(kb, types.SupportLevelOfficial)
	if len(officialComponents) != 1 {
		t.Errorf("Expected 1 official support level component, got %d", len(officialComponents))
	}

	// Test getting community support level components
	communityComponents := storage.GetComponentsBySupportLevel(kb, types.SupportLevelCommunity)
	if len(communityComponents) != 1 {
		t.Errorf("Expected 1 community support level component, got %d", len(communityComponents))
	}

	// Test getting vendor support level components
	vendorComponents := storage.GetComponentsBySupportLevel(kb, types.SupportLevelVendor)
	if len(vendorComponents) != 1 {
		t.Errorf("Expected 1 vendor support level component, got %d", len(vendorComponents))
	}
}

func TestStorage_GetInstrumentationsByFramework(t *testing.T) {
	kb := &types.KnowledgeBase{
		Components: []types.Component{
			{
				Name:     "express-instrumentation",
				Type:     types.ComponentTypeInstrumentation,
				Language: types.ComponentLanguageJavaScript,
				InstrumentationTargets: []types.InstrumentationTarget{
					{
						Framework:    "Express",
						VersionRange: ">=4.0.0",
					},
				},
			},
			{
				Name:     "fastify-instrumentation",
				Type:     types.ComponentTypeInstrumentation,
				Language: types.ComponentLanguageJavaScript,
				InstrumentationTargets: []types.InstrumentationTarget{
					{
						Framework:    "Fastify",
						VersionRange: ">=3.0.0",
					},
				},
			},
			{
				Name:     "api-component",
				Type:     types.ComponentTypeAPI,
				Language: types.ComponentLanguageJavaScript,
			},
		},
	}

	storage := NewStorage("")

	// Test getting Express instrumentations
	expressComponents := storage.GetInstrumentationsByFramework(kb, "express")
	if len(expressComponents) != 1 {
		t.Errorf("Expected 1 Express instrumentation, got %d", len(expressComponents))
	}

	// Test getting Fastify instrumentations
	fastifyComponents := storage.GetInstrumentationsByFramework(kb, "fastify")
	if len(fastifyComponents) != 1 {
		t.Errorf("Expected 1 Fastify instrumentation, got %d", len(fastifyComponents))
	}

	// Test getting non-existent framework
	nonexistentComponents := storage.GetInstrumentationsByFramework(kb, "nonexistent")
	if len(nonexistentComponents) != 0 {
		t.Errorf("Expected 0 components for nonexistent framework, got %d", len(nonexistentComponents))
	}
}

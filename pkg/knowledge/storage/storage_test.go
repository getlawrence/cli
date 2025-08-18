package storage

import (
	"os"
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

func TestSQLiteStorage(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_storage.db"
	defer os.Remove(dbPath)

	// Create new storage
	logger := &logger.StdoutLogger{}
	storage, err := NewStorage(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create a test knowledge base
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
				Language:     types.ComponentLanguageGo,
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
	}

	// Test saving knowledge base
	err = storage.SaveKnowledgeBase(kb, "test")
	if err != nil {
		t.Fatalf("Failed to save knowledge base: %v", err)
	}

	// Test loading knowledge base
	loadedKB, err := storage.LoadKnowledgeBase()
	if err != nil {
		t.Fatalf("Failed to load knowledge base: %v", err)
	}

	// Query for the component we just saved
	query := Query{
		Language: "go",
		Type:     "API",
	}

	result := storage.QueryKnowledgeBase(loadedKB, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 result from query, got %d", result.Total)
	}

	if len(result.Components) != 1 {
		t.Errorf("Expected 1 component in result, got %d", len(result.Components))
	}

	component := result.Components[0]
	if component.Name != "test-component" {
		t.Errorf("Expected component name 'test-component', got '%s'", component.Name)
	}

	if component.Type != types.ComponentTypeAPI {
		t.Errorf("Expected component type API, got %s", component.Type)
	}

	// Test querying (reuse the query from above)
	result = storage.QueryKnowledgeBase(loadedKB, query)
	if result.Total != 1 {
		t.Errorf("Expected 1 result from query, got %d", result.Total)
	}

	// Test getting components by type
	components := storage.GetComponentsByType(loadedKB, types.ComponentTypeAPI)
	if len(components) != 1 {
		t.Errorf("Expected 1 component by type, got %d", len(components))
	}

	// Test getting component by name
	componentByName := storage.GetComponentByName(loadedKB, "test-component")
	if componentByName == nil {
		t.Error("Expected to find component by name")
	}

	if componentByName.Name != "test-component" {
		t.Errorf("Expected component name 'test-component', got '%s'", componentByName.Name)
	}
}

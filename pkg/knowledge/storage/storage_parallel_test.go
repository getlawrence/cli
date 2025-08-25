package storage

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

func TestSaveKnowledgeBaseParallel(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_parallel.db"
	defer os.Remove(dbPath)

	// Create storage instance
	logger := &logger.StdoutLogger{}
	storage, err := NewStorage(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	components := createTestComponents(100)

	// Test parallel processing
	startTime := time.Now()
	err = storage.SaveKnowledgeBase(components)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to save knowledge base: %v", err)
	}

	// Use query to count components
	result := storage.QueryKnowledgeBase(Query{})
	if result.Total != len(components) {
		t.Errorf("Expected %d components, got %d", len(components), result.Total)
	}

	// Verify versions were saved by summing versions of returned components
	totalVersions := 0
	for _, component := range result.Components {
		totalVersions += len(component.Versions)
	}
	// If not all components returned due to pagination default, fetch light list and count
	if result.Total != result.Returned {
		light := storage.GetComponentsLight(Query{Limit: result.Total})
		totalVersions = 0
		for _, c := range light.Components {
			versions, _ := storage.LoadComponentVersions(c.Name)
			totalVersions += len(versions)
		}
	}

	expectedVersions := 0
	for _, component := range components {
		expectedVersions += len(component.Versions)
	}

	if totalVersions != expectedVersions {
		t.Errorf("Expected %d versions, got %d", expectedVersions, totalVersions)
	}

	t.Logf("Parallel processing completed in %v for %d components with %d total versions",
		duration, len(components), totalVersions)
}

func TestSaveKnowledgeBaseSequential(t *testing.T) {
	// Create a temporary database file
	dbPath := "test_sequential.db"
	defer os.Remove(dbPath)

	// Create storage instance
	logger := &logger.StdoutLogger{}
	storage, err := NewStorage(dbPath, logger)
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test data with few components (should trigger sequential processing)
	components := createTestComponents(5)

	// Test sequential processing
	startTime := time.Now()
	err = storage.SaveKnowledgeBase(components)
	duration := time.Since(startTime)

	if err != nil {
		t.Fatalf("Failed to save knowledge base: %v", err)
	}

	// Verify the data was saved correctly
	result := storage.QueryKnowledgeBase(Query{})
	if result.Total != len(components) {
		t.Errorf("Expected %d components, got %d", len(components), result.Total)
	}

	t.Logf("Sequential processing completed in %v for %d components", duration, len(components))
}

// createTestComponents creates test components with versions for testing
func createTestComponents(count int) []types.Component {
	components := make([]types.Component, count)

	for i := 0; i < count; i++ {
		component := types.Component{
			Name:         fmt.Sprintf("test-component-%d", i),
			Type:         types.ComponentTypeInstrumentation,
			Category:     types.ComponentCategoryContrib,
			Status:       types.ComponentStatusStable,
			SupportLevel: types.SupportLevelOfficial,
			Language:     types.ComponentLanguageGo,
			Description:  fmt.Sprintf("Test component %d", i),
			Repository:   fmt.Sprintf("https://github.com/test/component-%d", i),
			LastUpdated:  time.Now(),
			Versions:     createTestVersions(10), // 10 versions per component
		}
		components[i] = component
	}

	return components
}

// createTestVersions creates test versions for testing
func createTestVersions(count int) []types.Version {
	versions := make([]types.Version, count)

	for i := 0; i < count; i++ {
		version := types.Version{
			Name:        fmt.Sprintf("v1.%d.%d", i/10, i%10),
			ReleaseDate: time.Now().AddDate(0, 0, -i),
			Status:      types.VersionStatusStable,
			Deprecated:  false,
		}
		versions[i] = version
	}

	return versions
}

func BenchmarkSaveKnowledgeBaseParallel(b *testing.B) {
	// Create a temporary database file
	dbPath := "benchmark_parallel.db"
	defer os.Remove(dbPath)

	// Create storage instance
	logger := &logger.StdoutLogger{}
	storage, err := NewStorage(dbPath, logger)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test data
	components := createTestComponents(100)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear the database for each iteration
		storage.db.Exec("DELETE FROM versions")
		storage.db.Exec("DELETE FROM components")

		err := storage.SaveKnowledgeBase(components)
		if err != nil {
			b.Fatalf("Failed to save knowledge base: %v", err)
		}
	}
}

func BenchmarkSaveKnowledgeBaseSequential(b *testing.B) {
	// Create a temporary database file
	dbPath := "benchmark_sequential.db"
	defer os.Remove(dbPath)

	// Create storage instance
	logger := &logger.StdoutLogger{}
	storage, err := NewStorage(dbPath, logger)
	if err != nil {
		b.Fatalf("Failed to create storage: %v", err)
	}
	defer storage.Close()

	// Create test data with few components to trigger sequential processing
	components := createTestComponents(5)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear the database for each iteration
		storage.db.Exec("DELETE FROM versions")
		storage.db.Exec("DELETE FROM components")

		err := storage.SaveKnowledgeBase(components)
		if err != nil {
			b.Fatalf("Failed to save knowledge base: %v", err)
		}
	}
}

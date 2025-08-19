package pipeline

import (
	"testing"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/providers"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/stretchr/testify/assert"
)

func TestGroupComponentsByRepository(t *testing.T) {
	logger := &logger.StdoutLogger{}
	providerFactory := providers.NewProviderFactory("", logger)
	storageClient, err := storage.NewStorage("knowledge.db", logger)
	if err != nil {
		t.Fatalf("failed to create storage client: %v", err)
	}
	pipeline := NewPipeline(providerFactory, logger, "", storageClient)

	components := []providers.RegistryComponent{
		{
			Name:       "package1",
			Repository: "https://github.com/open-telemetry/opentelemetry-js-contrib",
		},
		{
			Name:       "package2",
			Repository: "https://github.com/open-telemetry/opentelemetry-js-contrib",
		},
		{
			Name:       "package3",
			Repository: "https://github.com/open-telemetry/opentelemetry-js",
		},
		{
			Name:       "package4",
			Repository: "https://github.com/open-telemetry/opentelemetry-js-contrib",
		},
		{
			Name:       "package5",
			Repository: "https://github.com/open-telemetry/opentelemetry-go",
		},
	}

	groups := pipeline.groupComponentsByRepository(components)

	// Should have 3 unique repositories
	assert.Equal(t, 3, len(groups))

	// js-contrib should have 3 packages
	assert.Equal(t, 3, len(groups["github.com/open-telemetry/opentelemetry-js-contrib"]))

	// js should have 1 package
	assert.Equal(t, 1, len(groups["github.com/open-telemetry/opentelemetry-js"]))

	// go should have 1 package
	assert.Equal(t, 1, len(groups["github.com/open-telemetry/opentelemetry-go"]))
}

func TestRepositoryReleasesCache(t *testing.T) {
	cache := NewRepositoryReleasesCache()

	releases := []providers.GitHubRelease{
		{
			TagName:     "v1.0.0",
			Name:        "Release 1.0.0",
			Body:        "First release",
			PublishedAt: time.Now(),
			HTMLURL:     "https://github.com/test/repo/releases/tag/v1.0.0",
		},
		{
			TagName:     "v1.1.0",
			Name:        "Release 1.1.0",
			Body:        "Second release",
			PublishedAt: time.Now(),
			HTMLURL:     "https://github.com/test/repo/releases/tag/v1.1.0",
		},
	}

	repoURL := "https://github.com/test/repo"

	// Test setting and getting
	cache.Set(repoURL, releases)

	retrieved, exists := cache.Get(repoURL)
	assert.True(t, exists)
	assert.Equal(t, 2, len(retrieved))
	assert.Equal(t, "v1.0.0", retrieved[0].TagName)
	assert.Equal(t, "v1.1.0", retrieved[1].TagName)

	// Test non-existent repository
	_, exists = cache.Get("https://github.com/nonexistent/repo")
	assert.False(t, exists)
}

func TestMatchesVersion(t *testing.T) {
	logger := &logger.StdoutLogger{}
	providerFactory := providers.NewProviderFactory("", logger)
	storageClient, err := storage.NewStorage("knowledge.db", logger)
	if err != nil {
		t.Fatalf("failed to create storage client: %v", err)
	}
	pipeline := NewPipeline(providerFactory, logger, "", storageClient)

	// Test exact matches
	assert.True(t, pipeline.matchesVersion("v1.0.0", "1.0.0"))
	assert.True(t, pipeline.matchesVersion("1.0.0", "1.0.0"))
	assert.True(t, pipeline.matchesVersion("v1.0.0", "v1.0.0"))

	// Test semantic versioning
	assert.True(t, pipeline.matchesVersion("v1.0.0-beta.1", "1.0.0"))
	assert.True(t, pipeline.matchesVersion("v1.0.0-alpha.2", "1.0.0"))

	// Test non-matches
	assert.False(t, pipeline.matchesVersion("v1.0.0", "1.1.0"))
	assert.False(t, pipeline.matchesVersion("v2.0.0", "1.0.0"))
}

func TestGetCacheStats(t *testing.T) {
	logger := &logger.StdoutLogger{}
	providerFactory := providers.NewProviderFactory("", logger)
	storageClient, err := storage.NewStorage("knowledge.db", logger)
	if err != nil {
		t.Fatalf("failed to create storage client: %v", err)
	}
	pipeline := NewPipeline(providerFactory, logger, "", storageClient)

	// Initially should be empty
	stats := pipeline.GetCacheStats()
	assert.Equal(t, 0, stats["cached_repositories"])
	assert.Equal(t, 0, stats["total_cached_releases"])

	// Add some test data
	releases := []providers.GitHubRelease{
		{TagName: "v1.0.0"},
		{TagName: "v1.1.0"},
	}

	pipeline.releasesCache.Set("https://github.com/test/repo1", releases)
	pipeline.releasesCache.Set("https://github.com/test/repo2", releases)

	stats = pipeline.GetCacheStats()
	assert.Equal(t, 2, stats["cached_repositories"])
	assert.Equal(t, 4, stats["total_cached_releases"])
}

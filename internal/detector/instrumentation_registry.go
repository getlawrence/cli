package detector

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/detector/types"
	"gopkg.in/yaml.v3"
)

// InstrumentationRegistryService handles communication with the OpenTelemetry instrumentation registry
type InstrumentationRegistryService struct {
	baseURL string
	client  *http.Client
}

// NewInstrumentationRegistryService creates a new instrumentation registry service
func NewInstrumentationRegistryService() *InstrumentationRegistryService {
	return &InstrumentationRegistryService{
		baseURL: "https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/data/registry",
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// RegistryEntry represents the YAML structure from the OpenTelemetry registry
type RegistryEntry struct {
	Title        string   `yaml:"title"`
	RegistryType string   `yaml:"registryType"`
	Language     string   `yaml:"language"`
	Tags         []string `yaml:"tags"`
	Description  string   `yaml:"description"`
	License      string   `yaml:"license"`
	Authors      []struct {
		Name string `yaml:"name"`
	} `yaml:"authors"`
	URLs struct {
		Repo string `yaml:"repo"`
	} `yaml:"urls"`
	CreatedAt    string `yaml:"createdAt"`
	IsFirstParty bool   `yaml:"isFirstParty"`
}

// GetInstrumentation checks if instrumentation exists for a given package
func (s *InstrumentationRegistryService) GetInstrumentation(ctx context.Context, pkg types.Package) (*types.InstrumentationInfo, error) {
	// Convert package name to registry format
	packageName := s.PackageName(pkg.Name)

	// Construct URL for instrumentation file
	url := fmt.Sprintf("%s/instrumentation-%s-%s.yml", s.baseURL, pkg.Language, packageName)

	// Create HTTP request with context
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Execute request
	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch instrumentation data: %w", err)
	}
	defer resp.Body.Close()

	// Check if instrumentation exists
	if resp.StatusCode == 404 {
		return nil, nil // No instrumentation available
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read and parse YAML
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var entry RegistryEntry
	if err := yaml.Unmarshal(body, &entry); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to types.InstrumentationInfo
	info := &types.InstrumentationInfo{
		Package:      pkg,
		Title:        entry.Title,
		Description:  entry.Description,
		RegistryType: entry.RegistryType,
		Language:     entry.Language,
		Tags:         entry.Tags,
		License:      entry.License,
		CreatedAt:    entry.CreatedAt,
		IsFirstParty: entry.IsFirstParty,
		IsAvailable:  true,
		RegistryURL:  url,
	}

	// Convert authors
	for _, author := range entry.Authors {
		info.Authors = append(info.Authors, types.Author{Name: author.Name})
	}

	// Set URLs
	info.URLs = types.URLs{Repo: entry.URLs.Repo}

	return info, nil
}

// normalizetypes.PackageName converts package names to the format used in the registry
func (s *InstrumentationRegistryService) PackageName(packageName string) string {
	// Handle common package name formats
	name := strings.ToLower(packageName)

	// Remove common prefixes/suffixes
	name = strings.TrimPrefix(name, "github.com/")
	name = strings.TrimPrefix(name, "go.opentelemetry.io/")
	name = strings.TrimSuffix(name, ".git")

	// Replace slashes and dots with hyphens (common in registry naming)
	name = strings.ReplaceAll(name, "/", "-")
	name = strings.ReplaceAll(name, ".", "-")
	name = strings.ReplaceAll(name, "_", "-")

	return name
}

// GetAvailableInstrumentations returns all instrumentations for a given language
func (s *InstrumentationRegistryService) GetAvailableInstrumentations(ctx context.Context, language string) ([]types.InstrumentationInfo, error) {
	// This would require listing all instrumentation files for a language
	// For now, we'll implement package-specific lookup only
	// This could be extended to fetch directory listings or use a registry API
	return nil, fmt.Errorf("listing all instrumentations not yet implemented")
}

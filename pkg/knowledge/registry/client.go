package registry

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"github.com/google/go-github/v74/github"
	"gopkg.in/yaml.v3"
)

const (
	// RegistryBaseURL is the base URL for the OpenTelemetry Registry API
	RegistryBaseURL = "https://raw.githubusercontent.com/open-telemetry/opentelemetry.io/main/data/registry"
	// RegistryAPIPath is the path for the registry API
	RegistryAPIPath = ""
)

// Client represents a client for the OpenTelemetry Registry API
type Client struct {
	baseURL      string
	httpClient   *http.Client
	logger       logger.Logger
	githubClient *github.Client // Use official GitHub client
}

// NewClient creates a new registry client with all parameters explicitly declared
func NewClient(githubToken string, logger logger.Logger, baseURL string) *Client {
	var githubClient *github.Client
	if githubToken != "" {
		githubClient = github.NewClient(nil).WithAuthToken(githubToken)
	} else {
		githubClient = github.NewClient(nil)
	}

	return &Client{
		baseURL:      baseURL,
		httpClient:   &http.Client{Timeout: 30 * time.Second},
		logger:       logger,
		githubClient: githubClient,
	}
}

// RegistryComponent represents a component from the registry
type RegistryComponent struct {
	Name        string                 `json:"name"`
	Type        string                 `json:"type"`
	Language    string                 `json:"language"`
	Description string                 `json:"description"`
	Repository  string                 `json:"repository"`
	RegistryURL string                 `json:"registry_url"`
	Homepage    string                 `json:"homepage"`
	Tags        []string               `json:"tags"`
	Maintainers []string               `json:"maintainers"`
	License     string                 `json:"license"`
	LastUpdated time.Time              `json:"last_updated"`
	Metadata    map[string]interface{} `json:"metadata"`
}

// RegistryResponse represents the response from the registry API
type RegistryResponse struct {
	Components []RegistryComponent `json:"components"`
	Total      int                 `json:"total"`
	Page       int                 `json:"page"`
	PerPage    int                 `json:"per_page"`
}

// GetJavaScriptComponents fetches all JavaScript components from the registry
func (c *Client) GetJavaScriptComponents() ([]RegistryComponent, error) {
	return c.GetComponentsByLanguage("javascript")
}

// GitHubContent represents a file or directory in the GitHub repository
type GitHubContent struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

// RegistryYAML represents the structure of registry YAML files
type RegistryYAML struct {
	Title        string   `yaml:"title"`
	RegistryType string   `yaml:"registryType"`
	Language     string   `yaml:"language"`
	Tags         []string `yaml:"tags"`
	License      string   `yaml:"license"`
	Description  string   `yaml:"description"`
	Authors      []struct {
		Name string `yaml:"name"`
	} `yaml:"authors"`
	URLs struct {
		Repo string `yaml:"repo"`
	} `yaml:"urls"`
	CreatedAt    string `yaml:"createdAt"`
	IsFirstParty bool   `yaml:"isFirstParty"`
	Package      struct {
		Registry string `yaml:"registry"`
		Name     string `yaml:"name"`
		Version  string `yaml:"version"`
	} `yaml:"package"`
}

// GetComponentsByLanguage fetches components by language from the registry
func (c *Client) GetComponentsByLanguage(language string) ([]RegistryComponent, error) {
	// Check if logger is initialized to prevent panic
	if c.logger == nil {
		return nil, fmt.Errorf("logger is not initialized")
	}

	// Map language to GitHub language identifier
	langMap := map[string]string{
		"javascript": "js",
		"js":         "js",
		"go":         "go",
		"python":     "python",
		"java":       "java",
		"csharp":     "dotnet",
		"php":        "php",
		"ruby":       "ruby",
	}

	githubLang := langMap[language]
	if githubLang == "" {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	// Check if githubClient is initialized to prevent panic
	if c.githubClient == nil {
		return nil, fmt.Errorf("GitHub client is not initialized")
	}

	// Fetch the list of files from GitHub API using the official client
	ctx := context.Background()
	var rawContents interface{}
	var err error
	rawContents, _, _, err = c.githubClient.Repositories.GetContents(ctx, "open-telemetry", "opentelemetry.io", "data/registry", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry index: %w", err)
	}

	if rawContents == nil {
		return []RegistryComponent{}, nil
	}

	var components []RegistryComponent

	// GetContents returns interface{} - we need to type assert it
	var contents []*github.RepositoryContent
	switch v := rawContents.(type) {
	case []*github.RepositoryContent:
		contents = v
	case *github.RepositoryContent:
		contents = []*github.RepositoryContent{v}
	default:
		return nil, fmt.Errorf("unexpected content type from GitHub API: %T", rawContents)
	}

	// Filter for files that match the language and are YAML files
	for _, content := range contents {
		// Check if content is nil (GitHub API can return nil elements)
		if content == nil {
			continue
		}

		if content.Type != nil && *content.Type == "file" && content.Name != nil && strings.HasSuffix(*content.Name, ".yml") {
			// Check if the filename contains the language identifier
			if strings.Contains(*content.Name, "-"+githubLang+"-") ||
				strings.Contains(*content.Name, githubLang+"-") ||
				strings.HasPrefix(*content.Name, githubLang+"-") {

				// Check if DownloadURL is available
				if content.DownloadURL == nil {
					c.logger.Logf("Warning: DownloadURL is nil for %s, skipping", *content.Name)
					continue
				}

				// Fetch and parse the YAML file
				component, err := c.fetchComponentFromYAML(*content.DownloadURL)
				if err != nil {
					// Log error but continue with other files
					c.logger.Logf("Warning: failed to parse %s: %v\n", *content.Name, err)
					continue
				}

				if component != nil {
					components = append(components, *component)
				}
			}
		}
	}

	return components, nil
}

// fetchComponentFromYAML fetches and parses a single YAML file
func (c *Client) fetchComponentFromYAML(downloadURL string) (*RegistryComponent, error) {
	req, err := http.NewRequest("GET", downloadURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set User-Agent to avoid being blocked
	req.Header.Set("User-Agent", "Lawrence-CLI/1.0")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch YAML file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch YAML file, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read YAML file: %w", err)
	}

	var yamlData RegistryYAML
	if err := yaml.Unmarshal(body, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to RegistryComponent
	component := &RegistryComponent{
		Name:        yamlData.Package.Name,
		Type:        yamlData.RegistryType,
		Language:    yamlData.Language,
		Description: yamlData.Description,
		Repository:  yamlData.URLs.Repo,
		RegistryURL: downloadURL,
		Homepage:    yamlData.URLs.Repo,
		Tags:        yamlData.Tags,
		License:     yamlData.License,
		LastUpdated: time.Now(), // We could parse createdAt if needed
		Metadata: map[string]interface{}{
			"title":        yamlData.Title,
			"isFirstParty": yamlData.IsFirstParty,
			"package":      yamlData.Package,
			"createdAt":    yamlData.CreatedAt,
		},
	}

	return component, nil
}

// GetComponentByName fetches a specific component by name
func (c *Client) GetComponentByName(name string) (*RegistryComponent, error) {
	// For now, this would need to be implemented to search through all components
	// and find the one with the matching name. For now, return nil to avoid errors.
	// This could be enhanced to cache components or implement a more efficient search.
	return nil, nil
}

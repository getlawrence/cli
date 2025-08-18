package registry

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlawrence/cli/internal/logger"
	"gopkg.in/yaml.v3"
)

// LocalClient represents a client that works with a local copy of the OpenTelemetry registry
type Client struct {
	registryPath string
	logger       logger.Logger
	cache        map[string][]RegistryComponent // language -> components cache
}

// NewLocalClient creates a new local registry client
func NewClient(registryPath string, logger logger.Logger) *Client {
	return &Client{
		registryPath: registryPath,
		logger:       logger,
		cache:        make(map[string][]RegistryComponent),
	}
}

// LocalClient uses the existing RegistryComponent and RegistryYAML types from client.go

// GetComponentsByLanguage fetches components by language from the local registry
func (c *Client) GetComponentsByLanguage(language string) ([]RegistryComponent, error) {
	// Check cache first
	if components, exists := c.cache[language]; exists {
		return components, nil
	}

	// Map language to directory name
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

	// Build the path to the language-specific registry directory
	langPath := filepath.Join(c.registryPath, "data", "registry")

	// Check if the registry path exists
	if _, err := os.Stat(langPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("registry path does not exist: %s. Please run 'lawrence registry sync' first", langPath)
	}

	var components []RegistryComponent

	// Walk through all YAML files in the registry directory
	err := filepath.Walk(langPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Only process YAML files
		if !strings.HasSuffix(strings.ToLower(info.Name()), ".yml") && !strings.HasSuffix(strings.ToLower(info.Name()), ".yaml") {
			return nil
		}

		// Check if the filename contains the language identifier
		if strings.Contains(info.Name(), "-"+githubLang+"-") ||
			strings.Contains(info.Name(), githubLang+"-") ||
			strings.HasPrefix(info.Name(), githubLang+"-") {

			// Parse the YAML file
			component, err := c.parseComponentFromFile(path)
			if err != nil {
				c.logger.Logf("Warning: failed to parse %s: %v", info.Name(), err)
				return nil // Continue with other files
			}

			if component != nil {
				components = append(components, *component)
			}
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to walk registry directory: %w", err)
	}

	// Cache the results
	c.cache[language] = components

	return components, nil
}

// parseComponentFromFile parses a single YAML file and returns a RegistryComponent
func (c *Client) parseComponentFromFile(filePath string) (*RegistryComponent, error) {
	// Read the file
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	// Read file content
	content, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse YAML
	var yamlData RegistryYAML
	if err := yaml.Unmarshal(content, &yamlData); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Convert to RegistryComponent
	component := &RegistryComponent{
		Name:        yamlData.Package.Name,
		Type:        yamlData.RegistryType,
		Language:    yamlData.Language,
		Description: yamlData.Description,
		Repository:  yamlData.URLs.Repo,
		RegistryURL: filePath, // Use local file path
		Homepage:    yamlData.URLs.Repo,
		Tags:        yamlData.Tags,
		License:     yamlData.License,
		LastUpdated: time.Now(), // Could parse createdAt if needed
		Metadata: map[string]interface{}{
			"title":        yamlData.Title,
			"isFirstParty": yamlData.IsFirstParty,
			"package":      yamlData.Package,
			"createdAt":    yamlData.CreatedAt,
			"sourceFile":   filePath,
		},
	}

	return component, nil
}

// GetComponentByName fetches a specific component by name from all languages
func (c *Client) GetComponentByName(name string) (*RegistryComponent, error) {
	// Check all supported languages
	languages := []string{"javascript", "go", "python", "java", "csharp", "php", "ruby"}

	for _, lang := range languages {
		components, err := c.GetComponentsByLanguage(lang)
		if err != nil {
			continue // Skip languages with errors
		}

		for _, component := range components {
			if component.Name == name {
				return &component, nil
			}
		}
	}

	return nil, fmt.Errorf("component not found: %s", name)
}

// GetAllComponents fetches all components from all languages
func (c *Client) GetAllComponents() ([]RegistryComponent, error) {
	var allComponents []RegistryComponent
	languages := []string{"javascript", "go", "python", "java", "csharp", "php", "ruby"}

	for _, lang := range languages {
		components, err := c.GetComponentsByLanguage(lang)
		if err != nil {
			c.logger.Logf("Warning: failed to get components for %s: %v", lang, err)
			continue
		}
		allComponents = append(allComponents, components...)
	}

	return allComponents, nil
}

// GetSupportedLanguages returns the list of supported languages
func (c *Client) GetSupportedLanguages() []string {
	return []string{"javascript", "go", "python", "java", "csharp", "php", "ruby"}
}

// GetRegistryStats returns statistics about the local registry
func (c *Client) GetRegistryStats() (map[string]interface{}, error) {
	stats := make(map[string]interface{})

	// Count total files
	totalFiles := 0
	err := filepath.Walk(filepath.Join(c.registryPath, "data", "registry"), func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && (strings.HasSuffix(strings.ToLower(info.Name()), ".yml") || strings.HasSuffix(strings.ToLower(info.Name()), ".yaml")) {
			totalFiles++
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to count registry files: %w", err)
	}

	stats["totalFiles"] = totalFiles
	stats["registryPath"] = c.registryPath
	stats["lastUpdated"] = time.Now()

	// Count by language
	langStats := make(map[string]int)
	for _, lang := range c.GetSupportedLanguages() {
		components, err := c.GetComponentsByLanguage(lang)
		if err == nil {
			langStats[lang] = len(components)
		}
	}
	stats["byLanguage"] = langStats

	return stats, nil
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

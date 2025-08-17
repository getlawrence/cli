package npm

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	// NPMRegistryBaseURL is the base URL for the npm registry API
	NPMRegistryBaseURL = "https://registry.npmjs.org"
)

// Client represents a client for the npm registry API
type Client struct {
	baseURL    string
	httpClient *http.Client
}

// NewClient creates a new npm client
func NewClient() *Client {
	return &Client{
		baseURL: NPMRegistryBaseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// NewClientWithBaseURL creates a new npm client with a custom base URL
func NewClientWithBaseURL(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// Package represents npm package metadata
type Package struct {
	Name                 string               `json:"name"`
	Description          string               `json:"description"`
	Version              string               `json:"version"`
	Homepage             string               `json:"homepage"`
	Repository           Repository           `json:"repository"`
	Author               interface{}          `json:"author"`
	License              string               `json:"license"`
	Keywords             []string             `json:"keywords"`
	Main                 string               `json:"main"`
	Types                string               `json:"types"`
	Scripts              map[string]string    `json:"scripts"`
	Dependencies         map[string]string    `json:"dependencies"`
	DevDependencies      map[string]string    `json:"devDependencies"`
	PeerDependencies     map[string]string    `json:"peerDependencies"`
	OptionalDependencies map[string]string    `json:"optionalDependencies"`
	Engines              map[string]string    `json:"engines"`
	OS                   []string             `json:"os"`
	CPU                  []string             `json:"cpu"`
	DistTags             map[string]string    `json:"dist-tags"`
	Time                 map[string]time.Time `json:"time"`
	Versions             map[string]Version   `json:"versions"`
	Maintainers          []Maintainer         `json:"maintainers"`
	Contributors         []Maintainer         `json:"contributors"`
	Bugs                 Bugs                 `json:"bugs"`
	Readme               string               `json:"readme"`
	ID                   string               `json:"_id"`
	Rev                  string               `json:"_rev"`
}

// Repository represents the repository information
type Repository struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// Version represents a specific version of a package
type Version struct {
	Name                 string            `json:"name"`
	Version              string            `json:"version"`
	Description          string            `json:"description"`
	Main                 string            `json:"main"`
	Types                string            `json:"types"`
	Scripts              map[string]string `json:"scripts"`
	Dependencies         map[string]string `json:"dependencies"`
	DevDependencies      map[string]string `json:"devDependencies"`
	PeerDependencies     map[string]string `json:"peerDependencies"`
	OptionalDependencies map[string]string `json:"optionalDependencies"`
	Engines              map[string]string `json:"engines"`
	OS                   []string          `json:"os"`
	CPU                  []string          `json:"cpu"`
	Dist                 Dist              `json:"dist"`
	Repository           Repository        `json:"repository"`
	Homepage             string            `json:"homepage"`
	License              string            `json:"license"`
	Keywords             []string          `json:"keywords"`
	Author               interface{}       `json:"author"`
	Maintainers          []Maintainer      `json:"maintainers"`
	Contributors         []Maintainer      `json:"contributors"`
	Bugs                 Bugs              `json:"bugs"`
	Readme               string            `json:"readme"`
	ID                   string            `json:"_id"`
	Rev                  string            `json:"_rev"`
}

// Dist represents distribution information
type Dist struct {
	Integrity    string `json:"integrity"`
	Shasum       string `json:"shasum"`
	Tarball      string `json:"tarball"`
	FileCount    int    `json:"fileCount"`
	UnpackedSize int    `json:"unpackedSize"`
}

// Maintainer represents a package maintainer
type Maintainer struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// Bugs represents bug reporting information
type Bugs struct {
	URL   string `json:"url"`
	Email string `json:"email"`
}

// GetPackage fetches package metadata from npm
func (c *Client) GetPackage(name string) (*Package, error) {
	url := fmt.Sprintf("%s/%s", c.baseURL, name)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package %s: %w", name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm registry returned status %d for package %s: %s", resp.StatusCode, name, string(body))
	}

	var pkg Package
	if err := json.NewDecoder(resp.Body).Decode(&pkg); err != nil {
		return nil, fmt.Errorf("failed to decode package response: %w", err)
	}

	return &pkg, nil
}

// GetPackageVersion fetches a specific version of a package
func (c *Client) GetPackageVersion(name, version string) (*Version, error) {
	url := fmt.Sprintf("%s/%s/%s", c.baseURL, name, version)

	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch package %s@%s: %w", name, version, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("npm registry returned status %d for package %s@%s: %s", resp.StatusCode, name, version, string(body))
	}

	var ver Version
	if err := json.NewDecoder(resp.Body).Decode(&ver); err != nil {
		return nil, fmt.Errorf("failed to decode package version response: %w", err)
	}

	return &ver, nil
}

// GetLatestVersion fetches the latest version of a package
func (c *Client) GetLatestVersion(name string) (*Version, error) {
	pkg, err := c.GetPackage(name)
	if err != nil {
		return nil, err
	}

	latestVersion, ok := pkg.DistTags["latest"]
	if !ok {
		return nil, fmt.Errorf("no latest version found for package %s", name)
	}

	latest, ok := pkg.Versions[latestVersion]
	if !ok {
		return nil, fmt.Errorf("latest version %s not found in versions for package %s", latestVersion, name)
	}

	return &latest, nil
}

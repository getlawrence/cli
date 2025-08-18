package providers

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/utils"
	"github.com/google/go-github/v74/github"
)

// GitHubClient represents a client for GitHub API operations
type GitHubClient struct {
	githubClient *github.Client
	rateLimiter  *utils.RateLimiter
}

// GitHubRelease represents a GitHub release
type GitHubRelease struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	Body        string    `json:"body"`
	PublishedAt time.Time `json:"published_at"`
	Prerelease  bool      `json:"prerelease"`
	URL         string    `json:"url"`
	HTMLURL     string    `json:"html_url"`
	AssetsURL   string    `json:"assets_url"`
	UploadURL   string    `json:"upload_url"`
	TarballURL  string    `json:"tarball_url"`
	ZipballURL  string    `json:"zipball_url"`
}

// NewGitHubClient creates a new GitHub client with all parameters explicitly declared
func NewGitHubClient(githubToken string) *GitHubClient {
	var client *github.Client
	var rateLimiter *utils.RateLimiter

	if githubToken != "" {
		client = github.NewClient(nil).WithAuthToken(githubToken)
		rateLimiter = utils.NewRateLimiter(5000, time.Hour) // Authenticated: 5000 requests per hour
	} else {
		client = github.NewClient(nil)
		rateLimiter = utils.NewRateLimiter(30, time.Minute) // Default: 30 requests per hour
	}

	return &GitHubClient{
		githubClient: client,
		rateLimiter:  rateLimiter,
	}
}

// GetReleaseNotes fetches release notes for a specific version from a GitHub repository
func (g *GitHubClient) GetReleaseNotes(ctx context.Context, repositoryURL, version string) (*ReleaseNotes, error) {
	// Extract owner and repo from repository URL
	owner, repo, err := g.ExtractOwnerAndRepo(repositoryURL)
	if err != nil {
		return nil, fmt.Errorf("failed to extract owner and repo: %w", err)
	}

	// Wait for rate limiter
	g.rateLimiter.Wait()

	// Fetch releases from GitHub API
	releases, err := g.FetchReleases(ctx, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	// Find the matching release
	var matchingRelease *GitHubRelease
	for _, release := range releases {
		if g.matchesVersionSimple(release.TagName, version) {
			matchingRelease = &release
			break
		}
	}

	if matchingRelease == nil {
		return &ReleaseNotes{
			Version:     version,
			ReleaseDate: time.Time{},
			Notes:       "",
			URL:         "",
			Found:       false,
		}, nil
	}

	return &ReleaseNotes{
		Version:     matchingRelease.TagName,
		ReleaseDate: matchingRelease.PublishedAt,
		Notes:       matchingRelease.Body,
		URL:         matchingRelease.HTMLURL,
		Found:       true,
	}, nil
}

// ExtractOwnerAndRepo extracts the owner and repository name from a GitHub URL
func (g *GitHubClient) ExtractOwnerAndRepo(repositoryURL string) (string, string, error) {
	// Handle different GitHub URL formats
	url := strings.TrimSuffix(repositoryURL, ".git")

	// https://github.com/owner/repo
	if strings.Contains(url, "github.com") {
		parts := strings.Split(url, "github.com/")
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid GitHub URL format: %s", repositoryURL)
		}

		pathParts := strings.Split(parts[1], "/")
		if len(pathParts) < 2 {
			return "", "", fmt.Errorf("invalid GitHub URL path: %s", repositoryURL)
		}

		return pathParts[0], pathParts[1], nil
	}

	return "", "", fmt.Errorf("unsupported repository URL format: %s", repositoryURL)
}

// matchesVersionSimple checks if a GitHub tag matches the requested version (simplified version)
func (g *GitHubClient) matchesVersionSimple(tagName, version string) bool {
	// Remove 'v' prefix if present
	tag := strings.TrimPrefix(tagName, "v")
	ver := strings.TrimPrefix(version, "v")

	// Direct match
	if tag == ver {
		return true
	}

	// Handle cases where GitHub tag might have additional prefixes/suffixes
	if strings.Contains(tag, ver) || strings.Contains(ver, tag) {
		return true
	}

	return false
}

// FetchReleases fetches all releases from a GitHub repository
func (g *GitHubClient) FetchReleases(ctx context.Context, owner, repo string) ([]GitHubRelease, error) {
	releases, _, err := g.githubClient.Repositories.ListReleases(ctx, owner, repo, &github.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch releases: %w", err)
	}

	var githubReleases []GitHubRelease
	for _, release := range releases {
		githubReleases = append(githubReleases, GitHubRelease{
			TagName:     release.GetTagName(),
			Name:        release.GetName(),
			Body:        release.GetBody(),
			PublishedAt: release.GetPublishedAt().Time,
			Prerelease:  release.GetPrerelease(),
			URL:         release.GetURL(),
			HTMLURL:     release.GetHTMLURL(),
			AssetsURL:   release.GetAssetsURL(),
			UploadURL:   release.GetUploadURL(),
			TarballURL:  release.GetTarballURL(),
			ZipballURL:  release.GetZipballURL(),
		})
	}

	return githubReleases, nil
}

// ReleaseNotes represents release notes for a specific version
type ReleaseNotes struct {
	Version     string    `json:"version"`
	ReleaseDate time.Time `json:"release_date"`
	Notes       string    `json:"notes"`
	URL         string    `json:"url"`
	Found       bool      `json:"found"`
}

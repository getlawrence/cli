package providers

import (
	"testing"
)

func TestExtractOwnerAndRepo(t *testing.T) {
	client := NewGitHubClient("")

	tests := []struct {
		name      string
		url       string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "standard github url",
			url:       "https://github.com/open-telemetry/opentelemetry-js",
			wantOwner: "open-telemetry",
			wantRepo:  "opentelemetry-js",
			wantErr:   false,
		},
		{
			name:      "standard github inner path",
			url:       "https://github.com/open-telemetry/opentelemetry-js/tree/main/packages/opentelemetry-api",
			wantOwner: "open-telemetry",
			wantRepo:  "opentelemetry-js",
			wantErr:   false,
		},
		{
			name:      "github url with .git suffix",
			url:       "https://github.com/open-telemetry/opentelemetry-js.git",
			wantOwner: "open-telemetry",
			wantRepo:  "opentelemetry-js",
			wantErr:   false,
		},
		{
			name:    "invalid github url",
			url:     "https://github.com/invalid",
			wantErr: true,
		},
		{
			name:    "non-github url",
			url:     "https://gitlab.com/user/repo",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := client.ExtractOwnerAndRepo(tt.url)
			if (err != nil) != tt.wantErr {
				t.Errorf("extractOwnerAndRepo() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if owner != tt.wantOwner {
					t.Errorf("extractOwnerAndRepo() owner = %v, want %v", owner, tt.wantOwner)
				}
				if repo != tt.wantRepo {
					t.Errorf("extractOwnerAndRepo() repo = %v, want %v", repo, tt.wantRepo)
				}
			}
		})
	}
}

func TestMatchesVersion(t *testing.T) {
	client := NewGitHubClient("")

	tests := []struct {
		name      string
		tagName   string
		version   string
		wantMatch bool
	}{
		{
			name:      "exact match",
			tagName:   "v1.2.3",
			version:   "1.2.3",
			wantMatch: true,
		},
		{
			name:      "exact match with v prefix",
			tagName:   "v1.2.3",
			version:   "v1.2.3",
			wantMatch: true,
		},
		{
			name:      "no match",
			tagName:   "v1.2.3",
			version:   "1.2.4",
			wantMatch: false,
		},
		{
			name:      "partial match",
			tagName:   "v1.2.3-beta",
			version:   "1.2.3",
			wantMatch: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := client.matchesVersionSimple(tt.tagName, tt.version)
			if got != tt.wantMatch {
				t.Errorf("matchesVersion() = %v, want %v", got, tt.wantMatch)
			}
		})
	}
}

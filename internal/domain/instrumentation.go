package domain

// Library represents an OpenTelemetry library or package
type Library struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Language    string `json:"language"`
	ImportPath  string `json:"import_path,omitempty"`
	PackageFile string `json:"package_file,omitempty"`
}

// Package represents a regular package/dependency
type Package struct {
	Name        string `json:"name"`
	Version     string `json:"version,omitempty"`
	Language    string `json:"language"`
	ImportPath  string `json:"import_path,omitempty"`
	PackageFile string `json:"package_file,omitempty"`
}

// InstrumentationInfo represents available instrumentation for a package
type InstrumentationInfo struct {
	Package      Package  `json:"package"`
	Title        string   `json:"title"`
	Description  string   `json:"description"`
	RegistryType string   `json:"registry_type"`
	Language     string   `json:"language"`
	Tags         []string `json:"tags,omitempty"`
	License      string   `json:"license,omitempty"`
	Authors      []Author `json:"authors,omitempty"`
	URLs         URLs     `json:"urls,omitempty"`
	CreatedAt    string   `json:"created_at,omitempty"`
	IsFirstParty bool     `json:"is_first_party"`
	IsAvailable  bool     `json:"is_available"`
	RegistryURL  string   `json:"registry_url,omitempty"`
}

// Author represents an author in instrumentation metadata
type Author struct {
	Name string `json:"name"`
}

// URLs represents URLs in instrumentation metadata
type URLs struct {
	Repo string `json:"repo,omitempty"`
}

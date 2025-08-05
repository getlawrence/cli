package detector

import (
	"context"
	"fmt"
	"path/filepath"
)

// Language represents a programming language detector
type Language interface {
	// Name returns the language name
	Name() string
	// Detect checks if files in the path use this language
	Detect(ctx context.Context, rootPath string) (bool, error)
	// GetOTelLibraries finds OpenTelemetry libraries used in the codebase
	GetOTelLibraries(ctx context.Context, rootPath string) ([]Library, error)
	// GetAllPackages finds all packages/dependencies used in the codebase
	GetAllPackages(ctx context.Context, rootPath string) ([]Package, error)
	// GetFilePatterns returns file patterns this language detector should scan
	GetFilePatterns() []string
}

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

// Issue represents a detected problem or recommendation
type Issue struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Category    Category `json:"category"`
	Language    string   `json:"language,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
	References  []string `json:"references,omitempty"`
}

// Severity levels for issues
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Category represents the type of issue
type Category string

const (
	CategoryMissingLibrary  Category = "missing_library"
	CategoryConfiguration   Category = "configuration"
	CategoryInstrumentation Category = "instrumentation"
	CategoryPerformance     Category = "performance"
	CategorySecurity        Category = "security"
	CategoryBestPractice    Category = "best_practice"
	CategoryDeprecated      Category = "deprecated"
)

// IssueDetector defines how to detect specific issues
type IssueDetector interface {
	// ID returns a unique identifier for this detector
	ID() string
	// Name returns a human-readable name
	Name() string
	// Description returns what this detector looks for
	Description() string
	// Category returns the issue category
	Category() Category
	// Languages returns which languages this detector applies to (empty = all)
	Languages() []string
	// Detect finds issues in the given context
	Detect(ctx context.Context, analysis *Analysis) ([]Issue, error)
}

// Analysis contains the results of language detection and library discovery
type Analysis struct {
	RootPath                  string                `json:"root_path"`
	DetectedLanguages         []string              `json:"detected_languages"`
	Libraries                 []Library             `json:"libraries"`
	Packages                  []Package             `json:"packages"`
	AvailableInstrumentations []InstrumentationInfo `json:"available_instrumentations"`
	FilesByLanguage           map[string][]string   `json:"files_by_language"`
}

// Manager coordinates the detection process
type Manager struct {
	languages []Language
	detectors []IssueDetector
}

// NewManager creates a new detection manager
func NewManager() *Manager {
	return &Manager{
		languages: make([]Language, 0),
		detectors: make([]IssueDetector, 0),
	}
}

// RegisterLanguage adds a language detector
func (m *Manager) RegisterLanguage(lang Language) {
	m.languages = append(m.languages, lang)
}

// RegisterDetector adds an issue detector
func (m *Manager) RegisterDetector(detector IssueDetector) {
	m.detectors = append(m.detectors, detector)
}

// AnalyzeCodebase performs the full analysis
func (m *Manager) AnalyzeCodebase(ctx context.Context, rootPath string) (*Analysis, []Issue, error) {
	analysis := &Analysis{
		RootPath:        rootPath,
		FilesByLanguage: make(map[string][]string),
	}

	// Detect languages
	for _, lang := range m.languages {
		detected, err := lang.Detect(ctx, rootPath)
		if err != nil {
			return nil, nil, err
		}
		if detected {
			analysis.DetectedLanguages = append(analysis.DetectedLanguages, lang.Name())

			// Get OTel libraries for this language
			libs, err := lang.GetOTelLibraries(ctx, rootPath)
			if err != nil {
				return nil, nil, err
			}
			analysis.Libraries = append(analysis.Libraries, libs...)

			// Get all packages for this language
			packages, err := lang.GetAllPackages(ctx, rootPath)
			if err != nil {
				return nil, nil, err
			}
			analysis.Packages = append(analysis.Packages, packages...)

			// Collect files for this language
			files, err := m.getFilesForLanguage(rootPath, lang)
			if err != nil {
				return nil, nil, err
			}
			analysis.FilesByLanguage[lang.Name()] = files
		}
	}

	// Check for available instrumentations
	instrumentationService := NewInstrumentationRegistryService()
	seenInstrumentations := make(map[string]bool)

	for _, pkg := range analysis.Packages {
		instrumentation, err := instrumentationService.GetInstrumentation(ctx, pkg)
		if err != nil {
			// Log error but continue - instrumentation lookup is optional
			continue
		}
		if instrumentation != nil {
			// Create a unique key to avoid duplicates
			key := fmt.Sprintf("%s-%s", instrumentation.Language, instrumentation.Package.Name)
			if !seenInstrumentations[key] {
				seenInstrumentations[key] = true
				analysis.AvailableInstrumentations = append(analysis.AvailableInstrumentations, *instrumentation)
			}
		}
	}

	// Run issue detectors
	var allIssues []Issue
	for _, detector := range m.detectors {
		// Check if detector applies to detected languages
		if m.detectorApplies(detector, analysis.DetectedLanguages) {
			issues, err := detector.Detect(ctx, analysis)
			if err != nil {
				return nil, nil, err
			}
			allIssues = append(allIssues, issues...)
		}
	}

	return analysis, allIssues, nil
}

// getFilesForLanguage collects files that match the language patterns
func (m *Manager) getFilesForLanguage(rootPath string, lang Language) ([]string, error) {
	var files []string
	patterns := lang.GetFilePatterns()

	for _, pattern := range patterns {
		matches, err := filepath.Glob(filepath.Join(rootPath, pattern))
		if err != nil {
			return nil, err
		}
		files = append(files, matches...)
	}

	return files, nil
}

// detectorApplies checks if a detector should run for the detected languages
func (m *Manager) detectorApplies(detector IssueDetector, detectedLanguages []string) bool {
	detectorLangs := detector.Languages()

	// If detector doesn't specify languages, it applies to all
	if len(detectorLangs) == 0 {
		return true
	}

	// Check if any detected language matches detector languages
	for _, detected := range detectedLanguages {
		for _, required := range detectorLangs {
			if detected == required {
				return true
			}
		}
	}

	return false
}

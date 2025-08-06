package detector

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/detector/types"
)

// Language represents a programming language detector
type Language interface {
	// Name returns the language name
	Name() string
	// GetOTelLibraries finds OpenTelemetry libraries used in the codebase
	GetOTelLibraries(ctx context.Context, rootPath string) ([]types.Library, error)
	// GetAllPackages finds all packages/dependencies used in the codebase
	GetAllPackages(ctx context.Context, rootPath string) ([]types.Package, error)
	// GetFilePatterns returns file patterns this language detector should scan
	GetFilePatterns() []string
}

// IssueDetector defines how to detect specific issues
type IssueDetector interface {
	// ID returns a unique identifier for this detector
	ID() string
	// Name returns a human-readable name
	Name() string
	// Description returns what this detector looks for
	Description() string
	// Category returns the issue category
	Category() types.Category
	// Languages returns which languages this detector applies to (empty = all)
	Languages() []string
	// Detect finds issues in the given context
	Detect(ctx context.Context, analysis *Analysis) ([]types.Issue, error)
}

// Analysis contains the results of language detection and library discovery
type Analysis struct {
	RootPath                  string                      `json:"root_path"`
	DetectedLanguages         []string                    `json:"detected_languages"`
	Libraries                 []types.Library             `json:"libraries"`
	Packages                  []types.Package             `json:"packages"`
	AvailableInstrumentations []types.InstrumentationInfo `json:"available_instrumentations"`
	FilesByLanguage           map[string][]string         `json:"files_by_language"`
}

// Manager coordinates the detection process
type Manager struct {
	detectors         []IssueDetector
	languageDetectors map[string]Language
}

// NewManager creates a new detection manager
func NewManager() *Manager {
	return &Manager{
		detectors:         make([]IssueDetector, 0),
		languageDetectors: make(map[string]Language),
	}
}

// RegisterLanguageDetector adds a language detector
func (m *Manager) RegisterLanguageDetector(name string, detector Language) {
	m.languageDetectors[strings.ToLower(name)] = detector
}

// RegisterDetector adds an issue detector
func (m *Manager) RegisterDetector(detector IssueDetector) {
	m.detectors = append(m.detectors, detector)
}

// AnalyzeCodebase performs the full analysis
func (m *Manager) AnalyzeCodebase(ctx context.Context, rootPath string) (*Analysis, []types.Issue, error) {
	analysis := &Analysis{
		RootPath:        rootPath,
		FilesByLanguage: make(map[string][]string),
	}

	// Use the enhanced language detection to get directory-specific languages
	directoryLanguages, err := DetectLanguages(rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect languages: %w", err)
	}

	// Process languages and collect data
	err = m.processDirectoryLanguages(ctx, rootPath, directoryLanguages, analysis)
	if err != nil {
		return nil, nil, err
	}

	// Check for available instrumentations
	err = m.populateInstrumentations(ctx, analysis)
	if err != nil {
		return nil, nil, err
	}

	// Run issue detectors
	allIssues, err := m.runIssueDetectors(ctx, analysis)
	if err != nil {
		return nil, nil, err
	}

	return analysis, allIssues, nil
}

// processDirectoryLanguages processes each directory with its detected language
func (m *Manager) processDirectoryLanguages(ctx context.Context, rootPath string, directoryLanguages map[string]string, analysis *Analysis) error {
	// Track which languages we've seen to avoid duplicates
	seenLanguages := make(map[string]bool)

	// Process each directory with its detected language
	for directory, language := range directoryLanguages {
		languageDetector := m.findLanguageDetector(language)
		if languageDetector == nil {
			// Skip if we don't have a detector for this language
			continue
		}

		// Calculate the full path for this directory
		dirPath := m.calculateDirectoryPath(rootPath, directory)

		// Only process each language once, but collect all directories
		if !seenLanguages[language] {
			seenLanguages[language] = true
			analysis.DetectedLanguages = append(analysis.DetectedLanguages, language)

			// Get OTel libraries for this language from the specific directory
			err := m.collectLibrariesAndPackages(ctx, dirPath, language, languageDetector, analysis)
			if err != nil {
				return err
			}
		}

		// Always collect files for each directory, regardless of whether we've seen the language
		err := m.collectFilesForLanguage(dirPath, language, languageDetector, analysis)
		if err != nil {
			return err
		}
	}
	return nil
}

// findLanguageDetector finds the corresponding language detector for a language name
func (m *Manager) findLanguageDetector(language string) Language {
	language = strings.ToLower(language)
	return m.languageDetectors[language]
}

// calculateDirectoryPath calculates the full path for a directory
func (m *Manager) calculateDirectoryPath(rootPath, directory string) string {
	if directory == "root" {
		return rootPath
	}
	return filepath.Join(rootPath, directory)
}

// collectLibrariesAndPackages collects OTel libraries and packages for a language
func (m *Manager) collectLibrariesAndPackages(ctx context.Context, dirPath, language string, languageDetector Language, analysis *Analysis) error {
	// Get OTel libraries for this language from the specific directory
	libs, err := languageDetector.GetOTelLibraries(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to get OTel libraries for %s in %s: %w", language, dirPath, err)
	}
	analysis.Libraries = append(analysis.Libraries, libs...)

	// Get all packages for this language from the specific directory
	packages, err := languageDetector.GetAllPackages(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to get packages for %s in %s: %w", language, dirPath, err)
	}
	analysis.Packages = append(analysis.Packages, packages...)

	return nil
}

// collectFilesForLanguage collects files for a specific language and directory
func (m *Manager) collectFilesForLanguage(dirPath, language string, languageDetector Language, analysis *Analysis) error {
	files, err := m.getFilesForLanguage(dirPath, languageDetector)
	if err != nil {
		return fmt.Errorf("failed to get files for %s in %s: %w", language, dirPath, err)
	}

	// Append to existing files for this language
	if analysis.FilesByLanguage[language] == nil {
		analysis.FilesByLanguage[language] = make([]string, 0)
	}
	analysis.FilesByLanguage[language] = append(analysis.FilesByLanguage[language], files...)

	return nil
}

// populateInstrumentations checks for available instrumentations
func (m *Manager) populateInstrumentations(ctx context.Context, analysis *Analysis) error {
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

	return nil
}

// runIssueDetectors runs all registered issue detectors
func (m *Manager) runIssueDetectors(ctx context.Context, analysis *Analysis) ([]types.Issue, error) {
	var allIssues []types.Issue
	for _, detector := range m.detectors {
		// Check if detector applies to detected languages
		if m.detectorApplies(detector, analysis.DetectedLanguages) {
			issues, err := detector.Detect(ctx, analysis)
			if err != nil {
				return nil, err
			}
			allIssues = append(allIssues, issues...)
		}
	}

	return allIssues, nil
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

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
	RootPath                  string                        `json:"root_path"`
	DetectedLanguages         []string                      `json:"detected_languages"`
	Libraries                 []types.Library               `json:"libraries"`
	Packages                  []types.Package               `json:"packages"`
	AvailableInstrumentations []types.InstrumentationInfo   `json:"available_instrumentations"`
	DirectoryAnalyses         map[string]*DirectoryAnalysis `json:"directory_analyses"`
}

// DirectoryAnalysis contains analysis results for a specific directory
type DirectoryAnalysis struct {
	Directory                 string                      `json:"directory"`
	Language                  string                      `json:"language"`
	Libraries                 []types.Library             `json:"libraries"`
	Packages                  []types.Package             `json:"packages"`
	AvailableInstrumentations []types.InstrumentationInfo `json:"available_instrumentations"`
	Issues                    []types.Issue               `json:"issues"`
}

// CodebaseAnalyzer coordinates the detection process
type CodebaseAnalyzer struct {
	detectors         []IssueDetector
	languageDetectors map[string]Language
}

// NewCodebaseAnalyzer creates a new analysis engine
func NewCodebaseAnalyzer(detectors []IssueDetector, languages map[string]Language) *CodebaseAnalyzer {
	return &CodebaseAnalyzer{
		detectors:         detectors,
		languageDetectors: languages,
	}
}

// AnalyzeCodebase performs the full analysis
func (ca *CodebaseAnalyzer) AnalyzeCodebase(ctx context.Context, rootPath string) (*Analysis, error) {
	analysis := &Analysis{
		RootPath:          rootPath,
		DirectoryAnalyses: make(map[string]*DirectoryAnalysis),
	}

	// Use the enhanced language detection to get directory-specific languages
	directoryLanguages, err := DetectLanguages(rootPath)
	if err != nil {
		return nil, fmt.Errorf("failed to detect languages: %w", err)
	}

	// iterate through detected languages in directories
	if len(directoryLanguages) == 0 {
		return nil, fmt.Errorf("no languages detected in the codebase at %s", rootPath)
	}

	seenLanguages := make(map[string]bool)

	for directory, language := range directoryLanguages {
		languageDetector := ca.findLanguageDetector(language)
		if languageDetector == nil {
			// Skip if we don't have a detector for this language
			continue
		}

		// Calculate the full path for this directory
		dirPath := ca.calculateDirectoryPath(rootPath, directory)
		seenLanguages[language] = true

		// Process each directory individually
		dirAnalysis, err := ca.processDirectory(ctx, directory, dirPath, language, languageDetector)
		if err != nil {
			return nil, fmt.Errorf("failed to process directory %s: %w", directory, err)
		}
		analysis.DirectoryAnalyses[directory] = dirAnalysis
	}

	// Set detected languages from seen languages
	for language := range seenLanguages {
		analysis.DetectedLanguages = append(analysis.DetectedLanguages, language)
	}

	return analysis, nil
}

// findLanguageDetector finds the corresponding language detector for a language name
func (ca *CodebaseAnalyzer) findLanguageDetector(language string) Language {
	language = strings.ToLower(language)
	return ca.languageDetectors[language]
}

// calculateDirectoryPath calculates the full path for a directory
func (ca *CodebaseAnalyzer) calculateDirectoryPath(rootPath, directory string) string {
	if directory == "root" {
		return rootPath
	}
	return filepath.Join(rootPath, directory)
}

// processDirectory handles the complete analysis pipeline for a single directory
func (ca *CodebaseAnalyzer) processDirectory(ctx context.Context, directory, dirPath, language string, languageDetector Language) (*DirectoryAnalysis, error) {
	// Step 1: Collect libraries and packages
	libs, packages, err := ca.collectLibrariesAndPackagesForDirectory(ctx, dirPath, language, languageDetector)
	if err != nil {
		return nil, err
	}
	dirAnalysis := &DirectoryAnalysis{
		Directory: directory,
		Libraries: libs,
		Packages:  packages,
	}

	// Step 2: Populate instrumentations
	if err := ca.populateInstrumentationsForDirectory(ctx, dirAnalysis); err != nil {
		return nil, fmt.Errorf("failed to populate instrumentations: %w", err)
	}

	// Step 3: Run issue detectors
	issues, err := ca.runIssueDetectorsForDirectory(ctx, dirAnalysis)
	if err != nil {
		return nil, fmt.Errorf("failed to run issue detectors: %w", err)
	}
	dirAnalysis.Issues = issues
	return dirAnalysis, nil
}

// collectLibrariesAndPackagesForDirectory collects libraries and packages for a specific directory
func (ca *CodebaseAnalyzer) collectLibrariesAndPackagesForDirectory(ctx context.Context, dirPath, language string, languageDetector Language) ([]types.Library, []types.Package, error) {
	libs, err := languageDetector.GetOTelLibraries(ctx, dirPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get OTel libraries for %s in %s: %w", language, dirPath, err)
	}

	packages, err := languageDetector.GetAllPackages(ctx, dirPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get packages for %s in %s: %w", language, dirPath, err)
	}

	return libs, packages, nil
}

// populateInstrumentationsForDirectory populates instrumentations for a specific directory
func (ca *CodebaseAnalyzer) populateInstrumentationsForDirectory(ctx context.Context, dirAnalysis *DirectoryAnalysis) error {
	instrumentationService := NewInstrumentationRegistryService()
	seenInstrumentations := make(map[string]bool)

	for _, pkg := range dirAnalysis.Packages {
		instrumentation, err := instrumentationService.GetInstrumentation(ctx, pkg)
		if err != nil {
			// Log error but continue - instrumentation lookup is optional
			continue
		}
		if instrumentation != nil {
			key := fmt.Sprintf("%s-%s", instrumentation.Language, instrumentation.Package.Name)
			if !seenInstrumentations[key] {
				seenInstrumentations[key] = true
				dirAnalysis.AvailableInstrumentations = append(dirAnalysis.AvailableInstrumentations, *instrumentation)
			}
		}
	}

	return nil
}

// runIssueDetectorsForDirectory runs issue detectors for a specific directory
func (ca *CodebaseAnalyzer) runIssueDetectorsForDirectory(ctx context.Context, dirAnalysis *DirectoryAnalysis) ([]types.Issue, error) {
	var issues []types.Issue

	for _, detector := range ca.detectors {
		if !ca.detectorAppliesForLanguage(detector, dirAnalysis.Language) {
			continue
		}

		// Create temporary analysis for this directory
		tempAnalysis := &Analysis{
			DetectedLanguages:         []string{dirAnalysis.Language},
			Libraries:                 dirAnalysis.Libraries,
			Packages:                  dirAnalysis.Packages,
			AvailableInstrumentations: dirAnalysis.AvailableInstrumentations,
		}

		detectorIssues, err := detector.Detect(ctx, tempAnalysis)
		if err != nil {
			return nil, fmt.Errorf("detector %s failed for directory %s: %w", detector.ID(), dirAnalysis.Directory, err)
		}
		issues = append(issues, detectorIssues...)
	}

	return issues, nil
}

// detectorAppliesForLanguage checks if a detector applies to a specific language
func (ca *CodebaseAnalyzer) detectorAppliesForLanguage(detector IssueDetector, language string) bool {
	detectorLanguages := detector.Languages()

	// If detector doesn't specify languages, it applies to all
	if len(detectorLanguages) == 0 {
		return true
	}

	// Check if the language matches any detector language
	for _, detectorLang := range detectorLanguages {
		if language == detectorLang {
			return true
		}
	}

	return false
}

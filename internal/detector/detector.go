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
func (ca *CodebaseAnalyzer) AnalyzeCodebase(ctx context.Context, rootPath string) (*Analysis, []types.Issue, error) {
	analysis := &Analysis{
		RootPath: rootPath,
	}

	// Use the enhanced language detection to get directory-specific languages
	directoryLanguages, err := DetectLanguages(rootPath)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to detect languages: %w", err)
	}

	// iterate through detected languages in directories
	if len(directoryLanguages) == 0 {
		return nil, nil, fmt.Errorf("no languages detected in the codebase at %s", rootPath)
	}

	for directory, language := range directoryLanguages {
		languageDetector := ca.findLanguageDetector(language)
		if languageDetector == nil {
			// Skip if we don't have a detector for this language
			continue
		}

		// Calculate the full path for this directory
		dirPath := ca.calculateDirectoryPath(rootPath, directory)

		analysis.DetectedLanguages = append(analysis.DetectedLanguages, language)

		// Get OTel libraries for this language from the specific directory
		err, libs, packages := ca.collectLibrariesAndPackages(ctx, dirPath, language, languageDetector)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to collect libraries and packages for %s in %s: %w", language, dirPath, err)
		}
		err = ca.populateInstrumentations(ctx, analysis)
		allIssues, err := ca.runIssueDetectors(ctx, analysis)

	}

	return analysis, allIssues, nil
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

// collectLibrariesAndPackages collects OTel libraries and packages for a language
func (ca *CodebaseAnalyzer) collectLibrariesAndPackages(ctx context.Context, dirPath, language string, languageDetector Language) (error, []types.Library, []types.Package) {
	// Get OTel libraries for this language from the specific directory
	libs, err := languageDetector.GetOTelLibraries(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to get OTel libraries for %s in %s: %w", language, dirPath, err), nil, nil
	}

	// Get all packages for this language from the specific directory
	packages, err := languageDetector.GetAllPackages(ctx, dirPath)
	if err != nil {
		return fmt.Errorf("failed to get packages for %s in %s: %w", language, dirPath, err), nil, nil
	}

	return nil, libs, packages
}

// populateInstrumentations checks for available instrumentations
func (ca *CodebaseAnalyzer) populateInstrumentations(ctx context.Context, analysis *Analysis) error {
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
func (ca *CodebaseAnalyzer) runIssueDetectors(ctx context.Context, analysis *Analysis) ([]types.Issue, error) {
	var allIssues []types.Issue
	for _, detector := range ca.detectors {
		// Check if detector applies to detected languages
		if ca.detectorApplies(detector, analysis.DetectedLanguages) {
			issues, err := detector.Detect(ctx, analysis)
			if err != nil {
				return nil, err
			}
			allIssues = append(allIssues, issues...)
		}
	}

	return allIssues, nil
}

// detectorApplies checks if a detector should run for the detected languages
func (ca *CodebaseAnalyzer) detectorApplies(detector IssueDetector, detectedLanguages []string) bool {
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

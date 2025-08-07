package issues

import (
	"context"
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/detector/types"
)

// MissingInstrumentationDetector detects packages that have available instrumentations but are not instrumented
type MissingInstrumentationDetector struct{}

// NewMissingInstrumentationDetector creates a new missing instrumentation detector
func NewMissingInstrumentationDetector() *MissingInstrumentationDetector {
	return &MissingInstrumentationDetector{}
}

// ID returns the detector identifier
func (m *MissingInstrumentationDetector) ID() string {
	return "missing_instrumentation"
}

// Name returns the detector name
func (m *MissingInstrumentationDetector) Name() string {
	return "Missing OpenTelemetry Instrumentation"
}

// Description returns what this detector looks for
func (m *MissingInstrumentationDetector) Description() string {
	return "Detects packages that have OpenTelemetry instrumentation available but are not currently instrumented"
}

// Category returns the issue category
func (m *MissingInstrumentationDetector) Category() types.Category {
	return types.CategoryInstrumentation
}

// Languages returns which languages this detector applies to
func (m *MissingInstrumentationDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds packages with available instrumentations
func (m *MissingInstrumentationDetector) Detect(ctx context.Context, directory *detector.DirectoryAnalysis) ([]types.Issue, error) {
	var issues []types.Issue

	// Check if any packages have available instrumentations
	for _, instrumentation := range directory.AvailableInstrumentations {
		// Check if this package is already instrumented
		isInstrumented := m.isPackageInstrumented(instrumentation.Package, directory.Libraries)

		if !isInstrumented {
			// Create an issue for missing instrumentation
			issue := types.Issue{
				ID:          fmt.Sprintf("missing_instrumentation_%s_%s", instrumentation.Language, instrumentation.Package.Name),
				Title:       fmt.Sprintf("OpenTelemetry instrumentation available for %s", instrumentation.Package.Name),
				Description: m.buildDescription(instrumentation),
				Severity:    types.SeverityInfo,
				Category:    types.CategoryInstrumentation,
				Language:    instrumentation.Language,
				Suggestion:  m.buildSuggestion(instrumentation),
				References:  m.buildReferences(instrumentation),
			}

			if instrumentation.Package.PackageFile != "" {
				issue.File = instrumentation.Package.PackageFile
			}

			issues = append(issues, issue)
		}
	}

	return issues, nil
}

// isPackageInstrumented checks if a package is already instrumented with OpenTelemetry
func (m *MissingInstrumentationDetector) isPackageInstrumented(pkg types.Package, libraries []types.Library) bool {
	packageName := strings.ToLower(pkg.Name)

	// Common patterns for instrumentation library names
	instrumentationPatterns := []string{
		fmt.Sprintf("opentelemetry-instrumentation-%s", packageName),
		fmt.Sprintf("opentelemetry-%s", packageName),
		fmt.Sprintf("go.opentelemetry.io/contrib/instrumentation/%s", packageName),
		fmt.Sprintf("go.opentelemetry.io/contrib/instrumentation/github.com/%s", packageName),
	}

	// Check if any OTel library matches instrumentation patterns
	for _, lib := range libraries {
		libName := strings.ToLower(lib.Name)

		for _, pattern := range instrumentationPatterns {
			if strings.Contains(libName, strings.ToLower(pattern)) ||
				strings.Contains(libName, strings.ReplaceAll(packageName, "-", "")) ||
				strings.Contains(libName, strings.ReplaceAll(packageName, "_", "")) {
				return true
			}
		}
	}

	return false
}

// buildDescription creates a detailed description for the issue
func (m *MissingInstrumentationDetector) buildDescription(instrumentation types.InstrumentationInfo) string {
	description := fmt.Sprintf("The package '%s' is used in your project and has OpenTelemetry instrumentation available.",
		instrumentation.Package.Name)

	if instrumentation.Description != "" {
		description += fmt.Sprintf("\n\nInstrumentation description: %s", instrumentation.Description)
	}

	if len(instrumentation.Tags) > 0 {
		description += fmt.Sprintf("\n\nTags: %s", strings.Join(instrumentation.Tags, ", "))
	}

	if instrumentation.IsFirstParty {
		description += "\n\nThis is a first-party OpenTelemetry instrumentation (officially maintained)."
	}

	return description
}

// buildSuggestion creates installation/usage suggestions
func (m *MissingInstrumentationDetector) buildSuggestion(instrumentation types.InstrumentationInfo) string {
	packageName := instrumentation.Package.Name
	language := instrumentation.Language

	switch language {
	case "python":
		instrumentationName := fmt.Sprintf("opentelemetry-instrumentation-%s",
			strings.ReplaceAll(strings.ToLower(packageName), "_", "-"))
		return fmt.Sprintf("Install the instrumentation: pip install %s\nThen enable auto-instrumentation or manually instrument your code.",
			instrumentationName)

	case "go":
		// For Go, the suggestion depends on the package structure
		suggestion := fmt.Sprintf("Add OpenTelemetry instrumentation for %s to your go.mod file.", packageName)
		if instrumentation.URLs.Repo != "" {
			suggestion += fmt.Sprintf("\nSee the repository for usage instructions: %s", instrumentation.URLs.Repo)
		}
		return suggestion

	case "javascript", "typescript":
		instrumentationName := fmt.Sprintf("@opentelemetry/instrumentation-%s",
			strings.ReplaceAll(strings.ToLower(packageName), "_", "-"))
		return fmt.Sprintf("Install the instrumentation: npm install %s\nThen register it with your OpenTelemetry setup.",
			instrumentationName)

	default:
		return fmt.Sprintf("Add OpenTelemetry instrumentation for %s to enable automatic tracing.", packageName)
	}
}

// buildReferences creates reference links
func (m *MissingInstrumentationDetector) buildReferences(instrumentation types.InstrumentationInfo) []string {
	var references []string

	if instrumentation.URLs.Repo != "" {
		references = append(references, instrumentation.URLs.Repo)
	}

	if instrumentation.RegistryURL != "" {
		references = append(references, instrumentation.RegistryURL)
	}

	// Add general OpenTelemetry documentation links
	references = append(references, "https://opentelemetry.io/docs/instrumentation/")

	return references
}

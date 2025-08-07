package issues

import (
	"context"
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
)

// MissingOTelDetector detects when no OpenTelemetry libraries are found
type MissingOTelDetector struct{}

// NewMissingOTelDetector creates a new missing OTel detector
func NewMissingOTelDetector() *MissingOTelDetector {
	return &MissingOTelDetector{}
}

// ID returns the detector identifier
func (m *MissingOTelDetector) ID() string {
	return "missing_otel_libraries"
}

// Name returns the detector name
func (m *MissingOTelDetector) Name() string {
	return "Missing OpenTelemetry Libraries"
}

// Description returns what this detector looks for
func (m *MissingOTelDetector) Description() string {
	return "Detects when no OpenTelemetry libraries are found in the codebase"
}

// Category returns the issue category
func (m *MissingOTelDetector) Category() domain.Category {
	return domain.CategoryMissingOtel
}

// Languages returns applicable languages (empty = all languages)
func (m *MissingOTelDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds missing OTel library issues
func (m *MissingOTelDetector) Detect(ctx context.Context, directory *detector.DirectoryAnalysis) ([]domain.Issue, error) {
	var issues []domain.Issue

	// Case 1: No OpenTelemetry libraries found at all
	if len(directory.Libraries) == 0 && len(directory.Language) > 0 {
		langList := fmt.Sprintf("Detected languages: %v", directory.Language)

		issues = append(issues, domain.Issue{
			ID:          m.ID(),
			Title:       "No OpenTelemetry libraries detected",
			Description: fmt.Sprintf("No OpenTelemetry libraries found in this codebase. %s", langList),
			Severity:    domain.SeverityWarning,
			Category:    m.Category(),
			Suggestion:  "Consider adding OpenTelemetry instrumentation to gain observability into your application",
			Language:    directory.Language,
			References: []string{
				"https://opentelemetry.io/docs/instrumentation/",
				"https://opentelemetry.io/docs/getting-started/",
			},
		})
	}

	// Case 2: OpenTelemetry libraries exist but entry points are not instrumented
	if len(directory.Libraries) > 0 && len(directory.EntryPoints) > 0 {
		for _, entryPoint := range directory.EntryPoints {
			if entryPoint.Confidence >= 0.8 && !m.hasOTELInitialization(entryPoint) {
				issues = append(issues, domain.Issue{
					ID:          fmt.Sprintf("%s_uninstrumented_entrypoint_%s", m.ID(), entryPoint.FunctionName),
					Title:       "Entry point not instrumented with OpenTelemetry",
					Description: fmt.Sprintf("Entry point '%s' in %s does not have OpenTelemetry initialization code", entryPoint.FunctionName, entryPoint.FilePath),
					Severity:    domain.SeverityInfo,
					Category:    m.Category(),
					Suggestion:  "Add OpenTelemetry initialization code to instrument this entry point",
					Language:    directory.Language,
					File:        entryPoint.FilePath,
					Line:        int(entryPoint.LineNumber),
					References: []string{
						"https://opentelemetry.io/docs/instrumentation/",
						"https://opentelemetry.io/docs/getting-started/",
					},
				})
			}
		}
	}

	return issues, nil
}

// hasOTELInitialization checks if an entry point contains OpenTelemetry initialization code
func (m *MissingOTelDetector) hasOTELInitialization(entryPoint domain.EntryPoint) bool {
	context := entryPoint.Context
	// Look for common OTEL initialization patterns
	return strings.Contains(context, "TracerProvider") ||
		strings.Contains(context, "set_tracer_provider") ||
		strings.Contains(context, "initialize_otel") ||
		strings.Contains(context, "init_tracer") ||
		strings.Contains(context, "otel") ||
		strings.Contains(context, "trace.set_tracer_provider") ||
		strings.Contains(context, "opentelemetry")
}

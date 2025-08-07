package issues

import (
	"context"
	"fmt"

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

	return issues, nil
}

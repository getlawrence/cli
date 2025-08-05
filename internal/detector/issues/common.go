package issues

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/internal/detector"
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
func (m *MissingOTelDetector) Category() detector.Category {
	return detector.CategoryMissingLibrary
}

// Languages returns applicable languages (empty = all languages)
func (m *MissingOTelDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds missing OTel library issues
func (m *MissingOTelDetector) Detect(ctx context.Context, analysis *detector.Analysis) ([]detector.Issue, error) {
	var issues []detector.Issue

	if len(analysis.Libraries) == 0 && len(analysis.DetectedLanguages) > 0 {
		langList := fmt.Sprintf("Detected languages: %v", analysis.DetectedLanguages)

		issues = append(issues, detector.Issue{
			ID:          m.ID(),
			Title:       "No OpenTelemetry libraries detected",
			Description: fmt.Sprintf("No OpenTelemetry libraries found in this codebase. %s", langList),
			Severity:    detector.SeverityWarning,
			Category:    m.Category(),
			Suggestion:  "Consider adding OpenTelemetry instrumentation to gain observability into your application",
			References: []string{
				"https://opentelemetry.io/docs/instrumentation/",
				"https://opentelemetry.io/docs/getting-started/",
			},
		})
	}

	return issues, nil
}

// IncompleteInstrumentationDetector detects missing core instrumentation
type IncompleteInstrumentationDetector struct{}

// NewIncompleteInstrumentationDetector creates a new incomplete instrumentation detector
func NewIncompleteInstrumentationDetector() *IncompleteInstrumentationDetector {
	return &IncompleteInstrumentationDetector{}
}

// ID returns the detector identifier
func (i *IncompleteInstrumentationDetector) ID() string {
	return "incomplete_instrumentation"
}

// Name returns the detector name
func (i *IncompleteInstrumentationDetector) Name() string {
	return "Incomplete Instrumentation"
}

// Description returns what this detector looks for
func (i *IncompleteInstrumentationDetector) Description() string {
	return "Detects when only partial OpenTelemetry instrumentation is present"
}

// Category returns the issue category
func (i *IncompleteInstrumentationDetector) Category() detector.Category {
	return detector.CategoryInstrumentation
}

// Languages returns applicable languages (empty = all languages)
func (i *IncompleteInstrumentationDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds incomplete instrumentation issues
func (i *IncompleteInstrumentationDetector) Detect(ctx context.Context, analysis *detector.Analysis) ([]detector.Issue, error) {
	var issues []detector.Issue

	if len(analysis.Libraries) > 0 {
		hasTracing := false
		hasMetrics := false
		hasLogging := false

		// Check what types of instrumentation are present
		for _, lib := range analysis.Libraries {
			switch {
			case containsAny(lib.Name, []string{"trace", "tracing"}):
				hasTracing = true
			case containsAny(lib.Name, []string{"metric", "metrics"}):
				hasMetrics = true
			case containsAny(lib.Name, []string{"log", "logging"}):
				hasLogging = true
			}
		}

		// Generate issues for missing telemetry types
		if !hasTracing {
			issues = append(issues, detector.Issue{
				ID:          i.ID() + "_tracing",
				Title:       "Missing distributed tracing",
				Description: "OpenTelemetry libraries found but no tracing instrumentation detected",
				Severity:    detector.SeverityInfo,
				Category:    i.Category(),
				Suggestion:  "Add OpenTelemetry tracing to track request flows across services",
				References: []string{
					"https://opentelemetry.io/docs/concepts/signals/traces/",
				},
			})
		}

		if !hasMetrics {
			issues = append(issues, detector.Issue{
				ID:          i.ID() + "_metrics",
				Title:       "Missing metrics collection",
				Description: "OpenTelemetry libraries found but no metrics instrumentation detected",
				Severity:    detector.SeverityInfo,
				Category:    i.Category(),
				Suggestion:  "Add OpenTelemetry metrics to monitor application performance and health",
				References: []string{
					"https://opentelemetry.io/docs/concepts/signals/metrics/",
				},
			})
		}

		if !hasLogging {
			issues = append(issues, detector.Issue{
				ID:          i.ID() + "_logging",
				Title:       "Missing structured logging",
				Description: "OpenTelemetry libraries found but no logging instrumentation detected",
				Severity:    detector.SeverityInfo,
				Category:    i.Category(),
				Suggestion:  "Add OpenTelemetry logging to correlate logs with traces and metrics",
				References: []string{
					"https://opentelemetry.io/docs/concepts/signals/logs/",
				},
			})
		}
	}

	return issues, nil
}

// OutdatedLibrariesDetector detects outdated OpenTelemetry libraries
type OutdatedLibrariesDetector struct{}

// NewOutdatedLibrariesDetector creates a new outdated libraries detector
func NewOutdatedLibrariesDetector() *OutdatedLibrariesDetector {
	return &OutdatedLibrariesDetector{}
}

// ID returns the detector identifier
func (o *OutdatedLibrariesDetector) ID() string {
	return "outdated_libraries"
}

// Name returns the detector name
func (o *OutdatedLibrariesDetector) Name() string {
	return "Outdated Libraries"
}

// Description returns what this detector looks for
func (o *OutdatedLibrariesDetector) Description() string {
	return "Detects outdated OpenTelemetry library versions"
}

// Category returns the issue category
func (o *OutdatedLibrariesDetector) Category() detector.Category {
	return detector.CategoryDeprecated
}

// Languages returns applicable languages (empty = all languages)
func (o *OutdatedLibrariesDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds outdated library issues
func (o *OutdatedLibrariesDetector) Detect(ctx context.Context, analysis *detector.Analysis) ([]detector.Issue, error) {
	var issues []detector.Issue

	// Simple version check - in a real implementation, you'd check against latest versions
	for _, lib := range analysis.Libraries {
		if lib.Version != "" && isOutdatedVersion(lib.Version) {
			issues = append(issues, detector.Issue{
				ID:          o.ID() + "_" + lib.Name,
				Title:       fmt.Sprintf("Outdated %s library", lib.Name),
				Description: fmt.Sprintf("Library %s version %s may be outdated", lib.Name, lib.Version),
				Severity:    detector.SeverityWarning,
				Category:    o.Category(),
				Language:    lib.Language,
				File:        lib.PackageFile,
				Suggestion:  fmt.Sprintf("Consider updating %s to the latest stable version", lib.Name),
				References: []string{
					"https://opentelemetry.io/docs/migration/",
				},
			})
		}
	}

	return issues, nil
}

// Helper functions

// containsAny checks if a string contains any of the given substrings
func containsAny(s string, substrings []string) bool {
	for _, substr := range substrings {
		if len(s) >= len(substr) {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
		}
	}
	return false
}

// isOutdatedVersion checks if a version appears to be outdated (simplified)
func isOutdatedVersion(version string) bool {
	// Simple heuristic - versions starting with 0.x might be pre-1.0
	return len(version) > 0 && version[0] == '0'
}

package detector

import (
	"context"
	"fmt"
	"strings"
)

// CodeGenDetector finds opportunities for AI-assisted code generation
type CodeGenDetector struct{}

// NewCodeGenDetector creates a new code generation detector
func NewCodeGenDetector() *CodeGenDetector {
	return &CodeGenDetector{}
}

func (d *CodeGenDetector) ID() string {
	return "codegen-opportunities"
}

func (d *CodeGenDetector) Name() string {
	return "Code Generation Opportunities"
}

func (d *CodeGenDetector) Description() string {
	return "Detects opportunities for AI-assisted OTEL instrumentation"
}

func (d *CodeGenDetector) Category() Category {
	return CategoryInstrumentation
}

func (d *CodeGenDetector) Languages() []string {
	return []string{} // Applies to all languages
}

func (d *CodeGenDetector) Detect(ctx context.Context, analysis *Analysis) ([]Issue, error) {
	var issues []Issue

	// Look for frameworks that have available instrumentations but aren't instrumented
	for _, instr := range analysis.AvailableInstrumentations {
		if instr.IsAvailable && !d.isAlreadyInstrumented(analysis, instr) {
			issue := Issue{
				ID:          "codegen-" + instr.Package.Name,
				Title:       fmt.Sprintf("AI-assisted instrumentation available for %s", instr.Package.Name),
				Description: fmt.Sprintf("OpenTelemetry instrumentation is available for %s. Use AI code generation to add it.", instr.Package.Name),
				Severity:    SeverityInfo,
				Category:    CategoryInstrumentation,
				Language:    instr.Language,
				Suggestion:  fmt.Sprintf("Run: lawrence codegen --language %s --agent github", instr.Language),
			}
			issues = append(issues, issue)
		}
	}

	// Look for common patterns that suggest instrumentation opportunities
	issues = append(issues, d.detectCommonPatterns(analysis)...)

	return issues, nil
}

func (d *CodeGenDetector) isAlreadyInstrumented(analysis *Analysis, instr InstrumentationInfo) bool {
	// Check if the instrumentation library is already in use
	for _, lib := range analysis.Libraries {
		if strings.Contains(lib.Name, instr.Package.Name) ||
			strings.Contains(lib.ImportPath, instr.Package.Name) {
			return true
		}
	}
	return false
}

func (d *CodeGenDetector) detectCommonPatterns(analysis *Analysis) []Issue {
	var issues []Issue

	// Check for common web frameworks without instrumentation
	for language, files := range analysis.FilesByLanguage {
		switch language {
		case "go":
			issues = append(issues, d.detectGoPatterns(files, analysis)...)
		case "python":
			issues = append(issues, d.detectPythonPatterns(files, analysis)...)
		case "javascript":
			issues = append(issues, d.detectJSPatterns(files, analysis)...)
		}
	}

	return issues
}

func (d *CodeGenDetector) detectGoPatterns(files []string, analysis *Analysis) []Issue {
	var issues []Issue

	// Check if HTTP server is used but not instrumented
	if d.hasPattern(files, "net/http") && !d.hasOTelLibrary(analysis, "otelhttp") {
		issues = append(issues, Issue{
			ID:          "codegen-go-http",
			Title:       "HTTP server detected without OpenTelemetry instrumentation",
			Description: "Your Go application uses net/http but doesn't have OpenTelemetry HTTP instrumentation",
			Severity:    SeverityInfo,
			Category:    CategoryInstrumentation,
			Language:    "go",
			Suggestion:  "Run: lawrence codegen --language go --agent github",
		})
	}

	return issues
}

func (d *CodeGenDetector) detectPythonPatterns(files []string, analysis *Analysis) []Issue {
	var issues []Issue

	// Check for Flask without instrumentation
	if d.hasPattern(files, "from flask import") && !d.hasOTelLibrary(analysis, "flask") {
		issues = append(issues, Issue{
			ID:          "codegen-python-flask",
			Title:       "Flask application detected without OpenTelemetry instrumentation",
			Description: "Your Python application uses Flask but doesn't have OpenTelemetry Flask instrumentation",
			Severity:    SeverityInfo,
			Category:    CategoryInstrumentation,
			Language:    "python",
			Suggestion:  "Run: lawrence codegen --language python --agent github",
		})
	}

	return issues
}

func (d *CodeGenDetector) detectJSPatterns(files []string, analysis *Analysis) []Issue {
	var issues []Issue

	// Check for Express without instrumentation
	if d.hasPattern(files, "express") && !d.hasOTelLibrary(analysis, "express") {
		issues = append(issues, Issue{
			ID:          "codegen-js-express",
			Title:       "Express application detected without OpenTelemetry instrumentation",
			Description: "Your JavaScript application uses Express but doesn't have OpenTelemetry Express instrumentation",
			Severity:    SeverityInfo,
			Category:    CategoryInstrumentation,
			Language:    "javascript",
			Suggestion:  "Run: lawrence codegen --language javascript --agent github",
		})
	}

	return issues
}

func (d *CodeGenDetector) hasPattern(files []string, pattern string) bool {
	// Simple pattern check - in a real implementation, you'd parse the files
	// This is a placeholder for now
	return len(files) > 0 // Simplified for demo
}

func (d *CodeGenDetector) hasOTelLibrary(analysis *Analysis, libraryName string) bool {
	for _, lib := range analysis.Libraries {
		if strings.Contains(strings.ToLower(lib.Name), strings.ToLower(libraryName)) ||
			strings.Contains(strings.ToLower(lib.ImportPath), strings.ToLower(libraryName)) {
			return true
		}
	}
	return false
}

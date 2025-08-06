package detector

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAnalyzeCodebaseWithEnhancedLanguageDetection(t *testing.T) {
	// Create a temporary test directory structure with mixed languages
	tempDir, err := os.MkdirTemp("", "analyze_codebase_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files in different subdirectories with different languages
	testFiles := map[string]string{
		"go.mod":                  "module test\n\ngo 1.19\n",
		"main.go":                 "package main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello, World!\")\n}\n",
		"server/handler.go":       "package server\n\nfunc Handler() {}\n",
		"client/app.py":           "import requests\n\ndef get_data():\n    pass\n",
		"client/requirements.txt": "requests==2.28.0\nopentelemetry-api==1.12.0\n",
		"frontend/app.js":         "console.log('hello');\n",
		"utils/helper.go":         "package utils\n\nfunc Helper() {}\n",
	}

	for filePath, content := range testFiles {
		fullPath := filepath.Join(tempDir, filePath)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatalf("Failed to create directory for %s: %v", filePath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", filePath, err)
		}
	}

	// Create a manager and register language detectors
	manager := NewManager()

	// Run the analysis
	ctx := context.Background()
	analysis, issues, err := manager.AnalyzeCodebase(ctx, tempDir)
	if err != nil {
		t.Fatalf("AnalyzeCodebase failed: %v", err)
	}

	// Verify the analysis results
	if analysis == nil {
		t.Fatal("Analysis is nil")
	}

	if analysis.RootPath != tempDir {
		t.Errorf("Expected RootPath to be %s, got %s", tempDir, analysis.RootPath)
	}

	// Check that languages were detected
	if len(analysis.DetectedLanguages) == 0 {
		t.Error("No languages were detected")
	}

	// Verify that both Go and Python are detected (they should be in different directories)
	hasGo := false
	hasPython := false
	for _, lang := range analysis.DetectedLanguages {
		if lang == "go" || lang == "Go" {
			hasGo = true
		}
		if lang == "python" || lang == "Python" {
			hasPython = true
		}
	}

	if !hasGo {
		t.Error("Go language was not detected")
	}
	if !hasPython {
		t.Error("Python language was not detected")
	}

	// Check that files are properly categorized by language
	if analysis.FilesByLanguage == nil {
		t.Fatal("FilesByLanguage is nil")
	}

	// Verify that we have files for the detected languages
	for _, lang := range analysis.DetectedLanguages {
		if files, exists := analysis.FilesByLanguage[lang]; !exists || len(files) == 0 {
			t.Errorf("No files found for language %s", lang)
		}
	}

	// Issues can be empty or non-empty, just verify it's not nil
	if issues == nil {
		t.Error("Issues slice is nil")
	}

	t.Logf("Analysis completed successfully:")
	t.Logf("  Detected languages: %v", analysis.DetectedLanguages)
	t.Logf("  Number of libraries: %d", len(analysis.Libraries))
	t.Logf("  Number of packages: %d", len(analysis.Packages))
	t.Logf("  Number of issues: %d", len(issues))
	for lang, files := range analysis.FilesByLanguage {
		t.Logf("  Files for %s: %d", lang, len(files))
	}
}

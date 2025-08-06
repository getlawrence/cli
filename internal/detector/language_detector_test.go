package detector

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectLanguages(t *testing.T) {
	// Create a temporary test directory structure
	tempDir, err := os.MkdirTemp("", "language_detector_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files in different subdirectories
	testFiles := map[string]string{
		"main.go":            "package main\n\nfunc main() {}\n",
		"server/handler.go":  "package server\n\nfunc Handler() {}\n",
		"client/client.py":   "import requests\n\ndef get_data():\n    pass\n",
		"client/utils.py":    "def helper():\n    pass\n",
		"frontend/app.js":    "console.log('hello');\n",
		"config/config.yaml": "database:\n  host: localhost\n",
		"docs/README.md":     "# Project Documentation\n",
		"scripts/deploy.sh":  "#!/bin/bash\necho 'deploying'\n",
		"data/sample.json":   "{\"key\": \"value\"}\n",
		".env":               "API_KEY=secret\n",
		"Makefile":           "build:\n\tgo build\n",
		"Dockerfile":         "FROM golang:1.19\n",
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

	// Test DetectLanguages
	languages, err := DetectLanguages(tempDir)
	if err != nil {
		t.Fatalf("DetectLanguages failed: %v", err)
	}

	// Verify expected languages are detected
	expectedLanguages := map[string]string{
		"root":     "Go",         // main.go in root
		"server":   "Go",         // handler.go
		"client":   "Python",     // client.py, utils.py
		"frontend": "JavaScript", // app.js
		"scripts":  "Shell",      // deploy.sh
	}

	for dir, expectedLang := range expectedLanguages {
		if detectedLang, exists := languages[dir]; !exists {
			t.Errorf("Expected language %s for directory %s, but directory not found", expectedLang, dir)
		} else if detectedLang != expectedLang {
			t.Errorf("Expected language %s for directory %s, got %s", expectedLang, dir, detectedLang)
		}
	}

	// Verify config/docs directories are not included (they only have non-programming files)
	nonProgrammingDirs := []string{"config", "docs", "data"}
	for _, dir := range nonProgrammingDirs {
		if _, exists := languages[dir]; exists {
			t.Errorf("Directory %s should not be detected as it contains only non-programming files", dir)
		}
	}
}

func TestDetectLanguageForFile(t *testing.T) {
	// Create temporary test files
	tempDir, err := os.MkdirTemp("", "file_detector_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		filename     string
		content      string
		expectedLang string
		description  string
	}{
		{
			filename:     "main.go",
			content:      "package main\n\nfunc main() {}\n",
			expectedLang: "Go",
			description:  "Go file with .go extension",
		},
		{
			filename:     "app.py",
			content:      "import os\n\ndef main():\n    pass\n",
			expectedLang: "Python",
			description:  "Python file with .py extension",
		},
		{
			filename:     "script.js",
			content:      "console.log('hello world');\n",
			expectedLang: "JavaScript",
			description:  "JavaScript file with .js extension",
		},
		{
			filename:     "style.css",
			content:      "body { margin: 0; }\n",
			expectedLang: "CSS",
			description:  "CSS file",
		},
		{
			filename:     "config.json",
			content:      "{\"key\": \"value\"}\n",
			expectedLang: "JSON",
			description:  "JSON file",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			filePath := filepath.Join(tempDir, tc.filename)
			if err := os.WriteFile(filePath, []byte(tc.content), 0644); err != nil {
				t.Fatalf("Failed to create test file %s: %v", tc.filename, err)
			}

			detectedLang, err := DetectLanguageForFile(filePath)
			if err != nil {
				t.Errorf("DetectLanguageForFile failed for %s: %v", tc.filename, err)
			}

			if detectedLang != tc.expectedLang {
				t.Errorf("Expected language %s for file %s, got %s", tc.expectedLang, tc.filename, detectedLang)
			}
		})
	}
}

func TestShouldSkipFile(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "skip_file_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	testCases := []struct {
		path        string
		isDir       bool
		shouldSkip  bool
		description string
	}{
		{
			path:        filepath.Join(tempDir, "main.go"),
			isDir:       false,
			shouldSkip:  false,
			description: "Regular source file should not be skipped",
		},
		{
			path:        filepath.Join(tempDir, ".hidden"),
			isDir:       false,
			shouldSkip:  true,
			description: "Hidden file should be skipped",
		},
		{
			path:        filepath.Join(tempDir, "node_modules", "package", "index.js"),
			isDir:       false,
			shouldSkip:  true,
			description: "File in node_modules should be skipped",
		},
		{
			path:        filepath.Join(tempDir, "vendor", "github.com", "lib", "file.go"),
			isDir:       false,
			shouldSkip:  true,
			description: "File in vendor should be skipped",
		},
		{
			path:        filepath.Join(tempDir, "__pycache__", "module.pyc"),
			isDir:       false,
			shouldSkip:  true,
			description: "File in __pycache__ should be skipped",
		},
		{
			path:        filepath.Join(tempDir, "src"),
			isDir:       true,
			shouldSkip:  false,
			description: "Regular directory should not be skipped",
		},
		{
			path:        filepath.Join(tempDir, ".git"),
			isDir:       true,
			shouldSkip:  false, // shouldSkipFile returns false for directories, they get handled by filepath.SkipDir
			description: "Git directory detection",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			// Create the file/directory for testing
			if tc.isDir {
				if err := os.MkdirAll(tc.path, 0755); err != nil {
					t.Fatalf("Failed to create test directory %s: %v", tc.path, err)
				}
			} else {
				if err := os.MkdirAll(filepath.Dir(tc.path), 0755); err != nil {
					t.Fatalf("Failed to create parent directory for %s: %v", tc.path, err)
				}
				if err := os.WriteFile(tc.path, []byte("test content"), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", tc.path, err)
				}
			}

			info, err := os.Stat(tc.path)
			if err != nil {
				t.Fatalf("Failed to stat test path %s: %v", tc.path, err)
			}

			shouldSkip := shouldSkipFile(tempDir, tc.path, info)
			if shouldSkip != tc.shouldSkip {
				t.Errorf("Expected shouldSkip=%v for %s, got %v", tc.shouldSkip, tc.path, shouldSkip)
			}
		})
	}
}

func TestIsProgrammingLanguage(t *testing.T) {
	testCases := []struct {
		language      string
		isProgramming bool
		description   string
	}{
		// Programming languages
		{"Go", true, "Go should be considered programming language"},
		{"Python", true, "Python should be considered programming language"},
		{"JavaScript", true, "JavaScript should be considered programming language"},
		{"TypeScript", true, "TypeScript should be considered programming language"},
		{"Java", true, "Java should be considered programming language"},
		{"C++", true, "C++ should be considered programming language"},
		{"Rust", true, "Rust should be considered programming language"},
		{"Ruby", true, "Ruby should be considered programming language"},

		// Configuration and markup languages
		{"YAML", false, "YAML should not be considered programming language"},
		{"JSON", false, "JSON should not be considered programming language"},
		{"TOML", false, "TOML should not be considered programming language"},
		{"XML", false, "XML should not be considered programming language"},
		{"Markdown", false, "Markdown should not be considered programming language"},
		{"HTML", false, "HTML should not be considered programming language"},
		{"CSS", false, "CSS should not be considered programming language"},
		{"SCSS", false, "SCSS should not be considered programming language"},
		{"Dockerfile", false, "Dockerfile should not be considered programming language"},
		{"Makefile", false, "Makefile should not be considered programming language"},
		{"Text", false, "Text should not be considered programming language"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := isProgrammingLanguage(tc.language)
			if result != tc.isProgramming {
				t.Errorf("Expected isProgrammingLanguage(%s) = %v, got %v", tc.language, tc.isProgramming, result)
			}
		})
	}
}

func TestNormalizeDirectoryKey(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"", "root"},
		{"src", "src"},
		{"src/main", "src/main"},
		{".", "."},
	}

	for _, tc := range testCases {
		t.Run("normalize_"+tc.input, func(t *testing.T) {
			result := normalizeDirectoryKey(tc.input)
			if result != tc.expected {
				t.Errorf("Expected normalizeDirectoryKey(%s) = %s, got %s", tc.input, tc.expected, result)
			}
		})
	}
}

func TestFindMostCommonLanguage(t *testing.T) {
	testCases := []struct {
		langCounts  map[string]int
		expected    string
		description string
	}{
		{
			langCounts:  map[string]int{"Go": 5, "Python": 3},
			expected:    "Go",
			description: "Go should be primary with higher count",
		},
		{
			langCounts:  map[string]int{"Python": 3, "Go": 5},
			expected:    "Go",
			description: "Go should be primary regardless of order",
		},
		{
			langCounts:  map[string]int{"JavaScript": 1},
			expected:    "JavaScript",
			description: "Single language should be primary",
		},
		{
			langCounts:  map[string]int{},
			expected:    "",
			description: "Empty map should return empty string",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := findMostCommonLanguage(tc.langCounts)
			if result != tc.expected {
				t.Errorf("Expected %s, got %s", tc.expected, result)
			}
		})
	}
}

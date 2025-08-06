package languages

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestPythonDetectorName(t *testing.T) {
	d := NewPythonDetector()
	expected := "python"
	if got := d.Name(); got != expected {
		t.Errorf("PythonDetector.Name() = %v, want %v", got, expected)
	}
}

func TestPythonDetectorDetect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "python_detector_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewPythonDetector()
	ctx := context.Background()

	testCases := []struct {
		name        string
		files       map[string]string
		expected    bool
		description string
	}{
		{
			name: "requirements_txt",
			files: map[string]string{
				"requirements.txt": "requests==2.28.0\nflask==2.0.0\n",
			},
			expected:    true,
			description: "Should detect Python project with requirements.txt",
		},
		{
			name: "pyproject_toml",
			files: map[string]string{
				"pyproject.toml": "[tool.poetry]\nname = \"myapp\"\n",
			},
			expected:    true,
			description: "Should detect Python project with pyproject.toml",
		},
		{
			name: "no_python_files",
			files: map[string]string{
				"main.go":   "package main\n",
				"README.md": "# Project\n",
			},
			expected:    false,
			description: "Should not detect Python project without Python files",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Create test directory for this case
			testDir := filepath.Join(tempDir, tc.name)
			if err := os.MkdirAll(testDir, 0755); err != nil {
				t.Fatalf("Failed to create test dir: %v", err)
			}

			// Create test files
			for filename, content := range tc.files {
				filePath := filepath.Join(testDir, filename)
				if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
					t.Fatalf("Failed to create test file %s: %v", filename, err)
				}
			}

			// Test detection
			result, err := detector.Detect(ctx, testDir)
			if err != nil {
				t.Errorf("Detect() error = %v", err)
				return
			}

			if result != tc.expected {
				t.Errorf("Detect() = %v, want %v for %s", result, tc.expected, tc.description)
			}
		})
	}
}

func TestPythonDetectorGetFilePatterns(t *testing.T) {
	detector := NewPythonDetector()
	patterns := detector.GetFilePatterns()

	expectedPatterns := []string{"**/*.py", "requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}

	if !reflect.DeepEqual(patterns, expectedPatterns) {
		t.Errorf("GetFilePatterns() = %v, want %v", patterns, expectedPatterns)
	}
}

func TestPythonDetectorIsThirdPartyPackage(t *testing.T) {
	detector := NewPythonDetector()

	testCases := []struct {
		packageName  string
		isThirdParty bool
		description  string
	}{
		// Standard library packages
		{"os", false, "os is standard library"},
		{"sys", false, "sys is standard library"},
		{"json", false, "json is standard library"},
		{"datetime", false, "datetime is standard library"},

		// Third-party packages
		{"requests", true, "requests is third-party"},
		{"flask", true, "flask is third-party"},
		{"numpy", true, "numpy is third-party"},
		{"opentelemetry", true, "opentelemetry is third-party"},

		// Relative imports
		{".local_module", false, "relative imports should not be considered third-party"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := detector.isThirdPartyPythonPackage(tc.packageName)
			if result != tc.isThirdParty {
				t.Errorf("isThirdPartyPythonPackage(%s) = %v, want %v", tc.packageName, result, tc.isThirdParty)
			}
		})
	}
}

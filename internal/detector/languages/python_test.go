package languages

import (
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

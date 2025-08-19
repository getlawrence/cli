package registry

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/getlawrence/cli/internal/logger"
)

func TestNewLocalClient(t *testing.T) {
	client := NewClient("/test/path", &logger.StdoutLogger{})

	if client.registryPath != "/test/path" {
		t.Errorf("Expected registry path to be /test/path, got %s", client.registryPath)
	}

	if client.cache == nil {
		t.Error("Expected cache to be initialized")
	}
}

func TestGetSupportedLanguages(t *testing.T) {
	client := NewClient("/test/path", &logger.StdoutLogger{})
	languages := client.GetSupportedLanguages()

	expectedLanguages := []string{"javascript", "go", "python", "java", "csharp", "php", "ruby"}

	if len(languages) != len(expectedLanguages) {
		t.Errorf("Expected %d languages, got %d", len(expectedLanguages), len(languages))
	}

	for _, expected := range expectedLanguages {
		found := false
		for _, actual := range languages {
			if actual == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected language %s not found", expected)
		}
	}
}

func TestGetComponentsByLanguage_RegistryNotExists(t *testing.T) {
	client := NewClient("/nonexistent/path", &logger.StdoutLogger{})

	_, err := client.GetComponentsByLanguage("go")
	if err == nil {
		t.Error("Expected error when registry doesn't exist")
	}

	if err.Error() != "registry path does not exist: /nonexistent/path/data/registry. Please run 'lawrence registry sync' first" {
		t.Errorf("Unexpected error message: %s", err.Error())
	}
}

func TestParseComponentFromFile(t *testing.T) {
	// Create a temporary test file
	tempDir := t.TempDir()
	testFile := filepath.Join(tempDir, "test.yml")

	testYAML := `title: Test Component
registryType: Instrumentation
language: go
tags: [test, example]
license: MIT
description: A test component for testing
authors:
  - name: Test Author
urls:
  repo: https://github.com/test/test
createdAt: "2024-01-01"
isFirstParty: false
package:
  registry: npm
  name: test-component
  version: 1.0.0`

	if err := os.WriteFile(testFile, []byte(testYAML), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	client := NewClient(tempDir, &logger.StdoutLogger{})

	component, err := client.parseComponentFromFile(testFile)
	if err != nil {
		t.Fatalf("Failed to parse component: %v", err)
	}

	if component.Name != "test-component" {
		t.Errorf("Expected name 'test-component', got '%s'", component.Name)
	}

	if component.Type != "Instrumentation" {
		t.Errorf("Expected type 'Instrumentation', got '%s'", component.Type)
	}

	if component.Language != "go" {
		t.Errorf("Expected language 'go', got '%s'", component.Language)
	}

	if component.Description != "A test component for testing" {
		t.Errorf("Expected description 'A test component for testing', got '%s'", component.Description)
	}

	if component.Repository != "https://github.com/test/test" {
		t.Errorf("Expected repository 'https://github.com/test/test', got '%s'", component.Repository)
	}

	if component.License != "MIT" {
		t.Errorf("Expected license 'MIT', got '%s'", component.License)
	}

	if len(component.Tags) != 2 || component.Tags[0] != "test" || component.Tags[1] != "example" {
		t.Errorf("Expected tags [test example], got %v", component.Tags)
	}
}

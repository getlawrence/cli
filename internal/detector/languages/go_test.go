package languages

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestGoDetectorName(t *testing.T) {
	d := NewGoDetector()
	expected := "go"
	if got := d.Name(); got != expected {
		t.Errorf("GoDetector.Name() = %v, want %v", got, expected)
	}
}

func TestGoDetectorGetOTelLibraries(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go_otel_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewGoDetector()
	ctx := context.Background()

	// Create test files with OTel dependencies
	testFiles := map[string]string{
		"go.mod": `module github.com/example/app

go 1.19

require (
	go.opentelemetry.io/otel v1.15.0
	go.opentelemetry.io/otel/sdk v1.15.0
	go.opentelemetry.io/otel/exporters/jaeger v1.15.0
	go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp v0.40.0
	github.com/gin-gonic/gin v1.9.0
)`,

		"main.go": `package main

import (
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/sdk/trace"
	"github.com/gin-gonic/gin"
)

func main() {}`,

		"service.go": `package main

import (
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/exporters/jaeger"
)`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test GetOTelLibraries
	libraries, err := detector.GetOTelLibraries(ctx, tempDir)
	if err != nil {
		t.Fatalf("GetOTelLibraries() error = %v", err)
	}

	// Expected libraries (deduplicated)
	expectedLibs := map[string]bool{
		"go.opentelemetry.io/otel":                                      true,
		"go.opentelemetry.io/otel/sdk":                                  true,
		"go.opentelemetry.io/otel/exporters/jaeger":                     true,
		"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp": true,
		"go.opentelemetry.io/otel/trace":                                true,
		"go.opentelemetry.io/otel/sdk/trace":                            true,
	}

	if len(libraries) == 0 {
		t.Error("Expected to find OTel libraries, but got none")
	}

	// Check that we found the expected libraries
	foundLibs := make(map[string]bool)
	for _, lib := range libraries {
		foundLibs[lib.Name] = true
	}

	for expectedName := range expectedLibs {
		if !foundLibs[expectedName] {
			t.Errorf("Expected to find library %s, but it was not detected", expectedName)
		}
	}

	// Verify all libraries are marked as Go
	for _, lib := range libraries {
		if lib.Language != "go" {
			t.Errorf("Library %s has wrong language: %s, expected go", lib.Name, lib.Language)
		}
	}
}

func TestGoDetectorGetAllPackages(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "go_packages_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewGoDetector()
	ctx := context.Background()

	// Create test files with various dependencies
	testFiles := map[string]string{
		"go.mod": `module github.com/example/app

go 1.19

require (
	github.com/gin-gonic/gin v1.9.0
	github.com/gorilla/mux v1.8.0
	go.opentelemetry.io/otel v1.15.0
)`,

		"main.go": `package main

import (
	"fmt"
	"net/http"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/mux"
	"go.opentelemetry.io/otel"
)

func main() {}`,

		"utils.go": `package main

import (
	"context"
	"encoding/json"
	"database/sql"
	"github.com/lib/pq"
)`,
	}

	for filename, content := range testFiles {
		filePath := filepath.Join(tempDir, filename)
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", filename, err)
		}
	}

	// Test GetAllPackages
	packages, err := detector.GetAllPackages(ctx, tempDir)
	if err != nil {
		t.Fatalf("GetAllPackages() error = %v", err)
	}

	// Expected packages (from go.mod and imports)
	expectedPackages := []string{
		"github.com/gin-gonic/gin",
		"github.com/gorilla/mux",
		"go.opentelemetry.io/otel",
		"github.com/lib/pq", // from imports (third-party)
	}

	// Standard library packages that should NOT be included
	notExpectedPackages := []string{
		"fmt", "net/http", "context", "encoding/json", "database/sql", // standard library
	}

	foundPackages := make(map[string]bool)
	for _, pkg := range packages {
		foundPackages[pkg.Name] = true
		if pkg.Language != "go" {
			t.Errorf("Package %s has wrong language: %s, expected go", pkg.Name, pkg.Language)
		}
	}

	// Check expected packages are found
	for _, expectedPkg := range expectedPackages {
		if !foundPackages[expectedPkg] {
			t.Errorf("Expected to find package %s, but it was not detected", expectedPkg)
		}
	}

	// Check standard library packages are NOT found
	for _, notExpectedPkg := range notExpectedPackages {
		if foundPackages[notExpectedPkg] {
			t.Errorf("Did not expect to find standard library package %s, but it was detected", notExpectedPkg)
		}
	}
}

func TestGoDetectorGetFilePatterns(t *testing.T) {
	detector := NewGoDetector()
	patterns := detector.GetFilePatterns()

	expectedPatterns := []string{"**/*.go", "go.mod", "go.sum"}

	if !reflect.DeepEqual(patterns, expectedPatterns) {
		t.Errorf("GetFilePatterns() = %v, want %v", patterns, expectedPatterns)
	}
}

func TestGoDetectorIsThirdPartyPackage(t *testing.T) {
	detector := NewGoDetector()

	testCases := []struct {
		packageName  string
		isThirdParty bool
		description  string
	}{
		// Standard library packages
		{"fmt", false, "fmt is standard library"},
		{"net/http", false, "net/http is standard library"},
		{"encoding/json", false, "encoding/json is standard library"},
		{"context", false, "context is standard library"},
		{"database/sql", false, "database/sql is standard library"},
		{"time", false, "time is standard library"},
		{"os", false, "os is standard library"},
		{"path/filepath", false, "path/filepath is standard library"},

		// Third-party packages
		{"github.com/gin-gonic/gin", true, "gin is third-party"},
		{"github.com/gorilla/mux", true, "gorilla/mux is third-party"},
		{"go.opentelemetry.io/otel", true, "otel is third-party"},
		{"github.com/lib/pq", true, "lib/pq is third-party"},
		{"gopkg.in/yaml.v2", true, "yaml.v2 is third-party"},

		// Local packages (should be considered third-party for this test)
		{"./internal/utils", true, "local packages are not standard library"},
		{"../shared", true, "relative imports are not standard library"},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			result := detector.isThirdPartyPackage(tc.packageName)
			if result != tc.isThirdParty {
				t.Errorf("isThirdPartyPackage(%s) = %v, want %v", tc.packageName, result, tc.isThirdParty)
			}
		})
	}
}

func TestGoDetectorFindGoFiles(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "find_go_files_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	detector := NewGoDetector()

	// Create test directory structure
	testFiles := map[string]string{
		"main.go":                            "package main",
		"handler.go":                         "package main",
		"cmd/server/main.go":                 "package main",
		"internal/utils/helper.go":           "package utils",
		"pkg/models/user.go":                 "package models",
		"vendor/github.com/lib/pq/driver.go": "package pq",
		".git/hooks/pre-commit":              "#!/bin/bash",
		"README.md":                          "# Documentation",
		"go.mod":                             "module test",
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

	// Test findGoFiles
	goFiles, err := detector.findGoFiles(tempDir)
	if err != nil {
		t.Fatalf("findGoFiles() error = %v", err)
	}

	// Expected Go files (excluding those in skipped directories)
	expectedFiles := []string{
		filepath.Join(tempDir, "main.go"),
		filepath.Join(tempDir, "handler.go"),
		filepath.Join(tempDir, "cmd/server/main.go"),
		filepath.Join(tempDir, "internal/utils/helper.go"),
		filepath.Join(tempDir, "pkg/models/user.go"),
	}

	// Files that should NOT be included
	notExpectedFiles := []string{
		filepath.Join(tempDir, "vendor/github.com/lib/pq/driver.go"),
		filepath.Join(tempDir, ".git/hooks/pre-commit"),
		filepath.Join(tempDir, "README.md"),
		filepath.Join(tempDir, "go.mod"),
	}

	foundFiles := make(map[string]bool)
	for _, file := range goFiles {
		foundFiles[file] = true
	}

	// Check expected files are found
	for _, expectedFile := range expectedFiles {
		if !foundFiles[expectedFile] {
			t.Errorf("Expected to find Go file %s, but it was not detected", expectedFile)
		}
	}

	// Check files in skipped directories or non-Go files are NOT found
	for _, notExpectedFile := range notExpectedFiles {
		if foundFiles[notExpectedFile] {
			t.Errorf("Did not expect to find file %s, but it was detected", notExpectedFile)
		}
	}
}

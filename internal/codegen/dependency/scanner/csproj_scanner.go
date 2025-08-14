package scanner

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// CsprojScanner scans .csproj files for .NET dependencies
type CsprojScanner struct{}

// NewCsprojScanner creates a new csproj scanner
func NewCsprojScanner() Scanner {
	return &CsprojScanner{}
}

// Detect checks for .csproj files
func (s *CsprojScanner) Detect(projectPath string) bool {
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return false
	}

	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csproj") {
			return true
		}
	}
	return false
}

// Scan reads .csproj and returns package references
func (s *CsprojScanner) Scan(projectPath string) ([]string, error) {
	// Find first .csproj file
	entries, err := os.ReadDir(projectPath)
	if err != nil {
		return nil, err
	}

	var csprojPath string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".csproj") {
			csprojPath = filepath.Join(projectPath, entry.Name())
			break
		}
	}

	if csprojPath == "" {
		return nil, nil
	}

	content, err := os.ReadFile(csprojPath)
	if err != nil {
		return nil, err
	}

	// Extract PackageReference elements
	var deps []string
	re := regexp.MustCompile(`<PackageReference\s+Include="([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(content), -1)

	for _, match := range matches {
		if len(match) > 1 {
			deps = append(deps, match[1])
		}
	}

	return deps, nil
}

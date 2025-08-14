package scanner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// ComposerScanner scans composer.json for PHP dependencies
type ComposerScanner struct{}

// NewComposerScanner creates a new composer scanner
func NewComposerScanner() Scanner {
	return &ComposerScanner{}
}

// Detect checks for composer.json
func (s *ComposerScanner) Detect(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "composer.json"))
	return err == nil
}

// Scan reads composer.json and returns package names
func (s *ComposerScanner) Scan(projectPath string) ([]string, error) {
	file, err := os.ReadFile(filepath.Join(projectPath, "composer.json"))
	if err != nil {
		return nil, err
	}

	var composer struct {
		Require    map[string]string `json:"require"`
		RequireDev map[string]string `json:"require-dev"`
	}

	if err := json.Unmarshal(file, &composer); err != nil {
		return nil, err
	}

	var deps []string
	for pkg := range composer.Require {
		// Skip PHP version constraint
		if pkg != "php" && !strings.HasPrefix(pkg, "ext-") {
			deps = append(deps, pkg)
		}
	}
	for pkg := range composer.RequireDev {
		if pkg != "php" && !strings.HasPrefix(pkg, "ext-") {
			deps = append(deps, pkg)
		}
	}

	return deps, nil
}

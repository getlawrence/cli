package scanner

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Scanner detects and enumerates project dependencies for a given language/ecosystem
type Scanner interface {
	// Detect returns true if the project path appears to be managed by this scanner
	Detect(projectPath string) bool
	// Scan returns a list of dependency coordinates (import paths or package names)
	Scan(projectPath string) ([]string, error)
}

// GoModScanner scans go.mod files for module requirements
type GoModScanner struct{}

func NewGoModScanner() *GoModScanner { return &GoModScanner{} }

func (s *GoModScanner) Detect(projectPath string) bool {
	if _, err := os.Stat(filepath.Join(projectPath, "go.mod")); err == nil {
		return true
	}
	return false
}

func (s *GoModScanner) Scan(projectPath string) ([]string, error) {
	b, err := os.ReadFile(filepath.Join(projectPath, "go.mod"))
	if err != nil {
		return nil, err
	}
	lines := strings.Split(string(b), "\n")
	deps := make([]string, 0, 32)
	inBlock := false
	for _, ln := range lines {
		t := strings.TrimSpace(ln)
		if strings.HasPrefix(t, "require (") {
			inBlock = true
			continue
		}
		if inBlock && t == ")" {
			inBlock = false
			continue
		}
		if strings.HasPrefix(t, "require ") {
			t = strings.TrimSpace(strings.TrimPrefix(t, "require "))
		}
		if inBlock || strings.HasPrefix(ln, "require ") {
			fields := strings.Fields(t)
			if len(fields) >= 1 && !strings.HasPrefix(fields[0], "//") {
				deps = append(deps, fields[0])
			}
		}
	}
	return deps, nil
}

// NpmScanner scans package.json dependencies
type NpmScanner struct{}

func NewNpmScanner() *NpmScanner { return &NpmScanner{} }

func (s *NpmScanner) Detect(projectPath string) bool {
	if _, err := os.Stat(filepath.Join(projectPath, "package.json")); err == nil {
		return true
	}
	return false
}

func (s *NpmScanner) Scan(projectPath string) ([]string, error) {
	p := filepath.Join(projectPath, "package.json")
	raw, err := os.ReadFile(p)
	if err != nil {
		return nil, err
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, fmt.Errorf("invalid package.json: %w", err)
	}
	out := make([]string, 0, 64)
	for _, k := range []string{"dependencies", "devDependencies"} {
		if v, ok := m[k]; ok {
			if mm, ok := v.(map[string]any); ok {
				for name := range mm {
					out = append(out, name)
				}
			}
		}
	}
	return out, nil
}

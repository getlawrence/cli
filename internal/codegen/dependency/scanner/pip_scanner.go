package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// PipScanner scans requirements.txt for Python dependencies
type PipScanner struct{}

// NewPipScanner creates a new pip scanner
func NewPipScanner() Scanner {
	return &PipScanner{}
}

// Detect checks for requirements.txt
func (s *PipScanner) Detect(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "requirements.txt"))
	return err == nil
}

// Scan reads requirements.txt and returns package names
func (s *PipScanner) Scan(projectPath string) ([]string, error) {
	file, err := os.Open(filepath.Join(projectPath, "requirements.txt"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var deps []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		// Extract package name (before any version specifier)
		pkg := line
		for _, sep := range []string{"==", ">=", "<=", ">", "<", "~=", "!="} {
			if idx := strings.Index(pkg, sep); idx != -1 {
				pkg = pkg[:idx]
				break
			}
		}

		pkg = strings.TrimSpace(pkg)
		if pkg != "" {
			deps = append(deps, pkg)
		}
	}

	return deps, scanner.Err()
}

package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// GemfileScanner scans Gemfile for Ruby dependencies
type GemfileScanner struct{}

// NewGemfileScanner creates a new Gemfile scanner
func NewGemfileScanner() Scanner {
	return &GemfileScanner{}
}

// Detect checks for Gemfile
func (s *GemfileScanner) Detect(projectPath string) bool {
	_, err := os.Stat(filepath.Join(projectPath, "Gemfile"))
	return err == nil
}

// Scan reads Gemfile and returns gem names
func (s *GemfileScanner) Scan(projectPath string) ([]string, error) {
	file, err := os.Open(filepath.Join(projectPath, "Gemfile"))
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var deps []string
	gemRegex := regexp.MustCompile(`^\s*gem\s+['"]([^'"]+)['"]`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		// Skip comments
		if strings.TrimSpace(line) == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}

		// Match gem declarations
		if matches := gemRegex.FindStringSubmatch(line); len(matches) > 1 {
			deps = append(deps, matches[1])
		}
	}

	return deps, scanner.Err()
}

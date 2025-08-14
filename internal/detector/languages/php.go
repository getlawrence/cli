package languages

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/domain"
)

// PHPDetector detects PHP projects and OpenTelemetry usage
type PHPDetector struct{}

// NewPHPDetector creates a new PHP language detector
func NewPHPDetector() *PHPDetector { return &PHPDetector{} }

// Name returns the language name
func (p *PHPDetector) Name() string { return "php" }

// GetOTelLibraries finds OpenTelemetry libraries in PHP projects
func (p *PHPDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Parse composer.json for dependencies
	composerPath := filepath.Join(rootPath, "composer.json")
	if _, err := os.Stat(composerPath); err == nil {
		libs, err := p.parseComposerForOTel(composerPath)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Scan .php files for OpenTelemetry namespaces/usages as a heuristic
	phpFiles, err := p.findPHPFiles(rootPath)
	if err == nil {
		for _, f := range phpFiles {
			libs, perr := p.scanPHPFileForOTel(f)
			if perr == nil && len(libs) > 0 {
				libraries = append(libraries, libs...)
			}
		}
	}

	return p.deduplicateLibraries(libraries), nil
}

// GetFilePatterns returns patterns for PHP files
func (p *PHPDetector) GetFilePatterns() []string {
	return []string{"**/*.php", "composer.json", "composer.lock"}
}

// GetAllPackages finds all packages/dependencies used in the PHP project
func (p *PHPDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	// composer.json
	composerPath := filepath.Join(rootPath, "composer.json")
	if _, err := os.Stat(composerPath); err == nil {
		pkgs, err := p.parseComposerPackages(composerPath)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	return p.deduplicatePackages(packages), nil
}

// Helpers

func (p *PHPDetector) parseComposerForOTel(path string) ([]domain.Library, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	var libs []domain.Library
	for _, section := range []string{"require", "require-dev"} {
		deps, _ := data[section].(map[string]any)
		for name := range deps {
			n := strings.ToLower(name)
			if strings.Contains(n, "open-telemetry") || strings.Contains(n, "opentelemetry") {
				libs = append(libs, domain.Library{
					Name:        name,
					Language:    "php",
					ImportPath:  name,
					PackageFile: path,
				})
			}
		}
	}
	return libs, nil
}

func (p *PHPDetector) parseComposerPackages(path string) ([]domain.Package, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := json.Unmarshal(content, &data); err != nil {
		return nil, err
	}
	var pkgs []domain.Package
	for _, section := range []string{"require", "require-dev"} {
		deps, _ := data[section].(map[string]any)
		for name, ver := range deps {
			version := fmt.Sprintf("%v", ver)
			pkgs = append(pkgs, domain.Package{
				Name:        name,
				Version:     version,
				Language:    "php",
				ImportPath:  name,
				PackageFile: path,
			})
		}
	}
	return pkgs, nil
}

func (p *PHPDetector) findPHPFiles(root string) ([]string, error) {
	var files []string
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			switch name {
			case "vendor", ".git":
				return filepath.SkipDir
			}
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".php") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (p *PHPDetector) scanPHPFileForOTel(path string) ([]domain.Library, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	re := regexp.MustCompile(`(?i)OpenTelemetry\\|opentelemetry`) // namespace or string usage
	found := false
	for scanner.Scan() {
		if re.MatchString(scanner.Text()) {
			found = true
			break
		}
	}
	if found {
		return []domain.Library{{
			Name:       "open-telemetry/opentelemetry",
			Language:   "php",
			ImportPath: "open-telemetry/opentelemetry",
		}}, nil
	}
	return nil, nil
}

func (p *PHPDetector) deduplicateLibraries(libs []domain.Library) []domain.Library {
	seen := make(map[string]bool)
	var result []domain.Library
	for _, l := range libs {
		key := strings.ToLower(l.Name)
		if !seen[key] {
			seen[key] = true
			result = append(result, l)
		}
	}
	return result
}

func (p *PHPDetector) deduplicatePackages(pkgs []domain.Package) []domain.Package {
	seen := make(map[string]bool)
	var result []domain.Package
	for _, p := range pkgs {
		key := strings.ToLower(p.Name + ":" + p.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, p)
		}
	}
	return result
}

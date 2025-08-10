package languages

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/domain"
)

// RubyDetector detects Ruby projects and OpenTelemetry usage
type RubyDetector struct{}

// NewRubyDetector creates a new Ruby language detector
func NewRubyDetector() *RubyDetector { return &RubyDetector{} }

// Name returns the language name
func (r *RubyDetector) Name() string { return "ruby" }

// GetOTelLibraries finds OpenTelemetry libraries in Ruby projects
func (r *RubyDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Check Gemfile
	gemfile := filepath.Join(rootPath, "Gemfile")
	if _, err := os.Stat(gemfile); err == nil {
		if libs, err := r.parseGemfileForOTel(gemfile); err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Check Gemfile.lock
	lockfile := filepath.Join(rootPath, "Gemfile.lock")
	if _, err := os.Stat(lockfile); err == nil {
		if libs, err := r.parseLockfileForOTel(lockfile); err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Scan .rb files for require statements
	rbFiles, err := r.findRubyFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range rbFiles {
		if libs, err := r.parseRubyRequires(file); err == nil {
			libraries = append(libraries, libs...)
		}
	}

	return r.deduplicateLibraries(libraries), nil
}

// GetAllPackages finds all packages/dependencies used in the Ruby project
func (r *RubyDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	// Parse Gemfile.lock for all gems with versions
	lockfile := filepath.Join(rootPath, "Gemfile.lock")
	if _, err := os.Stat(lockfile); err == nil {
		if pkgs, err := r.parseAllFromLockfile(lockfile); err == nil {
			packages = append(packages, pkgs...)
		}
	}

	// Fallback: parse Gemfile for gem names (without versions)
	gemfile := filepath.Join(rootPath, "Gemfile")
	if _, err := os.Stat(gemfile); err == nil {
		if pkgs, err := r.parseAllFromGemfile(gemfile); err == nil {
			packages = append(packages, pkgs...)
		}
	}

	// Also glean from require statements in .rb files
	rbFiles, err := r.findRubyFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range rbFiles {
		if pkgs, err := r.parseAllRubyRequires(file); err == nil {
			packages = append(packages, pkgs...)
		}
	}

	return r.deduplicatePackages(packages), nil
}

// GetFilePatterns returns file patterns this detector should scan
func (r *RubyDetector) GetFilePatterns() []string {
	return []string{"**/*.rb", "Gemfile", "Gemfile.lock"}
}

// parseGemfileForOTel extracts OTel gems from Gemfile
func (r *RubyDetector) parseGemfileForOTel(path string) ([]domain.Library, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	// gem 'opentelemetry-api', '1.2.0' or gem "opentelemetry-api"
	re := regexp.MustCompile(`gem\s+["'](opentelemetry[^"']*)["'](?:\s*,\s*["']([^"']+)["'])?`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			version := ""
			if len(m) >= 3 {
				version = m[2]
			}
			libs = append(libs, domain.Library{
				Name:        m[1],
				Version:     version,
				Language:    "ruby",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return libs, scanner.Err()
}

// parseLockfileForOTel extracts OTel gems from Gemfile.lock
func (r *RubyDetector) parseLockfileForOTel(path string) ([]domain.Library, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	// lines like: opentelemetry-api (1.2.0)
	re := regexp.MustCompile(`^\s*(opentelemetry[^\s]+)\s*\(([^\)]+)\)`) // name (version)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := re.FindStringSubmatch(line); len(m) >= 3 {
			libs = append(libs, domain.Library{
				Name:        m[1],
				Version:     m[2],
				Language:    "ruby",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return libs, scanner.Err()
}

// parseRubyRequires extracts OTel requires from a Ruby source file
func (r *RubyDetector) parseRubyRequires(path string) ([]domain.Library, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`require\s+["'](opentelemetry[^"']*)["']`)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			libs = append(libs, domain.Library{
				Name:       m[1],
				Language:   "ruby",
				ImportPath: m[1],
			})
		}
	}
	return libs, scanner.Err()
}

// parseAllFromLockfile extracts all gems from Gemfile.lock
func (r *RubyDetector) parseAllFromLockfile(path string) ([]domain.Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pkgs []domain.Package
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^\s*([a-zA-Z0-9_\-]+)\s*\(([^\)]+)\)`) // name (version)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := re.FindStringSubmatch(line); len(m) >= 3 {
			pkgs = append(pkgs, domain.Package{
				Name:        m[1],
				Version:     m[2],
				Language:    "ruby",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return pkgs, scanner.Err()
}

// parseAllFromGemfile extracts all gem names from Gemfile
func (r *RubyDetector) parseAllFromGemfile(path string) ([]domain.Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pkgs []domain.Package
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`gem\s+["']([^"']+)["'](?:\s*,\s*["']([^"']+)["'])?`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			version := ""
			if len(m) >= 3 {
				version = m[2]
			}
			pkgs = append(pkgs, domain.Package{
				Name:        m[1],
				Version:     version,
				Language:    "ruby",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return pkgs, scanner.Err()
}

// parseAllRubyRequires extracts packages from require lines (best-effort)
func (r *RubyDetector) parseAllRubyRequires(path string) ([]domain.Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var pkgs []domain.Package
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`require\s+["']([^"']+)["']`)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			name := m[1]
			// Ignore relative requires
			if strings.HasPrefix(name, ".") || strings.HasPrefix(name, "/") {
				continue
			}
			root := strings.Split(name, "/")[0]
			pkgs = append(pkgs, domain.Package{
				Name:       root,
				Language:   "ruby",
				ImportPath: name,
			})
		}
	}
	return pkgs, scanner.Err()
}

// findRubyFiles recursively finds all .rb files
func (r *RubyDetector) findRubyFiles(rootPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "vendor", "node_modules":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".rb") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

// deduplicateLibraries removes duplicate library entries
func (r *RubyDetector) deduplicateLibraries(libs []domain.Library) []domain.Library {
	seen := make(map[string]bool)
	var res []domain.Library
	for _, l := range libs {
		key := fmt.Sprintf("%s:%s", l.Name, l.Version)
		if !seen[key] {
			seen[key] = true
			res = append(res, l)
		}
	}
	return res
}

// deduplicatePackages removes duplicate package entries
func (r *RubyDetector) deduplicatePackages(pkgs []domain.Package) []domain.Package {
	seen := make(map[string]bool)
	var res []domain.Package
	for _, p := range pkgs {
		key := fmt.Sprintf("%s:%s", p.Name, p.Version)
		if !seen[key] {
			seen[key] = true
			res = append(res, p)
		}
	}
	return res
}

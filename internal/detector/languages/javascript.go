package languages

import (
	"bufio"
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/domain"
)

// JavaScriptDetector detects JavaScript projects and OpenTelemetry usage
type JavaScriptDetector struct{}

// NewJavaScriptDetector creates a new JavaScript language detector
func NewJavaScriptDetector() *JavaScriptDetector { return &JavaScriptDetector{} }

// Name returns the language name
func (j *JavaScriptDetector) Name() string { return "javascript" }

// GetOTelLibraries finds OpenTelemetry libraries in JavaScript projects
func (j *JavaScriptDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Check package.json
	pkgJSON := filepath.Join(rootPath, "package.json")
	if _, err := os.Stat(pkgJSON); err == nil {
		libs, err := j.parsePackageJSON(pkgJSON)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Scan .js/.mjs for imports/requires
	jsFiles, err := j.findJavaScriptFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range jsFiles {
		libs, err := j.parseJSImports(file)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	return j.deduplicateLibraries(libraries), nil
}

// GetFilePatterns returns patterns for JavaScript files
func (j *JavaScriptDetector) GetFilePatterns() []string {
	return []string{"**/*.js", "**/*.mjs", "package.json", "package-lock.json", "yarn.lock", "pnpm-lock.yaml"}
}

// GetAllPackages finds all packages/dependencies used in the project
func (j *JavaScriptDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	// package.json dependencies
	pkgJSON := filepath.Join(rootPath, "package.json")
	if _, err := os.Stat(pkgJSON); err == nil {
		pkgs, err := j.parseAllFromPackageJSON(pkgJSON)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	// JS imports/requires
	jsFiles, err := j.findJavaScriptFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range jsFiles {
		pkgs, err := j.parseAllJSImports(file)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	return j.deduplicatePackages(packages), nil
}

// parsePackageJSON extracts OTel deps from package.json (simple regex-based scan)
func (j *JavaScriptDetector) parsePackageJSON(path string) ([]domain.Library, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var libraries []domain.Library
	// Match keys under dependencies/devDependencies with @opentelemetry/*
	re := regexp.MustCompile(`"(@opentelemetry\\/[^"]+)"\s*:\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(content), -1)
	for _, m := range matches {
		if len(m) >= 3 {
			libraries = append(libraries, domain.Library{
				Name:        m[1],
				Version:     m[2],
				Language:    "javascript",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return libraries, nil
}

// parseJSImports extracts OTel imports from JS source files
func (j *JavaScriptDetector) parseJSImports(filePath string) ([]domain.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	// import x from '@opentelemetry/api' OR require('@opentelemetry/api')
	importRE := regexp.MustCompile(`@opentelemetry\\/[^'"\\s]+`)

	for scanner.Scan() {
		line := scanner.Text()
		imps := importRE.FindAllString(line, -1)
		for _, imp := range imps {
			libraries = append(libraries, domain.Library{
				Name:       imp,
				Language:   "javascript",
				ImportPath: imp,
			})
		}
	}
	return libraries, scanner.Err()
}

// findJavaScriptFiles recursively finds JS files
func (j *JavaScriptDetector) findJavaScriptFiles(rootPath string) ([]string, error) {
	var files []string
	filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case "node_modules", ".git":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".js") || strings.HasSuffix(path, ".mjs") {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}

// deduplicateLibraries removes duplicates
func (j *JavaScriptDetector) deduplicateLibraries(libs []domain.Library) []domain.Library {
	seen := map[string]bool{}
	var res []domain.Library
	for _, l := range libs {
		key := l.Name + ":" + l.Version
		if !seen[key] {
			seen[key] = true
			res = append(res, l)
		}
	}
	return res
}

// parseAllFromPackageJSON extracts all deps from package.json (not only otel)
func (j *JavaScriptDetector) parseAllFromPackageJSON(path string) ([]domain.Package, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	// Match "name": "version" pairs in dependency blocks
	depRE := regexp.MustCompile(`"([@a-zA-Z0-9_\-/]+)"\s*:\s*"([^"]+)"`)
	var packages []domain.Package
	for _, m := range depRE.FindAllStringSubmatch(string(content), -1) {
		if len(m) >= 3 {
			packages = append(packages, domain.Package{
				Name:        m[1],
				Version:     m[2],
				Language:    "javascript",
				ImportPath:  m[1],
				PackageFile: path,
			})
		}
	}
	return packages, nil
}

// parseAllJSImports extracts all imports from JS files
func (j *JavaScriptDetector) parseAllJSImports(filePath string) ([]domain.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	// Common import/require patterns
	re := regexp.MustCompile(`(?:import\s+[^'";]+from\s+['"]([^'"]+)['"]|require\(\s*['"]([^'"]+)['"]\s*\))`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		m := re.FindStringSubmatch(line)
		if len(m) >= 2 {
			name := m[1]
			if name == "" && len(m) >= 3 {
				name = m[2]
			}
			if name != "" && j.isThirdParty(name) {
				root := name
				packages = append(packages, domain.Package{Name: root, Language: "javascript", ImportPath: name})
			}
		}
	}
	return packages, scanner.Err()
}

func (j *JavaScriptDetector) isThirdParty(name string) bool {
	// exclude relative and absolute paths
	return !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "/")
}

// deduplicatePackages removes duplicates
func (j *JavaScriptDetector) deduplicatePackages(pkgs []domain.Package) []domain.Package {
	seen := map[string]bool{}
	var res []domain.Package
	for _, p := range pkgs {
		key := p.Name + ":" + p.Version
		if !seen[key] {
			seen[key] = true
			res = append(res, p)
		}
	}
	return res
}

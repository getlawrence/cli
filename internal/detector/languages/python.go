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

// PythonDetector detects Python projects and OpenTelemetry usage
type PythonDetector struct{}

// NewPythonDetector creates a new Python language detector
func NewPythonDetector() *PythonDetector {
	return &PythonDetector{}
}

// Name returns the language name
func (p *PythonDetector) Name() string {
	return "python"
}

// GetOTelLibraries finds OpenTelemetry libraries in Python projects
func (p *PythonDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Check requirements.txt
	reqPath := filepath.Join(rootPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		libs, err := p.parseRequirements(reqPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// Check pyproject.toml
	pyprojectPath := filepath.Join(rootPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		libs, err := p.parsePyproject(pyprojectPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// Check Python imports
	pyFiles, err := p.findPythonFiles(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range pyFiles {
		libs, err := p.parsePythonImports(file)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	return p.deduplicateLibraries(libraries), nil
}

// GetFilePatterns returns patterns for Python files
func (p *PythonDetector) GetFilePatterns() []string {
	return []string{"**/*.py", "requirements.txt", "pyproject.toml", "setup.py", "Pipfile"}
}

// GetAllPackages finds all packages/dependencies used in the Python project
func (p *PythonDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	// Check requirements.txt
	reqPath := filepath.Join(rootPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		pkgs, err := p.parseAllRequirements(reqPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	// Check pyproject.toml
	pyprojectPath := filepath.Join(rootPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		pkgs, err := p.parseAllPyproject(pyprojectPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	// Check Python imports
	pyFiles, err := p.findPythonFiles(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range pyFiles {
		pkgs, err := p.parseAllPythonImports(file)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	return p.deduplicatePackages(packages), nil
}

// parseRequirements extracts OTel dependencies from requirements.txt
func (p *PythonDetector) parseRequirements(reqPath string) ([]domain.Library, error) {
	file, err := os.Open(reqPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)

	// Regex for matching OTel packages
	otelRegex := regexp.MustCompile(`^(opentelemetry[a-zA-Z0-9\-_]*)(==|>=|<=|>|<|~=)([^\s;#]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		// Skip comments and empty lines
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}

		matches := otelRegex.FindStringSubmatch(line)
		if len(matches) >= 4 {
			libraries = append(libraries, domain.Library{
				Name:        matches[1],
				Version:     matches[3],
				Language:    "python",
				ImportPath:  matches[1],
				PackageFile: reqPath,
			})
		}
	}

	return libraries, scanner.Err()
}

// parsePyproject extracts OTel dependencies from pyproject.toml
func (p *PythonDetector) parsePyproject(pyprojectPath string) ([]domain.Library, error) {
	file, err := os.Open(pyprojectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	inDependencies := false

	// Simple TOML parsing for dependencies section
	otelRegex := regexp.MustCompile(`"(opentelemetry[a-zA-Z0-9\-_]*)[^"]*"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.Contains(line, "dependencies") && strings.Contains(line, "=") {
			inDependencies = true
			continue
		}

		if inDependencies && strings.HasPrefix(line, "[") {
			inDependencies = false
			continue
		}

		if inDependencies {
			matches := otelRegex.FindStringSubmatch(line)
			if len(matches) >= 2 {
				libraries = append(libraries, domain.Library{
					Name:        matches[1],
					Language:    "python",
					ImportPath:  matches[1],
					PackageFile: pyprojectPath,
				})
			}
		}
	}

	return libraries, scanner.Err()
}

// parsePythonImports extracts OTel imports from Python source files
func (p *PythonDetector) parsePythonImports(filePath string) ([]domain.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)

	// Regex for matching OTel imports
	importRegex := regexp.MustCompile(`^(?:from\s+)?(opentelemetry[a-zA-Z0-9\._]*)\s*(?:import|$)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		matches := importRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			libraries = append(libraries, domain.Library{
				Name:       matches[1],
				Language:   "python",
				ImportPath: matches[1],
			})
		}
	}

	return libraries, scanner.Err()
}

// findPythonFiles recursively finds all Python files
func (p *PythonDetector) findPythonFiles(rootPath string) ([]string, error) {
	var pyFiles []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip common directories
		if info.IsDir() && (info.Name() == "__pycache__" || info.Name() == ".git" ||
			info.Name() == "venv" || info.Name() == ".venv" || info.Name() == "env") {
			return filepath.SkipDir
		}

		if strings.HasSuffix(path, ".py") {
			pyFiles = append(pyFiles, path)
		}

		return nil
	})

	return pyFiles, err
}

// deduplicateLibraries removes duplicate library entries
func (p *PythonDetector) deduplicateLibraries(libraries []domain.Library) []domain.Library {
	seen := make(map[string]bool)
	var result []domain.Library

	for _, lib := range libraries {
		key := fmt.Sprintf("%s:%s", lib.Name, lib.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, lib)
		}
	}

	return result
}

// parseAllRequirements extracts all dependencies from requirements.txt
func (p *PythonDetector) parseAllRequirements(reqPath string) ([]domain.Package, error) {
	file, err := os.Open(reqPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []domain.Package
	scanner := bufio.NewScanner(file)

	// Regex for matching package requirements
	packageRegex := regexp.MustCompile(`^([a-zA-Z0-9\-\_\.]+)(?:[>=<!\s]+([^\s#]+))?`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		matches := packageRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			version := ""
			if len(matches) >= 3 {
				version = matches[2]
			}

			packages = append(packages, domain.Package{
				Name:        matches[1],
				Version:     version,
				Language:    "python",
				ImportPath:  matches[1],
				PackageFile: reqPath,
			})
		}
	}

	return packages, scanner.Err()
}

// parseAllPyproject extracts all dependencies from pyproject.toml
func (p *PythonDetector) parseAllPyproject(pyprojectPath string) ([]domain.Package, error) {
	file, err := os.Open(pyprojectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	inDependencies := false

	// Regex for matching dependencies in TOML format
	depRegex := regexp.MustCompile(`^\s*"([^"]+)"\s*=`)
	depArrayRegex := regexp.MustCompile(`^\s*"([^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Check for dependencies section
		if line == "[tool.poetry.dependencies]" || line == "[project]" {
			inDependencies = true
			continue
		}

		// End of section
		if strings.HasPrefix(line, "[") && inDependencies {
			inDependencies = false
			continue
		}

		if inDependencies {
			// Handle TOML dependency format
			if matches := depRegex.FindStringSubmatch(line); len(matches) >= 2 {
				packages = append(packages, domain.Package{
					Name:        matches[1],
					Language:    "python",
					ImportPath:  matches[1],
					PackageFile: pyprojectPath,
				})
			} else if matches := depArrayRegex.FindStringSubmatch(line); len(matches) >= 2 {
				packages = append(packages, domain.Package{
					Name:        matches[1],
					Language:    "python",
					ImportPath:  matches[1],
					PackageFile: pyprojectPath,
				})
			}
		}
	}

	return packages, scanner.Err()
}

// parseAllPythonImports extracts all imports from Python source files
func (p *PythonDetector) parseAllPythonImports(filePath string) ([]domain.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []domain.Package
	scanner := bufio.NewScanner(file)

	// Regex for matching Python imports
	fromImportRegex := regexp.MustCompile(`^from\s+([a-zA-Z0-9\._]+)\s+import`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments and empty lines
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var packageName string

		// Handle "from package import ..." format
		if matches := fromImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			packageName = matches[1]
		} else if strings.HasPrefix(line, "import ") {
			// Handle "import package" format
			importLine := strings.TrimPrefix(line, "import ")
			parts := strings.Split(importLine, ",")
			if len(parts) > 0 {
				packageName = strings.TrimSpace(strings.Split(parts[0], " as ")[0])
			}
		}

		if packageName != "" && p.isThirdPartyPythonPackage(packageName) {
			// Get root package name (e.g., "requests.auth" -> "requests")
			rootPackage := strings.Split(packageName, ".")[0]

			packages = append(packages, domain.Package{
				Name:       rootPackage,
				Language:   "python",
				ImportPath: packageName,
			})
		}
	}

	return packages, scanner.Err()
}

// isThirdPartyPythonPackage determines if a package is third-party (not standard library)
func (p *PythonDetector) isThirdPartyPythonPackage(packageName string) bool {
	// Common Python standard library modules to exclude
	standardLibrary := []string{
		"os", "sys", "json", "re", "time", "datetime", "math", "random",
		"collections", "itertools", "functools", "operator", "copy",
		"pickle", "sqlite3", "threading", "multiprocessing", "subprocess",
		"urllib", "http", "email", "html", "xml", "logging", "unittest",
		"argparse", "configparser", "pathlib", "io", "csv", "base64",
		"hashlib", "hmac", "secrets", "uuid", "typing", "dataclasses",
		"enum", "contextlib", "warnings", "traceback", "__future__",
	}

	rootPackage := strings.Split(packageName, ".")[0]

	for _, stdLib := range standardLibrary {
		if rootPackage == stdLib {
			return false
		}
	}

	// If it's not in standard library and not a relative import, it's likely third-party
	return !strings.HasPrefix(packageName, ".")
}

// deduplicatePackages removes duplicate package entries
func (p *PythonDetector) deduplicatePackages(packages []domain.Package) []domain.Package {
	seen := make(map[string]bool)
	var result []domain.Package

	for _, pkg := range packages {
		key := fmt.Sprintf("%s:%s", pkg.Name, pkg.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, pkg)
		}
	}

	return result
}

package languages

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/getlawrence/cli/internal/detector"
)

// GoDetector detects Go projects and OpenTelemetry usage
type GoDetector struct{}

// NewGoDetector creates a new Go language detector
func NewGoDetector() *GoDetector {
	return &GoDetector{}
}

// Name returns the language name
func (g *GoDetector) Name() string {
	return "go"
}

// Detect checks if this is a Go project
func (g *GoDetector) Detect(ctx context.Context, rootPath string) (bool, error) {
	// Check for go.mod file
	goModPath := filepath.Join(rootPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		return true, nil
	}

	// Check for .go files
	goFiles, err := filepath.Glob(filepath.Join(rootPath, "**/*.go"))
	if err != nil {
		return false, err
	}

	return len(goFiles) > 0, nil
}

// GetOTelLibraries finds OpenTelemetry libraries in Go projects
func (g *GoDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]detector.Library, error) {
	var libraries []detector.Library

	// Check go.mod for OTel dependencies
	goModPath := filepath.Join(rootPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		libs, err := g.parseGoMod(goModPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// Check .go files for OTel imports
	goFiles, err := g.findGoFiles(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range goFiles {
		libs, err := g.parseGoImports(file)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	return g.deduplicateLibraries(libraries), nil
}

// GetFilePatterns returns patterns for Go files
func (g *GoDetector) GetFilePatterns() []string {
	return []string{"**/*.go", "go.mod", "go.sum"}
}

// GetAllPackages finds all packages/dependencies used in the Go project
func (g *GoDetector) GetAllPackages(ctx context.Context, rootPath string) ([]detector.Package, error) {
	var packages []detector.Package

	// Check go.mod for all dependencies
	goModPath := filepath.Join(rootPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		pkgs, err := g.parseAllDependencies(goModPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	// Check .go files for all imports
	goFiles, err := g.findGoFiles(rootPath)
	if err != nil {
		return nil, err
	}

	for _, file := range goFiles {
		pkgs, err := g.parseAllImports(file)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	return g.deduplicatePackages(packages), nil
}

// parseGoMod extracts OTel dependencies from go.mod
func (g *GoDetector) parseGoMod(goModPath string) ([]detector.Library, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []detector.Library
	scanner := bufio.NewScanner(file)

	// Regex for matching OTel dependencies
	otelRegex := regexp.MustCompile(`^\s*(go\.opentelemetry\.io/[^\s]+)\s+([^\s]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := otelRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			libraries = append(libraries, detector.Library{
				Name:        matches[1],
				Version:     matches[2],
				Language:    "go",
				ImportPath:  matches[1],
				PackageFile: goModPath,
			})
		}
	}

	return libraries, scanner.Err()
}

// parseGoImports extracts OTel imports from Go source files
func (g *GoDetector) parseGoImports(filePath string) ([]detector.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []detector.Library
	scanner := bufio.NewScanner(file)
	inImportBlock := false

	// Regex for matching OTel imports
	otelImportRegex := regexp.MustCompile(`"(go\.opentelemetry\.io/[^"]+)"`)
	singleImportRegex := regexp.MustCompile(`import\s+"(go\.opentelemetry\.io/[^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle single-line imports
		if matches := singleImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			libraries = append(libraries, detector.Library{
				Name:       matches[1],
				Language:   "go",
				ImportPath: matches[1],
			})
			continue
		}

		// Handle import blocks
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}

		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock {
			if matches := otelImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
				libraries = append(libraries, detector.Library{
					Name:       matches[1],
					Language:   "go",
					ImportPath: matches[1],
				})
			}
		}
	}

	return libraries, scanner.Err()
}

// findGoFiles recursively finds all Go files
func (g *GoDetector) findGoFiles(rootPath string) ([]string, error) {
	var goFiles []string

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip vendor and .git directories
		if info.IsDir() && (info.Name() == "vendor" || info.Name() == ".git") {
			return filepath.SkipDir
		}

		if strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}

		return nil
	})

	return goFiles, err
}

// deduplicateLibraries removes duplicate library entries
func (g *GoDetector) deduplicateLibraries(libraries []detector.Library) []detector.Library {
	seen := make(map[string]bool)
	var result []detector.Library

	for _, lib := range libraries {
		key := fmt.Sprintf("%s:%s", lib.Name, lib.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, lib)
		}
	}

	return result
}

// parseAllDependencies extracts all dependencies from go.mod
func (g *GoDetector) parseAllDependencies(goModPath string) ([]detector.Package, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []detector.Package
	scanner := bufio.NewScanner(file)

	// Regex for matching dependencies (including version)
	depRegex := regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)`)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle require blocks
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}

		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}

		// Handle single require line
		if strings.HasPrefix(line, "require ") {
			line = strings.TrimPrefix(line, "require ")
		}

		if inRequireBlock || strings.HasPrefix(scanner.Text(), "require ") {
			matches := depRegex.FindStringSubmatch(line)
			if len(matches) >= 3 {
				// Skip golang.org/x and standard library packages
				packageName := matches[1]
				if !strings.HasPrefix(packageName, "golang.org/x/") &&
					!strings.Contains(packageName, ".") {
					continue
				}

				packages = append(packages, detector.Package{
					Name:        packageName,
					Version:     strings.TrimSuffix(matches[2], " // indirect"),
					Language:    "go",
					ImportPath:  packageName,
					PackageFile: goModPath,
				})
			}
		}
	}

	return packages, scanner.Err()
}

// parseAllImports extracts all imports from Go source files
func (g *GoDetector) parseAllImports(filePath string) ([]detector.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []detector.Package
	scanner := bufio.NewScanner(file)
	inImportBlock := false

	// Regex for matching imports
	importRegex := regexp.MustCompile(`"([^"]+)"`)
	singleImportRegex := regexp.MustCompile(`import\s+"([^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Handle single-line imports
		if matches := singleImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			packageName := matches[1]
			if g.isThirdPartyPackage(packageName) {
				packages = append(packages, detector.Package{
					Name:       packageName,
					Language:   "go",
					ImportPath: packageName,
				})
			}
			continue
		}

		// Handle import blocks
		if strings.HasPrefix(line, "import (") {
			inImportBlock = true
			continue
		}

		if inImportBlock && line == ")" {
			inImportBlock = false
			continue
		}

		if inImportBlock {
			if matches := importRegex.FindStringSubmatch(line); len(matches) >= 2 {
				packageName := matches[1]
				if g.isThirdPartyPackage(packageName) {
					packages = append(packages, detector.Package{
						Name:       packageName,
						Language:   "go",
						ImportPath: packageName,
					})
				}
			}
		}
	}

	return packages, scanner.Err()
}

// isThirdPartyPackage determines if a package is third-party (not standard library)
func (g *GoDetector) isThirdPartyPackage(packageName string) bool {
	// Standard library packages don't contain dots or are specific known packages
	if !strings.Contains(packageName, ".") {
		return false
	}

	// Common third-party package patterns
	thirdPartyPrefixes := []string{
		"github.com/",
		"gitlab.com/",
		"go.uber.org/",
		"google.golang.org/",
		"gopkg.in/",
		"go.opentelemetry.io/",
	}

	for _, prefix := range thirdPartyPrefixes {
		if strings.HasPrefix(packageName, prefix) {
			return true
		}
	}

	// If it contains a dot and doesn't match known standard patterns, likely third-party
	return strings.Contains(packageName, ".")
}

// deduplicatePackages removes duplicate package entries
func (g *GoDetector) deduplicatePackages(packages []detector.Package) []detector.Package {
	seen := make(map[string]bool)
	var result []detector.Package

	for _, pkg := range packages {
		key := fmt.Sprintf("%s:%s", pkg.Name, pkg.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, pkg)
		}
	}

	return result
}

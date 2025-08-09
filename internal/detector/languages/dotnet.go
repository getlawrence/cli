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

// DotNetDetector detects .NET/C# projects and OpenTelemetry usage
type DotNetDetector struct{}

// NewDotNetDetector creates a new .NET language detector
func NewDotNetDetector() *DotNetDetector { return &DotNetDetector{} }

// Name returns the language name key used by the analyzer
// We align with enry's C# output and overall tool usage by handling "c#" as the map key.
func (d *DotNetDetector) Name() string { return "csharp" }

// GetOTelLibraries finds OpenTelemetry libraries in .NET projects
func (d *DotNetDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Scan .csproj files for OpenTelemetry packages
	csprojFiles, err := d.findCSProjFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, csproj := range csprojFiles {
		libs, err := d.parseCsProjForOTel(csproj)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Scan .cs files for using OpenTelemetry.*
	csFiles, err := d.findCSFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range csFiles {
		libs, err := d.parseCSForOTelUsings(file)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	return d.deduplicateLibraries(libraries), nil
}

// GetAllPackages finds all packages/dependencies used in the project
func (d *DotNetDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	csprojFiles, err := d.findCSProjFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, csproj := range csprojFiles {
		pkgs, err := d.parseAllFromCsProj(csproj)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	return d.deduplicatePackages(packages), nil
}

// GetFilePatterns returns patterns for .NET projects
func (d *DotNetDetector) GetFilePatterns() []string {
	return []string{"**/*.cs", "**/*.csproj"}
}

// Helpers
func (d *DotNetDetector) findCSProjFiles(rootPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "bin", "obj":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".csproj") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (d *DotNetDetector) findCSFiles(rootPath string) ([]string, error) {
	var files []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "bin", "obj":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".cs") {
			files = append(files, path)
		}
		return nil
	})
	return files, err
}

func (d *DotNetDetector) parseCsProjForOTel(path string) ([]domain.Library, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var libraries []domain.Library
	// Match OpenTelemetry packages in PackageReference Include
	re := regexp.MustCompile(`(?i)<PackageReference\s+Include=\"(OpenTelemetry(?:\.[^\"]*)?)\"\s*(?:Version=\"([^\"]+)\")?`) // simple
	for _, m := range re.FindAllStringSubmatch(string(content), -1) {
		if len(m) >= 2 {
			name := m[1]
			version := ""
			if len(m) >= 3 {
				version = m[2]
			}
			libraries = append(libraries, domain.Library{
				Name:        name,
				Version:     version,
				Language:    "csharp",
				ImportPath:  name,
				PackageFile: path,
			})
		}
	}
	return libraries, nil
}

func (d *DotNetDetector) parseCSForOTelUsings(path string) ([]domain.Library, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^\s*using\s+(OpenTelemetry(?:\.[A-Za-z0-9_]+)*)\s*;`)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			libraries = append(libraries, domain.Library{
				Name:       m[1],
				Language:   "csharp",
				ImportPath: m[1],
			})
		}
	}
	return libraries, scanner.Err()
}

func (d *DotNetDetector) parseAllFromCsProj(path string) ([]domain.Package, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var packages []domain.Package
	re := regexp.MustCompile(`<PackageReference\s+Include=\"([^\"]+)\"\s*(?:Version=\"([^\"]+)\")?`)
	for _, m := range re.FindAllStringSubmatch(string(content), -1) {
		if len(m) >= 2 {
			name := m[1]
			version := ""
			if len(m) >= 3 {
				version = m[2]
			}
			// Skip known SDK-style references that aren't external packages
			packages = append(packages, domain.Package{
				Name:        name,
				Version:     version,
				Language:    "csharp",
				ImportPath:  name,
				PackageFile: path,
			})
		}
	}
	return packages, nil
}

func (d *DotNetDetector) deduplicateLibraries(libraries []domain.Library) []domain.Library {
	seen := make(map[string]bool)
	var res []domain.Library
	for _, l := range libraries {
		key := fmt.Sprintf("%s:%s", l.Name, l.Version)
		if !seen[key] {
			seen[key] = true
			res = append(res, l)
		}
	}
	return res
}

func (d *DotNetDetector) deduplicatePackages(packages []domain.Package) []domain.Package {
	seen := make(map[string]bool)
	var res []domain.Package
	for _, p := range packages {
		key := fmt.Sprintf("%s:%s", p.Name, p.Version)
		if !seen[key] {
			seen[key] = true
			res = append(res, p)
		}
	}
	return res
}

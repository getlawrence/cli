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

// JavaDetector detects Java projects and OpenTelemetry usage
type JavaDetector struct{}

// NewJavaDetector creates a new Java language detector
func NewJavaDetector() *JavaDetector { return &JavaDetector{} }

// Name returns the language name
func (j *JavaDetector) Name() string { return "java" }

// GetOTelLibraries finds OpenTelemetry libraries in Java projects (Maven/Gradle and imports)
func (j *JavaDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Maven pom.xml
	pomPath := filepath.Join(rootPath, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		libs, err := j.parsePomForOTel(pomPath)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	// Gradle build files
	gradlePaths := []string{filepath.Join(rootPath, "build.gradle"), filepath.Join(rootPath, "build.gradle.kts"), filepath.Join(rootPath, "settings.gradle"), filepath.Join(rootPath, "settings.gradle.kts")}
	for _, p := range gradlePaths {
		if _, err := os.Stat(p); err == nil {
			libs, err := j.parseGradleForOTel(p)
			if err == nil {
				libraries = append(libraries, libs...)
			}
		}
	}

	// Source imports
	javaFiles, err := j.findJavaFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, f := range javaFiles {
		libs, err := j.parseJavaImportsForOTel(f)
		if err == nil {
			libraries = append(libraries, libs...)
		}
	}

	return j.deduplicateLibraries(libraries), nil
}

// GetFilePatterns returns patterns for Java files
func (j *JavaDetector) GetFilePatterns() []string {
	return []string{"**/*.java", "pom.xml", "build.gradle", "build.gradle.kts", "settings.gradle", "settings.gradle.kts"}
}

// GetAllPackages finds all packages/dependencies used in the Java project
func (j *JavaDetector) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	// Maven
	pomPath := filepath.Join(rootPath, "pom.xml")
	if _, err := os.Stat(pomPath); err == nil {
		pkgs, err := j.parseAllFromPom(pomPath)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	// Gradle
	gradlePaths := []string{filepath.Join(rootPath, "build.gradle"), filepath.Join(rootPath, "build.gradle.kts")}
	for _, p := range gradlePaths {
		if _, err := os.Stat(p); err == nil {
			pkgs, err := j.parseAllFromGradle(p)
			if err == nil {
				packages = append(packages, pkgs...)
			}
		}
	}

	// Imports
	javaFiles, err := j.findJavaFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, f := range javaFiles {
		pkgs, err := j.parseAllJavaImports(f)
		if err == nil {
			packages = append(packages, pkgs...)
		}
	}

	return j.deduplicatePackages(packages), nil
}

// --- Helpers ---

func (j *JavaDetector) parsePomForOTel(pomPath string) ([]domain.Library, error) {
	file, err := os.Open(pomPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	// Very simple XML regexes to detect artifact coordinates containing opentelemetry
	depRegex := regexp.MustCompile(`<(groupId|artifactId)>([^<]+)</\1>`) // capture group/artifact ids
	var groupID, artifactID string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := depRegex.FindStringSubmatch(line); len(m) == 3 {
			if m[1] == "groupId" {
				groupID = m[2]
			}
			if m[1] == "artifactId" {
				artifactID = m[2]
			}
			if groupID != "" && artifactID != "" {
				coord := groupID + ":" + artifactID
				if strings.Contains(coord, "opentelemetry") {
					libs = append(libs, domain.Library{Name: coord, Language: "java", ImportPath: coord, PackageFile: pomPath})
				}
				groupID, artifactID = "", ""
			}
		}
	}
	return libs, scanner.Err()
}

func (j *JavaDetector) parseGradleForOTel(path string) ([]domain.Library, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	// Match implementation("group:artifact:version") or implementation 'group:artifact:version'
	re := regexp.MustCompile(`['"]([a-zA-Z0-9_.\-]+:opentelemetry[^:'"]+)(?::[^'"\)]+)?['"]`)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			libs = append(libs, domain.Library{Name: m[1], Language: "java", ImportPath: m[1], PackageFile: path})
		}
	}
	return libs, scanner.Err()
}

func (j *JavaDetector) parseJavaImportsForOTel(filePath string) ([]domain.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libs []domain.Library
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^import\s+(io\.opentelemetry\.[a-zA-Z0-9_\.]+)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			libs = append(libs, domain.Library{Name: m[1], Language: "java", ImportPath: m[1]})
		}
	}
	return libs, scanner.Err()
}

func (j *JavaDetector) findJavaFiles(rootPath string) ([]string, error) {
	var files []string
	_ = filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			switch info.Name() {
			case ".git", "target", "build", "out":
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(strings.ToLower(path), ".java") {
			files = append(files, path)
		}
		return nil
	})
	return files, nil
}

func (j *JavaDetector) parseAllFromPom(pomPath string) ([]domain.Package, error) {
	file, err := os.Open(pomPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	depStart := regexp.MustCompile(`^<dependency>`) // naive
	depEnd := regexp.MustCompile(`^</dependency>`)
	gidRe := regexp.MustCompile(`^<groupId>([^<]+)</groupId>`)
	aidRe := regexp.MustCompile(`^<artifactId>([^<]+)</artifactId>`)
	verRe := regexp.MustCompile(`^<version>([^<]+)</version>`)
	inDep := false
	var gid, aid, ver string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if depStart.MatchString(line) {
			inDep = true
			gid, aid, ver = "", "", ""
			continue
		}
		if depEnd.MatchString(line) && inDep {
			if gid != "" && aid != "" {
				coord := gid + ":" + aid
				packages = append(packages, domain.Package{Name: coord, Version: ver, Language: "java", ImportPath: coord, PackageFile: pomPath})
			}
			inDep = false
			continue
		}
		if !inDep {
			continue
		}
		if m := gidRe.FindStringSubmatch(line); len(m) == 2 {
			gid = m[1]
		}
		if m := aidRe.FindStringSubmatch(line); len(m) == 2 {
			aid = m[1]
		}
		if m := verRe.FindStringSubmatch(line); len(m) == 2 {
			ver = m[1]
		}
	}
	return packages, scanner.Err()
}

func (j *JavaDetector) parseAllFromGradle(path string) ([]domain.Package, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`['"]([a-zA-Z0-9_.\-]+:[a-zA-Z0-9_.\-]+)(?::([^'"\)]+))?['"]`)
	for scanner.Scan() {
		line := scanner.Text()
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			coord := m[1]
			ver := ""
			if len(m) >= 3 {
				ver = m[2]
			}
			packages = append(packages, domain.Package{Name: coord, Version: ver, Language: "java", ImportPath: coord, PackageFile: path})
		}
	}
	return packages, scanner.Err()
}

func (j *JavaDetector) parseAllJavaImports(filePath string) ([]domain.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	re := regexp.MustCompile(`^import\s+([a-zA-Z0-9_\.]+)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if m := re.FindStringSubmatch(line); len(m) >= 2 {
			pkg := m[1]
			// Ignore java.* and javax.*
			if strings.HasPrefix(pkg, "java.") || strings.HasPrefix(pkg, "javax.") {
				continue
			}
			root := strings.Split(pkg, ".")[0]
			packages = append(packages, domain.Package{Name: root, Language: "java", ImportPath: pkg})
		}
	}
	return packages, scanner.Err()
}

func (j *JavaDetector) deduplicateLibraries(libs []domain.Library) []domain.Library {
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

func (j *JavaDetector) deduplicatePackages(pkgs []domain.Package) []domain.Package {
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

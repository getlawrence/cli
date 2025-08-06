package detector

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/go-enry/go-enry/v2"
)

// DetectLanguages scans a directory and detects the primary programming language of each subdirectory
func DetectLanguages(rootPath string) (map[string]string, error) {
	directories, err := collectLanguagesByDirectory(rootPath)
	if err != nil {
		return nil, err
	}

	return determinePrimaryLanguages(directories), nil
}

// collectLanguagesByDirectory walks the file tree and counts languages by directory
func collectLanguagesByDirectory(rootPath string) (map[string]map[string]int, error) {
	directories := make(map[string]map[string]int) // dir -> language -> count

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if shouldSkipFile(rootPath, path, info) {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if !info.IsDir() {
			processSourceFile(path, rootPath, directories)
		}

		return nil
	})

	return directories, err
}

// processSourceFile processes a single source file and updates the language count for its directory
func processSourceFile(filePath, rootPath string, directories map[string]map[string]int) {
	lang := detectFileLanguage(filePath)
	if lang == "" || !isProgrammingLanguage(lang) {
		return
	}

	// Normalize language names
	lang = normalizeLanguageName(lang)

	dir := filepath.Dir(filePath)
	relDir, _ := filepath.Rel(rootPath, dir)

	// Normalize the relative directory path
	if relDir == "." {
		relDir = ""
	}

	if directories[relDir] == nil {
		directories[relDir] = make(map[string]int)
	}
	directories[relDir][lang]++
}

// determinePrimaryLanguages finds the most common language in each directory
func determinePrimaryLanguages(directories map[string]map[string]int) map[string]string {
	dirToLangMap := make(map[string]string)

	for dir, langCounts := range directories {
		primaryLang := findMostCommonLanguage(langCounts)
		if primaryLang != "" {
			dirKey := normalizeDirectoryKey(dir)
			dirToLangMap[dirKey] = primaryLang
		}
	}

	return dirToLangMap
}

// findMostCommonLanguage returns the language with the highest count
func findMostCommonLanguage(langCounts map[string]int) string {
	primaryLang := ""
	maxCount := 0

	for lang, count := range langCounts {
		if count > maxCount {
			maxCount = count
			primaryLang = lang
		}
	}

	return primaryLang
}

// normalizeDirectoryKey converts directory path to a normalized key
func normalizeDirectoryKey(dir string) string {
	if dir == "" {
		return "root"
	}
	return dir
}

// normalizeLanguageName normalizes language names to handle variations
func normalizeLanguageName(lang string) string {
	switch lang {
	case "Go Module":
		return "Go"
	default:
		return lang
	}
}

// isProgrammingLanguage filters out configuration, markup, and documentation languages
func isProgrammingLanguage(lang string) bool {
	// Configuration and markup languages to skip
	configLanguages := map[string]bool{
		"YAML":       true,
		"JSON":       true,
		"TOML":       true,
		"XML":        true,
		"Markdown":   true,
		"HTML":       true,
		"CSS":        true,
		"SCSS":       true,
		"Less":       true,
		"Dockerfile": true,
		"Makefile":   true,
		"INI":        true,
		"Properties": true,
		"Text":       true,
		"CSV":        true,
		"SVG":        true,
	}

	return !configLanguages[lang]
}

// shouldSkipFile determines if a file or directory should be skipped during language detection
func shouldSkipFile(rootPath, path string, info os.FileInfo) bool {
	// Skip directories (they will be traversed)
	if info.IsDir() {
		return false
	}

	// Skip hidden files
	if strings.HasPrefix(info.Name(), ".") {
		return true
	}

	// Skip common non-source directories
	relPath, _ := filepath.Rel(rootPath, path)
	pathParts := strings.Split(relPath, string(filepath.Separator))

	skipDirs := []string{"node_modules", "vendor", ".git", "__pycache__", ".venv", "venv"}
	for _, part := range pathParts {
		for _, skipDir := range skipDirs {
			if part == skipDir {
				return true
			}
		}
	}

	return false
}

func detectFileLanguage(path string) string {
	lang, safe := enry.GetLanguageByExtension(path)
	if safe && lang != "" {
		return lang
	}

	content, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return enry.GetLanguage(path, content)
}

func DetectLanguageForFile(filePath string) (string, error) {
	lang := detectFileLanguage(filePath)
	return lang, nil
}

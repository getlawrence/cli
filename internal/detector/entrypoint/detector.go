package entrypoint

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/domain"
	langplugins "github.com/getlawrence/cli/internal/languages"
	sitter "github.com/smacker/go-tree-sitter"
)

// TreeSitterEntryDetector uses Tree-sitter for multi-language entry point detection
type TreeSitterEntryDetector struct {
	languages map[string]*sitter.Language
	queries   map[string]string
}

// NewTreeSitterEntryDetector creates a new detector with language support
func NewTreeSitterEntryDetector() *TreeSitterEntryDetector {
	d := &TreeSitterEntryDetector{
		languages: make(map[string]*sitter.Language),
		queries:   make(map[string]string),
	}
	// Populate from language plugins
	for _, plugin := range langplugins.DefaultRegistry.All() {
		lang := plugin.DisplayName()
		if ts := plugin.EntryPointTreeSitterLanguage(); ts != nil {
			d.languages[lang] = ts
			if q := plugin.EntrypointQuery(); q != "" {
				d.queries[lang] = q
			}
		}
	}
	return d
}

// DetectEntryPoints finds entry points in the specified language
// Returns one entry point per directory to keep things simple
func (d *TreeSitterEntryDetector) DetectEntryPoints(projectPath, language string) ([]domain.EntryPoint, error) {
	lang, exists := d.languages[language]
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	query, exists := d.queries[language]
	if !exists {
		return nil, fmt.Errorf("no query defined for language: %s", language)
	}

	dirEntryPoints, err := d.collectEntryPointsByDirectory(projectPath, lang, query, language)
	if err != nil {
		return nil, err
	}

	return d.convertMapToSlice(dirEntryPoints), nil
}

// collectEntryPointsByDirectory walks through the project and collects the best entry point per directory
func (d *TreeSitterEntryDetector) collectEntryPointsByDirectory(projectPath string, lang *sitter.Language, query, language string) (map[string]domain.EntryPoint, error) {
	dirEntryPoints := make(map[string]domain.EntryPoint)
	fileExtensions := d.getFileExtensions(language)

	skipDirs := map[string]struct{}{
		".git": {}, "node_modules": {}, "vendor": {}, "venv": {}, ".venv": {}, "__pycache__": {},
	}

	err := filepath.WalkDir(projectPath, func(path string, de fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if de.IsDir() {
			name := de.Name()
			if _, skip := skipDirs[name]; skip {
				return filepath.SkipDir
			}
			return nil
		}

		if !d.hasValidExtension(path, fileExtensions) {
			return nil
		}

		return d.processFileForEntryPoints(path, lang, query, language, dirEntryPoints)
	})

	return dirEntryPoints, err
}

// processFileForEntryPoints analyzes a file and updates the directory entry points map
func (d *TreeSitterEntryDetector) processFileForEntryPoints(path string, lang *sitter.Language, query, language string, dirEntryPoints map[string]domain.EntryPoint) error {
	entries, err := d.analyzeFile(path, lang, query, language)
	if err != nil {
		// Swallow per-file analysis errors to keep directory scan resilient
		return nil
	}

	for _, entry := range entries {
		d.updateBestEntryPointForDirectory(entry, dirEntryPoints)
	}

	return nil
}

// updateBestEntryPointForDirectory updates the entry point for a directory if the new one has higher confidence
func (d *TreeSitterEntryDetector) updateBestEntryPointForDirectory(entry domain.EntryPoint, dirEntryPoints map[string]domain.EntryPoint) {
	dir := filepath.Dir(entry.FilePath)

	if existing, exists := dirEntryPoints[dir]; !exists || entry.Confidence > existing.Confidence {
		dirEntryPoints[dir] = entry
	}
}

// convertMapToSlice converts the directory entry points map to a slice
func (d *TreeSitterEntryDetector) convertMapToSlice(dirEntryPoints map[string]domain.EntryPoint) []domain.EntryPoint {
	var entryPoints []domain.EntryPoint
	for _, entryPoint := range dirEntryPoints {
		entryPoints = append(entryPoints, entryPoint)
	}
	return entryPoints
}

// analyzeFile parses a single file and extracts entry points
func (d *TreeSitterEntryDetector) analyzeFile(filePath string, lang *sitter.Language, queryStr, language string) ([]domain.EntryPoint, error) {
	// Read file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Create parser
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	// Parse the file
	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, err
	}
	defer tree.Close()

	// Create query
	q, err := sitter.NewQuery([]byte(queryStr), lang)
	if err != nil {
		return nil, err
	}
	defer q.Close()

	// Execute query
	qc := sitter.NewQueryCursor()
	defer qc.Close()

	qc.Exec(q, tree.RootNode())

	var entryPoints []domain.EntryPoint

	// Process matches
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			capture := q.CaptureNameForId(c.Index)
			node := c.Node

			entryPoint := domain.EntryPoint{
				FilePath:   filePath,
				Language:   language,
				NodeType:   capture,
				LineNumber: node.StartPoint().Row + 1, // Tree-sitter uses 0-based indexing
				Column:     node.StartPoint().Column + 1,
				Context:    node.Content(content),
			}

			// Set function name and confidence based on capture type
			switch capture {
			case "main_function", "main_method", "main_block":
				entryPoint.FunctionName = "main"
				entryPoint.Confidence = 1.0
			case "main_if_block":
				entryPoint.FunctionName = "if __name__ == '__main__'"
				entryPoint.Confidence = 1.0
			case "init_function":
				entryPoint.FunctionName = "init"
				entryPoint.Confidence = 0.9
			case "http_server", "web_server", "server_listen":
				entryPoint.FunctionName = "web_server"
				entryPoint.Confidence = 0.8
			case "async_main":
				entryPoint.FunctionName = "async_main"
				entryPoint.Confidence = 1.0
			default:
				entryPoint.FunctionName = "unknown"
				entryPoint.Confidence = 0.5
			}

			entryPoints = append(entryPoints, entryPoint)
		}
	}

	// Fallback for Python: use regex if tree-sitter query found nothing
	if len(entryPoints) == 0 && language == "Python" {
		entryPoints = d.findPythonMainWithRegex(filePath, content)
	}

	return entryPoints, nil
}

// findPythonMainWithRegex uses regex to find Python if __name__ == '__main__': patterns
func (d *TreeSitterEntryDetector) findPythonMainWithRegex(filePath string, content []byte) []domain.EntryPoint {
	var entryPoints []domain.EntryPoint
	lines := strings.Split(string(content), "\n")

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `if __name__ == '__main__'`) || strings.Contains(trimmed, `if __name__ == "__main__"`) {
			entryPoint := domain.EntryPoint{
				FilePath:     filePath,
				Language:     "Python",
				FunctionName: "if __name__ == '__main__'",
				LineNumber:   uint32(i + 1), // 1-based line numbers
				Column:       1,
				NodeType:     "main_if_block",
				Confidence:   1.0,
				Context:      trimmed,
			}
			entryPoints = append(entryPoints, entryPoint)
			break // Only one main block per file
		}
	}

	return entryPoints
}

// getFileExtensions returns file extensions for a language
func (d *TreeSitterEntryDetector) getFileExtensions(language string) []string {
	// Resolve from plugins by display name
	for _, plugin := range langplugins.DefaultRegistry.All() {
		if plugin.DisplayName() == language {
			return plugin.FileExtensions()
		}
	}
	return nil
}

// hasValidExtension checks if file has valid extension for the language
func (d *TreeSitterEntryDetector) hasValidExtension(filePath string, extensions []string) bool {
	ext := strings.ToLower(filepath.Ext(filePath))
	for _, validExt := range extensions {
		if ext == validExt {
			return true
		}
	}
	return false
}

// DetectAllEntryPoints detects entry points across all supported languages
func (d *TreeSitterEntryDetector) DetectAllEntryPoints(projectPath string) (map[string][]domain.EntryPoint, error) {
	allEntryPoints := make(map[string][]domain.EntryPoint)

	for language := range d.languages {
		entryPoints, err := d.DetectEntryPoints(projectPath, language)
		if err != nil {
			continue
		}

		if len(entryPoints) > 0 {
			allEntryPoints[language] = entryPoints
		}
	}

	return allEntryPoints, nil
}

// Enhanced detection with framework patterns
func (d *TreeSitterEntryDetector) DetectFrameworkPatterns(projectPath, language string) ([]domain.EntryPoint, error) {
	// Implementation similar to DetectEntryPoints but with framework-specific queries
	// This allows for more targeted instrumentation
	return d.DetectEntryPoints(projectPath, language)
}

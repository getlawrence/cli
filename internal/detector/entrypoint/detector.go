package entrypoint

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/detector/types"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
)

// TreeSitterEntryDetector uses Tree-sitter for multi-language entry point detection
type TreeSitterEntryDetector struct {
	languages map[string]*sitter.Language
	queries   map[string]string
}

// NewTreeSitterEntryDetector creates a new detector with language support
func NewTreeSitterEntryDetector() *TreeSitterEntryDetector {
	return &TreeSitterEntryDetector{
		languages: map[string]*sitter.Language{
			"Go":     golang.GetLanguage(),
			"Python": python.GetLanguage(),
		},
		queries: map[string]string{
			"Go": `
				(function_declaration 
					name: (identifier) @func_name
					(#eq? @func_name "main")
				) @main_function
			`,
			"Python": `
				(if_statement
					condition: (binary_operator
						left: (identifier) @name_var
						right: (string) @main_str
					)
					(#eq? @name_var "__name__")
					(#match? @main_str ".*__main__.*")
				) @main_if_block
				
				(function_definition
					name: (identifier) @func_name
					(#eq? @func_name "main")
				) @main_function
			`,
		},
	}
}

// DetectEntryPoints finds entry points in the specified language
func (d *TreeSitterEntryDetector) DetectEntryPoints(projectPath, language string) ([]types.EntryPoint, error) {
	lang, exists := d.languages[language]
	if !exists {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	query, exists := d.queries[language]
	if !exists {
		return nil, fmt.Errorf("no query defined for language: %s", language)
	}

	var entryPoints []types.EntryPoint
	fileExtensions := d.getFileExtensions(language)

	err := filepath.Walk(projectPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() || !d.hasValidExtension(path, fileExtensions) {
			return nil
		}

		entries, err := d.analyzeFile(path, lang, query, language)
		if err != nil {
			// Log error but continue with other files
			fmt.Printf("Warning: Could not analyze %s: %v\n", path, err)
			return nil
		}

		entryPoints = append(entryPoints, entries...)
		return nil
	})

	return entryPoints, err
}

// analyzeFile parses a single file and extracts entry points
func (d *TreeSitterEntryDetector) analyzeFile(filePath string, lang *sitter.Language, queryStr, language string) ([]types.EntryPoint, error) {
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

	var entryPoints []types.EntryPoint

	// Process matches
	for {
		m, ok := qc.NextMatch()
		if !ok {
			break
		}

		for _, c := range m.Captures {
			capture := q.CaptureNameForId(c.Index)
			node := c.Node

			entryPoint := types.EntryPoint{
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

	return entryPoints, nil
}

// getFileExtensions returns file extensions for a language
func (d *TreeSitterEntryDetector) getFileExtensions(language string) []string {
	extensions := map[string][]string{
		"Go":         {".go"},
		"Python":     {".py", ".pyw"},
		"JavaScript": {".js", ".mjs"},
		"TypeScript": {".ts", ".tsx"},
		"Java":       {".java"},
		"Rust":       {".rs"},
		"C++":        {".cpp", ".cxx", ".cc", ".hpp", ".hxx"},
		"C":          {".c", ".h"},
	}
	return extensions[language]
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
func (d *TreeSitterEntryDetector) DetectAllEntryPoints(projectPath string) (map[string][]types.EntryPoint, error) {
	allEntryPoints := make(map[string][]types.EntryPoint)

	for language := range d.languages {
		entryPoints, err := d.DetectEntryPoints(projectPath, language)
		if err != nil {
			fmt.Printf("Warning: Error detecting entry points for %s: %v\n", language, err)
			continue
		}

		if len(entryPoints) > 0 {
			allEntryPoints[language] = entryPoints
		}
	}

	return allEntryPoints, nil
}

// Enhanced detection with framework patterns
func (d *TreeSitterEntryDetector) DetectFrameworkPatterns(projectPath, language string) ([]types.EntryPoint, error) {
	// Implementation similar to DetectEntryPoints but with framework-specific queries
	// This allows for more targeted instrumentation
	return d.DetectEntryPoints(projectPath, language)
}

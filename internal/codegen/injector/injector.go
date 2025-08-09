package injector

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	sitter "github.com/smacker/go-tree-sitter"
)

type CodeInjector struct {
	handlers map[string]LanguageInjector
}

func NewCodeInjector() *CodeInjector {
	return &CodeInjector{
		handlers: map[string]LanguageInjector{
			"go":         NewGoHandler(),
			"javascript": NewJavaScriptHandler(),
			"python":     NewPythonHandler(),
			"java":       NewJavaHandler(),
			"c#":         NewDotNetHandler(),
			"dotnet":     NewDotNetHandler(),
			"ruby":       NewRubyHandler(),
		},
	}
}

// DetectEntryPoints scans a project directory for entry points using language handlers.
// Returns at most one best entry point per directory.
func (ci *CodeInjector) DetectEntryPoints(projectPath string, language string) ([]domain.EntryPoint, error) {
	handler, ok := ci.handlers[strings.ToLower(language)]
	if !ok {
		return nil, fmt.Errorf("unsupported language: %s", language)
	}

	config := handler.GetConfig()
	validExts := make(map[string]bool)
	for _, ext := range config.FileExtensions {
		validExts[strings.ToLower(ext)] = true
	}

	// Best entrypoint per directory
	dirBest := make(map[string]domain.EntryPoint)

	walkFn := func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Skip hidden and common vendor/build directories
			name := d.Name()
			if strings.HasPrefix(name, ".") {
				return filepath.SkipDir
			}
			switch name {
			case "node_modules", "vendor", ".git", "__pycache__", ".venv", "venv", "dist", "build", "target", "out":
				return filepath.SkipDir
			}
			return nil
		}

		// Defensive: if file resides under ignored directories, skip it
		if rel, relErr := filepath.Rel(projectPath, path); relErr == nil {
			parts := strings.Split(rel, string(filepath.Separator))
			for _, part := range parts {
				if strings.HasPrefix(part, ".") {
					return nil
				}
				switch part {
				case "node_modules", "vendor", ".git", "__pycache__", ".venv", "venv", "dist", "build", "target", "out":
					return nil
				}
			}
		}
		ext := strings.ToLower(filepath.Ext(path))
		if !validExts[ext] {
			return nil
		}

		analysis, err := ci.analyzeFile(path, handler)
		if err != nil {
			// best-effort: skip unreadable/unparseable files
			return nil
		}

		if len(analysis.EntryPoints) == 0 {
			return nil
		}

		// Choose first entry point as best for the file, assign confidence
		epInfo := analysis.EntryPoints[0]
		confidence := 0.8
		if strings.EqualFold(epInfo.Name, "main") || strings.Contains(strings.ToLower(epInfo.Name), "main") {
			confidence = 1.0
		}
		dir := filepath.Dir(path)

		entry := domain.EntryPoint{
			FilePath:     path,
			Language:     config.Language,
			FunctionName: epInfo.Name,
			LineNumber:   epInfo.BodyStart.LineNumber,
			Column:       epInfo.BodyStart.Column,
			NodeType:     "main_function",
			Confidence:   confidence,
			Context:      "",
		}

		if existing, exists := dirBest[dir]; !exists || entry.Confidence > existing.Confidence {
			dirBest[dir] = entry
		}
		return nil
	}

	if err := filepath.WalkDir(projectPath, walkFn); err != nil {
		return nil, err
	}

	var result []domain.EntryPoint
	for _, ep := range dirBest {
		result = append(result, ep)
	}
	return result, nil
}

func (ci *CodeInjector) InjectOtelInitialization(ctx context.Context,
	entryPoint *domain.EntryPoint,
	operationsData *types.OperationsData,
	req types.GenerationRequest) ([]string, error) {

	handler, exists := ci.handlers[strings.ToLower(entryPoint.Language)]
	if !exists {
		return nil, fmt.Errorf("unsupported language for modification: %s", entryPoint.Language)
	}

	// Analyze the current file
	analysis, err := ci.analyzeFile(entryPoint.FilePath, handler)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze file %s: %w", entryPoint.FilePath, err)
	}

	// Generate modifications
	modifications, err := ci.generateModifications(analysis, operationsData, handler, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate modifications: %w", err)
	}

	// Apply modifications
	if err := ci.applyModifications(entryPoint.FilePath, modifications, req.Config.DryRun); err != nil {
		return nil, fmt.Errorf("failed to apply modifications: %w", err)
	}

	return []string{entryPoint.FilePath}, nil
}

// analyzeFile analyzes a source file to understand its structure
func (ci *CodeInjector) analyzeFile(filePath string, handler LanguageInjector) (*types.FileAnalysis, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lang := handler.GetLanguage()
	config := handler.GetConfig()
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	defer tree.Close()

	analysis := &types.FileAnalysis{
		Language:        config.Language,
		FilePath:        filePath,
		ExistingImports: make(map[string]bool),
		FunctionBodies:  make(map[string]types.InsertionPoint),
	}

	// Analyze imports
	if err := ci.analyzeImports(tree, content, handler, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze imports: %w", err)
	}

	// Analyze entry points
	if err := ci.analyzeEntryPoints(tree, content, handler, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze entry points: %w", err)
	}

	return analysis, nil
}

// analyzeImports analyzes existing imports and finds import insertion points
func (ci *CodeInjector) analyzeImports(tree *sitter.Tree, content []byte, handler LanguageInjector, analysis *types.FileAnalysis) error {
	config := handler.GetConfig()
	if importQuery, exists := config.ImportQueries["existing_imports"]; exists {
		query, err := sitter.NewQuery([]byte(importQuery), handler.GetLanguage())
		if err != nil {
			return fmt.Errorf("failed to create import query: %w", err)
		}
		defer query.Close()

		cursor := sitter.NewQueryCursor()
		defer cursor.Close()

		cursor.Exec(query, tree.RootNode())

		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			for _, capture := range match.Captures {
				captureName := query.CaptureNameForId(capture.Index)
				node := capture.Node

				// Use the handler's analyze method instead of the hardcoded switch
				handler.AnalyzeImportCapture(captureName, node, content, analysis)
			}
		}
	}

	// Allow language-specific fallback if no import locations were found
	if len(analysis.ImportLocations) == 0 {
		handler.FallbackAnalyzeImports(content, analysis)
	}

	return nil
}

// analyzeEntryPoints analyzes entry points and function bodies
func (ci *CodeInjector) analyzeEntryPoints(tree *sitter.Tree, content []byte, handler LanguageInjector, analysis *types.FileAnalysis) error {
	config := handler.GetConfig()
	if functionQuery, exists := config.FunctionQueries["main_function"]; exists {
		query, err := sitter.NewQuery([]byte(functionQuery), handler.GetLanguage())
		if err != nil {
			return fmt.Errorf("failed to create function query: %w", err)
		}
		defer query.Close()

		cursor := sitter.NewQueryCursor()
		defer cursor.Close()

		cursor.Exec(query, tree.RootNode())

		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			for _, capture := range match.Captures {
				captureName := query.CaptureNameForId(capture.Index)
				node := capture.Node

				// Use the handler's analyze method instead of the hardcoded switch
				handler.AnalyzeFunctionCapture(captureName, node, content, analysis, config)
			}
		}
	}

	// If no entry points found via tree-sitter, allow language-specific fallback
	if len(analysis.EntryPoints) == 0 {
		handler.FallbackAnalyzeEntryPoints(content, analysis)
	}

	return nil
}

// generateModifications creates the list of modifications to apply
func (ci *CodeInjector) generateModifications(
	analysis *types.FileAnalysis,
	operationsData *types.OperationsData,
	handler LanguageInjector,
	req types.GenerationRequest,
) ([]types.CodeModification, error) {
	var modifications []types.CodeModification
	config := handler.GetConfig()

	// Generate import modifications
	if !analysis.HasOTELImports && operationsData.InstallOTEL {
		importMods := ci.generateImportModifications(analysis, operationsData, handler)
		modifications = append(modifications, importMods...)
	}

	// Generate initialization modifications
	// Ensure we only insert initialization once per file even if multiple entry points are detected
	addedInit := false
	for _, entryPoint := range analysis.EntryPoints {
		if addedInit {
			break
		}
		if !entryPoint.HasOTELSetup && operationsData.InstallOTEL {
			initMod := ci.generateInitializationModification(entryPoint, operationsData, config, req)
			modifications = append(modifications, initMod)
			addedInit = true
		}
	}

	return modifications, nil
}

// generateImportModifications creates import-related modifications
func (ci *CodeInjector) generateImportModifications(
	analysis *types.FileAnalysis,
	operationsData *types.OperationsData,
	handler LanguageInjector,
) []types.CodeModification {
	var modifications []types.CodeModification

	requiredImports := handler.GetRequiredImports()
	requiredImports = append(requiredImports, operationsData.InstallInstrumentations...)
	newImports := make([]string, 0)

	// Collect imports that need to be added
	for _, importPath := range requiredImports {
		if !analysis.ExistingImports[importPath] {
			newImports = append(newImports, importPath)
		}
	}

	if len(newImports) == 0 {
		return modifications
	}

	// Find best insertion point for imports
	var insertionPoint types.InsertionPoint
	if len(analysis.ImportLocations) > 0 {
		// Find the insertion point with highest priority
		insertionPoint = analysis.ImportLocations[0]
		for _, location := range analysis.ImportLocations {
			if location.Priority > insertionPoint.Priority {
				insertionPoint = location
			}
		}
	} else {
		// Default to top of file after package declaration
		insertionPoint = types.InsertionPoint{LineNumber: 3, Column: 1, Priority: 1}
	}

	// Generate import code using the handler
	hasExistingImports := len(analysis.ImportLocations) > 0
	importCode := handler.FormatImports(newImports, hasExistingImports)

	if importCode != "" {
		modifications = append(modifications, types.CodeModification{
			Type:        types.ModificationAddImport,
			Language:    analysis.Language,
			FilePath:    analysis.FilePath,
			LineNumber:  insertionPoint.LineNumber,
			Column:      insertionPoint.Column,
			InsertAfter: true,
			Content:     importCode,
		})
	}

	return modifications
}

// generateInitializationModification creates OTEL initialization modification
func (ci *CodeInjector) generateInitializationModification(
	entryPoint types.EntryPointInfo,
	operationsData *types.OperationsData,
	config *types.LanguageConfig,
	req types.GenerationRequest,
) types.CodeModification {
	// Generate initialization code from template
	templateData := map[string]interface{}{
		"ServiceName":       filepath.Base(req.CodebasePath),
		"Instrumentations":  operationsData.InstallInstrumentations,
		"InstallComponents": operationsData.InstallComponents,
	}

	initCode := ci.generateFromTemplate(config.InitializationTemplate, templateData)

	return types.CodeModification{
		Type:         types.ModificationAddInit,
		Language:     config.Language,
		LineNumber:   entryPoint.BodyStart.LineNumber,
		Column:       entryPoint.BodyStart.Column,
		InsertBefore: false,
		InsertAfter:  true,
		Content:      initCode,
	}
}

// generateFromTemplate generates code from a template with data
func (ci *CodeInjector) generateFromTemplate(templateStr string, data map[string]interface{}) string {
	// For now, return the template as-is. In a full implementation,
	// you would use the text/template package to process the template
	return templateStr
}

// applyModifications applies the generated modifications to the source file
func (ci *CodeInjector) applyModifications(filePath string, modifications []types.CodeModification, dryRun bool) error {
	if len(modifications) == 0 {
		return nil
	}

	// Read the original file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	lines := strings.Split(string(content), "\n")

	// Sort modifications by line number (reverse order to avoid offset issues)
	// Apply modifications from bottom to top
	for i := len(modifications) - 1; i >= 0; i-- {
		mod := modifications[i]

		if int(mod.LineNumber) > len(lines) {
			continue // Skip invalid line numbers
		}

		// Insert the modification content
		if mod.InsertAfter {
			// Insert after the specified line
			if int(mod.LineNumber) <= len(lines) {
				newLines := make([]string, len(lines)+1)
				copy(newLines[:mod.LineNumber], lines[:mod.LineNumber])
				newLines[mod.LineNumber] = mod.Content
				copy(newLines[mod.LineNumber+1:], lines[mod.LineNumber:])
				lines = newLines
			}
		} else if mod.InsertBefore {
			// Insert before the specified line
			if int(mod.LineNumber) > 0 {
				newLines := make([]string, len(lines)+1)
				copy(newLines[:mod.LineNumber-1], lines[:mod.LineNumber-1])
				newLines[mod.LineNumber-1] = mod.Content
				copy(newLines[mod.LineNumber:], lines[mod.LineNumber-1:])
				lines = newLines
			}
		}
	}

	modifiedContent := strings.Join(lines, "\n")

	if dryRun {
		fmt.Printf("Would modify file: %s\n", filePath)
		fmt.Printf("Modifications:\n")
		for _, mod := range modifications {
			fmt.Printf("  Line %d: %s\n", mod.LineNumber, strings.TrimSpace(mod.Content))
		}
		return nil
	}

	// Create backup
	backupPath := filePath + ".backup"
	if err := os.WriteFile(backupPath, content, 0644); err != nil {
		fmt.Printf("Warning: failed to create backup: %v\n", err)
	}

	// Write modified content
	if err := os.WriteFile(filePath, []byte(modifiedContent), 0644); err != nil {
		return fmt.Errorf("failed to write modified file: %w", err)
	}

	fmt.Printf("Successfully modified: %s (backup: %s)\n", filePath, backupPath)
	return nil
}

package injector

import (
	"context"
	"fmt"
	gformat "go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	sitter "github.com/smacker/go-tree-sitter"
)

type CodeInjector struct {
	handlers map[string]LanguageInjector
}

// registry for injectors to avoid import cycle with languages
var injectorRegistry = map[string]LanguageInjector{}

func RegisterLanguageInjector(id string, h LanguageInjector) {
	injectorRegistry[strings.ToLower(id)] = h
}

func getRegisteredLanguageInjectors() map[string]LanguageInjector {
	out := make(map[string]LanguageInjector, len(injectorRegistry))
	for k, v := range injectorRegistry {
		out[k] = v
	}
	return out
}

func NewCodeInjector() *CodeInjector {
	handlers := make(map[string]LanguageInjector)
	// Populate from package-level registry
	for id, h := range getRegisteredLanguageInjectors() {
		handlers[strings.ToLower(id)] = h
	}
	// Fallback defaults
	// go handler is registered by plugin init; no local fallback
	// python handler now registered by plugin init; no local fallback
	return &CodeInjector{handlers: handlers}
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

	return nil
}

// findBestInsertionPoint finds the optimal location to insert OTEL initialization code
func (ci *CodeInjector) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, handler LanguageInjector) types.InsertionPoint {
	config := handler.GetConfig()
	// Default to the beginning of the function body
	defaultPoint := types.InsertionPoint{
		LineNumber: bodyNode.StartPoint().Row + 1,
		Column:     bodyNode.StartPoint().Column + 1,
		Priority:   1,
	}

	// Use language-specific insertion query if available
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), handler.GetLanguage())
		if err != nil {
			return defaultPoint
		}
		defer query.Close()

		cursor := sitter.NewQueryCursor()
		defer cursor.Close()

		cursor.Exec(query, bodyNode)

		bestPoint := defaultPoint
		for {
			match, ok := cursor.NextMatch()
			if !ok {
				break
			}

			for _, capture := range match.Captures {
				captureName := query.CaptureNameForId(capture.Index)
				node := capture.Node

				var priority int
				switch captureName {
				case "after_variables":
					priority = 3 // High priority - after variable declarations
				case "before_function_calls":
					priority = 2 // Medium priority - before function calls
				case "function_start":
					priority = 1 // Low priority - start of function
				}

				if priority > bestPoint.Priority {
					bestPoint = types.InsertionPoint{
						LineNumber: node.EndPoint().Row + 1,
						Column:     node.EndPoint().Column + 1,
						Context:    node.Content(content),
						Priority:   priority,
					}
				}
			}
		}

		return bestPoint
	}

	return defaultPoint
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
	if templateStr == "" {
		return ""
	}
	tmpl, err := template.New("snippet").Parse(templateStr)
	if err != nil {
		return templateStr
	}
	var sb strings.Builder
	if err := tmpl.Execute(&sb, data); err != nil {
		return templateStr
	}
	return sb.String()
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

	// Format Go files for syntactic correctness
	if strings.EqualFold(filepath.Ext(filePath), ".go") {
		if formatted, err := gformat.Source([]byte(modifiedContent)); err == nil {
			modifiedContent = string(formatted)
		}
	}

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

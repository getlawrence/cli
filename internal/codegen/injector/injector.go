package injector

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
	"github.com/smacker/go-tree-sitter/python"
)

type CodeInjector struct {
	languages map[string]*sitter.Language
	configs   map[string]*types.LanguageConfig
}

func NewCodeInjector() *CodeInjector {
	injector := &CodeInjector{
		languages: map[string]*sitter.Language{
			"Go":     golang.GetLanguage(),
			"Python": python.GetLanguage(),
		},
		configs: map[string]*types.LanguageConfig{
			"go":     InitializeGoConfig(),
			"python": InitializePythonConfig(),
		},
	}
	return injector
}

func (ci *CodeInjector) InjectOtelInitialization(ctx context.Context,
	entryPoint *domain.EntryPoint,
	operationsData *types.OperationsData,
	req types.GenerationRequest) ([]string, error) {

	config, exists := ci.configs[strings.ToLower(entryPoint.Language)]
	if !exists {
		return nil, fmt.Errorf("unsupported language for modification: %s", entryPoint.Language)
	}

	// Analyze the current file
	analysis, err := ci.analyzeFile(entryPoint.FilePath, entryPoint.Language, config)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze file %s: %w", entryPoint.FilePath, err)
	}

	// Generate modifications
	modifications, err := ci.generateModifications(analysis, operationsData, config, req)
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
func (ci *CodeInjector) analyzeFile(filePath, language string, config *types.LanguageConfig) (*types.FileAnalysis, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lang := ci.languages[language]
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	defer tree.Close()

	analysis := &types.FileAnalysis{
		Language:        language,
		FilePath:        filePath,
		ExistingImports: make(map[string]bool),
		FunctionBodies:  make(map[string]types.InsertionPoint),
	}

	// Analyze imports
	if err := ci.analyzeImports(tree, content, config, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze imports: %w", err)
	}

	// Analyze entry points
	if err := ci.analyzeEntryPoints(tree, content, config, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze entry points: %w", err)
	}

	return analysis, nil
}

// analyzeImports analyzes existing imports and finds import insertion points
func (ci *CodeInjector) analyzeImports(tree *sitter.Tree, content []byte, config *types.LanguageConfig, analysis *types.FileAnalysis) error {
	if importQuery, exists := config.ImportQueries["existing_imports"]; exists {
		query, err := sitter.NewQuery([]byte(importQuery), ci.languages[config.Language])
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

				switch captureName {
				case "import_path":
					importPath := strings.Trim(node.Content(content), `"'`)
					analysis.ExistingImports[importPath] = true
					if strings.Contains(importPath, "opentelemetry") || strings.Contains(importPath, "otel") {
						analysis.HasOTELImports = true
					}
				case "import_location":
					analysis.ImportLocations = append(analysis.ImportLocations, types.InsertionPoint{
						LineNumber: node.EndPoint().Row + 1,
						Column:     node.EndPoint().Column + 1,
						Context:    node.Content(content),
						Priority:   1,
					})
				}
			}
		}
	}

	return nil
}

// analyzeEntryPoints analyzes entry points and function bodies
func (ci *CodeInjector) analyzeEntryPoints(tree *sitter.Tree, content []byte, config *types.LanguageConfig, analysis *types.FileAnalysis) error {
	if functionQuery, exists := config.FunctionQueries["main_function"]; exists {
		query, err := sitter.NewQuery([]byte(functionQuery), ci.languages[config.Language])
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

				switch captureName {
				case "function_name":
					functionName := node.Content(content)
					analysis.EntryPoints = append(analysis.EntryPoints, types.EntryPointInfo{
						Name:       string(functionName),
						LineNumber: node.StartPoint().Row + 1,
						Column:     node.StartPoint().Column + 1,
					})
				case "function_body":
					// Find the best insertion point within the function body
					// bodyStart := node.StartPoint() // Keep for potential future use
					bodyContent := node.Content(content)

					// Check if OTEL setup already exists
					hasOTELSetup := strings.Contains(string(bodyContent), "opentelemetry") ||
						strings.Contains(string(bodyContent), "InitializeOTEL") ||
						strings.Contains(string(bodyContent), "otel")

					insertionPoint := ci.findBestInsertionPoint(node, content, config)

					if len(analysis.EntryPoints) > 0 {
						analysis.EntryPoints[len(analysis.EntryPoints)-1].BodyStart = insertionPoint
						analysis.EntryPoints[len(analysis.EntryPoints)-1].HasOTELSetup = hasOTELSetup
					}
				}
			}
		}
	}

	return nil
}

// findBestInsertionPoint finds the optimal location to insert OTEL initialization code
func (ci *CodeInjector) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	// Default to the beginning of the function body
	defaultPoint := types.InsertionPoint{
		LineNumber: bodyNode.StartPoint().Row + 1,
		Column:     bodyNode.StartPoint().Column + 1,
		Priority:   1,
	}

	// Use language-specific insertion query if available
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), ci.languages[config.Language])
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
	config *types.LanguageConfig,
	req types.GenerationRequest,
) ([]types.CodeModification, error) {
	var modifications []types.CodeModification

	// Generate import modifications
	if !analysis.HasOTELImports && operationsData.InstallOTEL {
		importMods := ci.generateImportModifications(analysis, operationsData, config)
		modifications = append(modifications, importMods...)
	}

	// Generate initialization modifications
	for _, entryPoint := range analysis.EntryPoints {
		if !entryPoint.HasOTELSetup && operationsData.InstallOTEL {
			initMod := ci.generateInitializationModification(entryPoint, operationsData, config, req)
			modifications = append(modifications, initMod)
		}
	}

	return modifications, nil
}

// generateImportModifications creates import-related modifications
func (ci *CodeInjector) generateImportModifications(
	analysis *types.FileAnalysis,
	operationsData *types.OperationsData,
	config *types.LanguageConfig,
) []types.CodeModification {
	var modifications []types.CodeModification

	requiredImports := ci.getRequiredImports(operationsData, config)

	for _, importPath := range requiredImports {
		if !analysis.ExistingImports[importPath] {
			// Find best insertion point for imports
			var insertionPoint types.InsertionPoint
			if len(analysis.ImportLocations) > 0 {
				insertionPoint = analysis.ImportLocations[len(analysis.ImportLocations)-1]
			} else {
				// Default to top of file after package declaration
				insertionPoint = types.InsertionPoint{LineNumber: 3, Column: 1, Priority: 1}
			}

			// Generate import code based on language template
			importCode := ci.formatImport(importPath, config)

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

// getRequiredImports returns the list of imports needed for the given operations
func (ci *CodeInjector) getRequiredImports(operationsData *types.OperationsData, config *types.LanguageConfig) []string {
	var imports []string

	if !operationsData.InstallOTEL {
		return imports
	}

	switch config.Language {
	case "Go":
		imports = append(imports,
			"context",
			"log",
			"go.opentelemetry.io/otel",
			"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
			"go.opentelemetry.io/otel/sdk/trace",
		)

		// Add instrumentation-specific imports
		for _, instr := range operationsData.InstallInstrumentations {
			switch instr {
			case "otelhttp":
				imports = append(imports, "go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp")
			case "otelgin":
				imports = append(imports, "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin")
			}
		}

	case "Python":
		imports = append(imports,
			"opentelemetry.sdk.trace",
			"opentelemetry.exporter.otlp.proto.http.trace_exporter",
		)

		// Add instrumentation-specific imports
		for _, instr := range operationsData.InstallInstrumentations {
			switch instr {
			case "flask":
				imports = append(imports, "opentelemetry.instrumentation.flask")
			case "django":
				imports = append(imports, "opentelemetry.instrumentation.django")
			}
		}
	}

	return imports
}

// formatImport formats an import statement according to the language style
func (ci *CodeInjector) formatImport(importPath string, config *types.LanguageConfig) string {
	switch config.Language {
	case "Go":
		return fmt.Sprintf("import \"%s\"\n", importPath)
	case "Python":
		if strings.Contains(importPath, ".") {
			parts := strings.Split(importPath, ".")
			return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
		}
		return fmt.Sprintf("import %s\n", importPath)
	}
	return fmt.Sprintf(config.ImportTemplate+"\n", importPath)
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

package codegen

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

// GenericCodeModifier provides language-agnostic code modification using Tree-sitter
type GenericCodeModifier struct {
	languages map[string]*sitter.Language
	configs   map[string]*LanguageConfig
}

// LanguageConfig defines how to modify code for a specific language
type LanguageConfig struct {
	Language               string
	FileExtensions         []string
	ImportQueries          map[string]string // Query name -> Tree-sitter query
	FunctionQueries        map[string]string // Query name -> Tree-sitter query
	InsertionQueries       map[string]string // Query name -> Tree-sitter query
	CodeTemplates          map[string]string // Template name -> code template
	ImportTemplate         string            // How to format imports
	InitializationTemplate string            // How to format OTEL initialization
	CleanupTemplate        string            // How to format cleanup code
}

// CodeModification represents a modification to be applied to source code
type CodeModification struct {
	Type         ModificationType
	Language     string
	FilePath     string
	LineNumber   uint32
	Column       uint32
	InsertBefore bool
	InsertAfter  bool
	Content      string
	Context      string // Surrounding code context for validation
}

type ModificationType string

const (
	ModificationAddImport     ModificationType = "add_import"
	ModificationAddInit       ModificationType = "add_initialization"
	ModificationAddCleanup    ModificationType = "add_cleanup"
	ModificationWrapFunction  ModificationType = "wrap_function"
	ModificationAddMiddleware ModificationType = "add_middleware"
)

// NewGenericCodeModifier creates a new generic code modifier
func NewGenericCodeModifier() *GenericCodeModifier {
	modifier := &GenericCodeModifier{
		languages: map[string]*sitter.Language{
			"Go":     golang.GetLanguage(),
			"Python": python.GetLanguage(),
		},
		configs: make(map[string]*LanguageConfig),
	}

	// Initialize language configurations
	modifier.initializeGoConfig()
	modifier.initializePythonConfig()

	return modifier
}

// ModifyEntryPoint modifies an entry point file to add OTEL initialization
func (gcm *GenericCodeModifier) ModifyEntryPoint(
	ctx context.Context,
	entryPoint *types.EntryPoint,
	operationsData *OperationsData,
	req GenerationRequest,
) ([]string, error) {
	config, exists := gcm.configs[entryPoint.Language]
	if !exists {
		return nil, fmt.Errorf("unsupported language for modification: %s", entryPoint.Language)
	}

	// Analyze the current file
	analysis, err := gcm.analyzeFile(entryPoint.FilePath, entryPoint.Language, config)
	if err != nil {
		return nil, fmt.Errorf("failed to analyze file %s: %w", entryPoint.FilePath, err)
	}

	// Generate modifications
	modifications, err := gcm.generateModifications(analysis, operationsData, config, req)
	if err != nil {
		return nil, fmt.Errorf("failed to generate modifications: %w", err)
	}

	// Apply modifications
	if err := gcm.applyModifications(entryPoint.FilePath, modifications, req.Config.DryRun); err != nil {
		return nil, fmt.Errorf("failed to apply modifications: %w", err)
	}

	return []string{entryPoint.FilePath}, nil
}

// FileAnalysis contains the analysis results for a source file
type FileAnalysis struct {
	Language        string
	FilePath        string
	HasOTELImports  bool
	HasOTELSetup    bool
	EntryPoints     []EntryPointInfo
	ImportLocations []InsertionPoint
	FunctionBodies  map[string]InsertionPoint
	ExistingImports map[string]bool
}

// EntryPointInfo contains information about an entry point in the file
type EntryPointInfo struct {
	Name         string
	LineNumber   uint32
	Column       uint32
	BodyStart    InsertionPoint
	BodyEnd      InsertionPoint
	HasOTELSetup bool
}

// InsertionPoint represents a location where code can be inserted
type InsertionPoint struct {
	LineNumber uint32
	Column     uint32
	Context    string
	Priority   int // Higher priority = better insertion point
}

// analyzeFile analyzes a source file to understand its structure
func (gcm *GenericCodeModifier) analyzeFile(filePath, language string, config *LanguageConfig) (*FileAnalysis, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	lang := gcm.languages[language]
	parser := sitter.NewParser()
	parser.SetLanguage(lang)

	tree, err := parser.ParseCtx(context.Background(), nil, content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse file: %w", err)
	}
	defer tree.Close()

	analysis := &FileAnalysis{
		Language:        language,
		FilePath:        filePath,
		ExistingImports: make(map[string]bool),
		FunctionBodies:  make(map[string]InsertionPoint),
	}

	// Analyze imports
	if err := gcm.analyzeImports(tree, content, config, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze imports: %w", err)
	}

	// Analyze entry points
	if err := gcm.analyzeEntryPoints(tree, content, config, analysis); err != nil {
		return nil, fmt.Errorf("failed to analyze entry points: %w", err)
	}

	return analysis, nil
}

// analyzeImports analyzes existing imports and finds import insertion points
func (gcm *GenericCodeModifier) analyzeImports(tree *sitter.Tree, content []byte, config *LanguageConfig, analysis *FileAnalysis) error {
	if importQuery, exists := config.ImportQueries["existing_imports"]; exists {
		query, err := sitter.NewQuery([]byte(importQuery), gcm.languages[config.Language])
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
					analysis.ImportLocations = append(analysis.ImportLocations, InsertionPoint{
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
func (gcm *GenericCodeModifier) analyzeEntryPoints(tree *sitter.Tree, content []byte, config *LanguageConfig, analysis *FileAnalysis) error {
	if functionQuery, exists := config.FunctionQueries["main_function"]; exists {
		query, err := sitter.NewQuery([]byte(functionQuery), gcm.languages[config.Language])
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
					analysis.EntryPoints = append(analysis.EntryPoints, EntryPointInfo{
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

					insertionPoint := gcm.findBestInsertionPoint(node, content, config)

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
func (gcm *GenericCodeModifier) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *LanguageConfig) InsertionPoint {
	// Default to the beginning of the function body
	defaultPoint := InsertionPoint{
		LineNumber: bodyNode.StartPoint().Row + 1,
		Column:     bodyNode.StartPoint().Column + 1,
		Priority:   1,
	}

	// Use language-specific insertion query if available
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), gcm.languages[config.Language])
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
					bestPoint = InsertionPoint{
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
func (gcm *GenericCodeModifier) generateModifications(
	analysis *FileAnalysis,
	operationsData *OperationsData,
	config *LanguageConfig,
	req GenerationRequest,
) ([]CodeModification, error) {
	var modifications []CodeModification

	// Generate import modifications
	if !analysis.HasOTELImports && operationsData.InstallOTEL {
		importMods := gcm.generateImportModifications(analysis, operationsData, config)
		modifications = append(modifications, importMods...)
	}

	// Generate initialization modifications
	for _, entryPoint := range analysis.EntryPoints {
		if !entryPoint.HasOTELSetup && operationsData.InstallOTEL {
			initMod := gcm.generateInitializationModification(entryPoint, operationsData, config, req)
			modifications = append(modifications, initMod)
		}
	}

	return modifications, nil
}

// generateImportModifications creates import-related modifications
func (gcm *GenericCodeModifier) generateImportModifications(
	analysis *FileAnalysis,
	operationsData *OperationsData,
	config *LanguageConfig,
) []CodeModification {
	var modifications []CodeModification

	requiredImports := gcm.getRequiredImports(operationsData, config)

	for _, importPath := range requiredImports {
		if !analysis.ExistingImports[importPath] {
			// Find best insertion point for imports
			var insertionPoint InsertionPoint
			if len(analysis.ImportLocations) > 0 {
				insertionPoint = analysis.ImportLocations[len(analysis.ImportLocations)-1]
			} else {
				// Default to top of file after package declaration
				insertionPoint = InsertionPoint{LineNumber: 3, Column: 1, Priority: 1}
			}

			// Generate import code based on language template
			importCode := gcm.formatImport(importPath, config)

			modifications = append(modifications, CodeModification{
				Type:        ModificationAddImport,
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
func (gcm *GenericCodeModifier) generateInitializationModification(
	entryPoint EntryPointInfo,
	operationsData *OperationsData,
	config *LanguageConfig,
	req GenerationRequest,
) CodeModification {
	// Generate initialization code from template
	templateData := map[string]interface{}{
		"ServiceName":       filepath.Base(req.CodebasePath),
		"Instrumentations":  operationsData.InstallInstrumentations,
		"InstallComponents": operationsData.InstallComponents,
	}

	initCode := gcm.generateFromTemplate(config.InitializationTemplate, templateData)

	return CodeModification{
		Type:         ModificationAddInit,
		Language:     config.Language,
		LineNumber:   entryPoint.BodyStart.LineNumber,
		Column:       entryPoint.BodyStart.Column,
		InsertBefore: false,
		InsertAfter:  true,
		Content:      initCode,
	}
}

// initializeGoConfig sets up the Go language configuration
func (gcm *GenericCodeModifier) initializeGoConfig() {
	gcm.configs["Go"] = &LanguageConfig{
		Language:       "Go",
		FileExtensions: []string{".go"},
		ImportQueries: map[string]string{
			"existing_imports": `
				(import_declaration 
					(import_spec 
						path: (interpreted_string_literal) @import_path
					)
				) @import_location
			`,
		},
		FunctionQueries: map[string]string{
			"main_function": `
				(function_declaration 
					name: (identifier) @function_name
					body: (block) @function_body
					(#eq? @function_name "main")
				)
			`,
		},
		InsertionQueries: map[string]string{
			"optimal_insertion": `
				(block
					(var_declaration) @after_variables
				)
				(block
					(call_expression) @before_function_calls
				)
				(block) @function_start
			`,
		},
		ImportTemplate: `import "%s"`,
		InitializationTemplate: `
	// Initialize OpenTelemetry
	tp, err := initializeOTEL()
	if err != nil {
		log.Fatal("Failed to initialize OpenTelemetry:", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %%v", err)
		}
	}()
`,
		CleanupTemplate: `defer tp.Shutdown(context.Background())`,
	}
}

// initializePythonConfig sets up the Python language configuration
func (gcm *GenericCodeModifier) initializePythonConfig() {
	gcm.configs["Python"] = &LanguageConfig{
		Language:       "Python",
		FileExtensions: []string{".py", ".pyw"},
		ImportQueries: map[string]string{
			"existing_imports": `
				(import_statement 
					name: (dotted_name) @import_path
				) @import_location
				(import_from_statement
					module_name: (dotted_name) @import_path
				) @import_location
			`,
		},
		FunctionQueries: map[string]string{
			"main_function": `
				(function_definition 
					name: (identifier) @function_name
					body: (block) @function_body
					(#eq? @function_name "main")
				)
				(if_statement
					condition: (comparison_operator
						left: (identifier) @name_var
						right: (string) @main_str
					)
					body: (block) @function_body
					(#eq? @name_var "__name__")
					(#match? @main_str ".*__main__.*")
				)
			`,
		},
		InsertionQueries: map[string]string{
			"optimal_insertion": `
				(block
					(assignment) @after_variables
				)
				(block
					(expression_statement 
						(call)) @before_function_calls
				)
				(block) @function_start
			`,
		},
		ImportTemplate: `from opentelemetry import %s`,
		InitializationTemplate: `
    # Initialize OpenTelemetry
    tp = initialize_otel()
    import atexit
    atexit.register(lambda: tp.shutdown())
`,
		CleanupTemplate: `tp.shutdown()`,
	}
}

// getRequiredImports returns the list of imports needed for the given operations
func (gcm *GenericCodeModifier) getRequiredImports(operationsData *OperationsData, config *LanguageConfig) []string {
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
func (gcm *GenericCodeModifier) formatImport(importPath string, config *LanguageConfig) string {
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
func (gcm *GenericCodeModifier) generateFromTemplate(templateStr string, data map[string]interface{}) string {
	// For now, return the template as-is. In a full implementation,
	// you would use the text/template package to process the template
	return templateStr
}

// applyModifications applies the generated modifications to the source file
func (gcm *GenericCodeModifier) applyModifications(filePath string, modifications []CodeModification, dryRun bool) error {
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

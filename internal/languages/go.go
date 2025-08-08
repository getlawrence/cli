package languages

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/golang"
)

type GoPlugin struct {
	config *types.LanguageConfig
}

func NewGoPlugin() *GoPlugin {
	return &GoPlugin{
		config: &types.LanguageConfig{
			Language:       "Go",
			FileExtensions: []string{".go"},
			ImportQueries: map[string]string{
				"existing_imports": `
 (import_declaration
   (import_spec
     path: (interpreted_string_literal) @import_path
   ) @import_spec
 ) @import_declaration

 (import_declaration
   (import_spec
     path: (raw_string_literal) @import_path
   ) @import_spec
 ) @import_declaration
`,
			},
			FunctionQueries: map[string]string{
				"main_function": `
(function_declaration
  name: (identifier) @function_name
  body: (block) @function_body
  (#eq? @function_name "main"))

(function_declaration
  name: (identifier) @function_name
  body: (block) @init_function
  (#eq? @function_name "init"))
`,
			},
			InsertionQueries: map[string]string{
				"optimal_insertion": `
(block
  (var_declaration) @after_variables)

(block
  (short_var_declaration) @after_variables)

(block
  (assignment_statement) @after_variables)

(block
  (expression_statement 
    (call_expression)) @before_function_calls)

(block) @function_start
`,
			},
			ImportTemplate: `"go.opentelemetry.io/%s"`,
			InitializationTemplate: `
	// Initialize OpenTelemetry
	tp, err := SetupOTEL()
	if err != nil {
		log.Fatalf("Failed to initialize OTEL: %v", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Failed to shutdown tracer provider: %v", err)
		}
	}()
`,
			CleanupTemplate: `tp.Shutdown(context.Background())`,
		},
	}
}

func (p *GoPlugin) ID() string {
	return "go"
}
func (p *GoPlugin) DisplayName() string {
	return "Go"
}
func (p *GoPlugin) EntryPointTreeSitterLanguage() *sitter.Language {
	return golang.GetLanguage()
}
func (p *GoPlugin) EntrypointQuery() string {
	return `
                (function_declaration 
                    name: (identifier) @func_name
                    (#eq? @func_name "main")
                ) @main_function
            `
}
func (p *GoPlugin) FileExtensions() []string {
	return []string{".go"}
}

// Plugin acts as injector
func (p *GoPlugin) Injector() inj.LanguageInjector {
	return p
}
func (p *GoPlugin) Dependencies() dep.DependencyHandler {
	return dep.NewGoHandler()
}

func (p *GoPlugin) SupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{templates.CodeInstrumentation, templates.AutoInstrumentation}
}
func (p *GoPlugin) OutputFilename(m templates.InstallationMethod) string {
	switch m {
	case templates.CodeInstrumentation:
		return "otel.go"
	case templates.AutoInstrumentation:
		return "otel_auto.go"
	default:
		return "otel.go"
	}
}

// Injector methods
func (p *GoPlugin) GetTreeSitterLanguage() *sitter.Language { return golang.GetLanguage() }
func (p *GoPlugin) GetLanguage() *sitter.Language           { return golang.GetLanguage() }
func (p *GoPlugin) GetConfig() *types.LanguageConfig        { return p.config }
func (p *GoPlugin) GetRequiredImports() []string {
	return []string{
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/trace",
		"go.opentelemetry.io/otel/sdk/trace",
		"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		"context",
		"log",
	}
}
func (p *GoPlugin) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}
	var result strings.Builder
	if hasExistingImports {
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%q\n", importPath))
		}
	} else {
		result.WriteString("import (\n")
		for _, importPath := range imports {
			result.WriteString(fmt.Sprintf("\t%q\n", importPath))
		}
		result.WriteString(")\n")
	}
	return result.String()
}
func (p *GoPlugin) FormatSingleImport(importPath string) string { return fmt.Sprintf("%q", importPath) }
func (p *GoPlugin) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		importPath := strings.Trim(node.Content(content), "\"")
		analysis.ExistingImports[importPath] = true
		if strings.Contains(importPath, "go.opentelemetry.io") {
			analysis.HasOTELImports = true
		}
	case "import_spec":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: 3}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	case "import_declaration":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: 0, Context: node.Content(content), Priority: 2}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}
func (p *GoPlugin) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "function_body":
		insertionPoint := p.findBestInsertionPoint(node, content, config)
		hasOTELSetup := p.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "main",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasOTELSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	case "init_function":
		insertionPoint := p.findBestInsertionPoint(node, content, config)
		hasOTELSetup := p.detectExistingOTELSetup(node, content)
		entryPoint := types.EntryPointInfo{
			Name:         "init",
			LineNumber:   node.StartPoint().Row + 1,
			Column:       node.StartPoint().Column + 1,
			BodyStart:    insertionPoint,
			BodyEnd:      types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1},
			HasOTELSetup: hasOTELSetup,
		}
		analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
	}
}
func (p *GoPlugin) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "function_start":
		return 100
	case "after_variables":
		return 3
	case "after_imports":
		return 2
	case "before_function_calls":
		return 1
	default:
		return 1
	}
}
func (p *GoPlugin) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: bodyNode.StartPoint().Row + 1, Column: bodyNode.StartPoint().Column + 1, Priority: 1}
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), p.GetTreeSitterLanguage())
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
				priority := p.GetInsertionPointPriority(captureName)
				if priority > bestPoint.Priority {
					if captureName == "function_start" {
						bestPoint = types.InsertionPoint{LineNumber: node.StartPoint().Row + 1, Column: node.StartPoint().Column + 1, Context: node.Content(content), Priority: priority}
					} else {
						bestPoint = types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: priority}
					}
				}
			}
		}
		return bestPoint
	}
	return defaultPoint
}
func (p *GoPlugin) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "trace.NewTracerProvider") ||
		strings.Contains(bodyContent, "otel.SetTracerProvider") ||
		strings.Contains(bodyContent, "SetupOTEL") ||
		strings.Contains(bodyContent, "setupTracing")
}

// Implement required method to satisfy injector.LanguageInjector
func (p *GoPlugin) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// DetectionProvider implementation (replaces legacy GoDetector)
func (p *GoPlugin) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// Parse go.mod
	goModPath := filepath.Join(rootPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		libs, err := p.parseGoMod(goModPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// Scan .go files
	goFiles, err := p.findGoFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range goFiles {
		libs, err := p.parseGoImports(file)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	return p.deduplicateLibraries(libraries), nil
}

func (p *GoPlugin) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	goModPath := filepath.Join(rootPath, "go.mod")
	if _, err := os.Stat(goModPath); err == nil {
		pkgs, err := p.parseAllDependencies(goModPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	goFiles, err := p.findGoFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, file := range goFiles {
		pkgs, err := p.parseAllImports(file)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	return p.deduplicatePackages(packages), nil
}

// Helpers (ported from legacy detector)
func (p *GoPlugin) parseGoMod(goModPath string) ([]domain.Library, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	otelRegex := regexp.MustCompile(`^\s*(go\.opentelemetry\.io/[^\s]+)\s+([^\s]+)`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		matches := otelRegex.FindStringSubmatch(line)
		if len(matches) >= 3 {
			libraries = append(libraries, domain.Library{
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

func (p *GoPlugin) parseGoImports(filePath string) ([]domain.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	inImportBlock := false
	otelImportRegex := regexp.MustCompile(`"(go\.opentelemetry\.io/[^"]+)"`)
	singleImportRegex := regexp.MustCompile(`import\s+"(go\.opentelemetry\.io/[^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := singleImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			libraries = append(libraries, domain.Library{Name: matches[1], Language: "go", ImportPath: matches[1]})
			continue
		}
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
				libraries = append(libraries, domain.Library{Name: matches[1], Language: "go", ImportPath: matches[1]})
			}
		}
	}
	return libraries, scanner.Err()
}

func (p *GoPlugin) findGoFiles(rootPath string) ([]string, error) {
	var goFiles []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
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

func (p *GoPlugin) deduplicateLibraries(libraries []domain.Library) []domain.Library {
	seen := make(map[string]bool)
	var result []domain.Library
	for _, lib := range libraries {
		key := fmt.Sprintf("%s:%s", lib.Name, lib.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, lib)
		}
	}
	return result
}

func (p *GoPlugin) parseAllDependencies(goModPath string) ([]domain.Package, error) {
	file, err := os.Open(goModPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	depRegex := regexp.MustCompile(`^\s*([^\s]+)\s+([^\s]+)`)
	inRequireBlock := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "require (") {
			inRequireBlock = true
			continue
		}
		if inRequireBlock && line == ")" {
			inRequireBlock = false
			continue
		}
		line = strings.TrimPrefix(line, "require ")
		if inRequireBlock || strings.HasPrefix(scanner.Text(), "require ") {
			matches := depRegex.FindStringSubmatch(line)
			if len(matches) >= 3 {
				packageName := matches[1]
				if !strings.HasPrefix(packageName, "golang.org/x/") && !strings.Contains(packageName, ".") {
					continue
				}
				packages = append(packages, domain.Package{
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

func (p *GoPlugin) parseAllImports(filePath string) ([]domain.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	inImportBlock := false
	importRegex := regexp.MustCompile(`"([^"]+)"`)
	singleImportRegex := regexp.MustCompile(`import\s+"([^"]+)"`)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := singleImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			packageName := matches[1]
			if p.isThirdPartyPackage(packageName) {
				packages = append(packages, domain.Package{Name: packageName, Language: "go", ImportPath: packageName})
			}
			continue
		}
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
				if p.isThirdPartyPackage(packageName) {
					packages = append(packages, domain.Package{Name: packageName, Language: "go", ImportPath: packageName})
				}
			}
		}
	}
	return packages, scanner.Err()
}

func (p *GoPlugin) isThirdPartyPackage(packageName string) bool {
	if !strings.Contains(packageName, ".") {
		return false
	}
	thirdPartyPrefixes := []string{"github.com/", "gitlab.com/", "go.uber.org/", "google.golang.org/", "gopkg.in/", "go.opentelemetry.io/"}
	for _, prefix := range thirdPartyPrefixes {
		if strings.HasPrefix(packageName, prefix) {
			return true
		}
	}
	return strings.Contains(packageName, ".")
}

func (p *GoPlugin) deduplicatePackages(packages []domain.Package) []domain.Package {
	seen := make(map[string]bool)
	var result []domain.Package
	for _, pkg := range packages {
		key := fmt.Sprintf("%s:%s", pkg.Name, pkg.Version)
		if !seen[key] {
			seen[key] = true
			result = append(result, pkg)
		}
	}
	return result
}

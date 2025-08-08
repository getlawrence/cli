package languages

import (
	"bufio"
	"context"
	"fmt"
	"strings"

	"os"
	"path/filepath"
	"regexp"

	dep "github.com/getlawrence/cli/internal/codegen/dependency"
	inj "github.com/getlawrence/cli/internal/codegen/injector"
	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/templates"
	sitter "github.com/smacker/go-tree-sitter"
	tspython "github.com/smacker/go-tree-sitter/python"
)

// PythonPlugin is a single type implementing both the LanguagePlugin API and the injector
type PythonPlugin struct {
	config *types.LanguageConfig
}

func NewPythonPlugin() *PythonPlugin {
	return &PythonPlugin{
		config: &types.LanguageConfig{
			Language:       "Python",
			FileExtensions: []string{".py", ".pyw"},
			ImportQueries: map[string]string{
				"existing_imports": `
 (import_statement 
     name: (dotted_name) @import_path
 ) @import_location

 (import_from_statement
     module: (dotted_name) @import_path
 ) @import_location
`,
			},
			FunctionQueries: map[string]string{
				"main_function": `
 (if_statement
     condition: (binary_operator
         left: (identifier) @name_var
         right: (string) @main_str
     )
     (#eq? @name_var "__name__")
     (#match? @main_str ".*__main__.*")
 ) @main_if_block
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
	from otel import init_tracer
	tracer_provider = init_tracer()
`,
			CleanupTemplate: `tp.shutdown()`,
		},
	}
}

// LanguagePlugin core
func (p *PythonPlugin) ID() string                                     { return "python" }
func (p *PythonPlugin) DisplayName() string                            { return "Python" }
func (p *PythonPlugin) EntryPointTreeSitterLanguage() *sitter.Language { return tspython.GetLanguage() }
func (p *PythonPlugin) EntrypointQuery() string {
	return `
                (if_statement
                    condition: (binary_operator
                        left: (identifier) @name_var
                        right: (string) @main_str
                    )
                    (#eq? @name_var "__name__")
                    (#match? @main_str ".*__main__.*")
                ) @main_if_block
            `
}
func (p *PythonPlugin) FileExtensions() []string { return []string{".py", ".pyw"} }

// Provide injector and dependencies
func (p *PythonPlugin) Injector() inj.LanguageInjector      { return p } // PythonPlugin itself implements LanguageInjector
func (p *PythonPlugin) Dependencies() dep.DependencyHandler { return dep.NewPythonHandler() }

// Template support
func (p *PythonPlugin) SupportedMethods() []templates.InstallationMethod {
	return []templates.InstallationMethod{templates.CodeInstrumentation, templates.AutoInstrumentation}
}
func (p *PythonPlugin) OutputFilename(m templates.InstallationMethod) string {
	switch m {
	case templates.CodeInstrumentation:
		return "otel.py"
	case templates.AutoInstrumentation:
		return "otel_auto.py"
	default:
		return "otel.py"
	}
}

// Injector implementation (methods from injector.LanguageInjector)
func (p *PythonPlugin) GetTreeSitterLanguage() *sitter.Language { return tspython.GetLanguage() }
func (p *PythonPlugin) GetLanguage() *sitter.Language           { return tspython.GetLanguage() }
func (p *PythonPlugin) GetConfig() *types.LanguageConfig        { return p.config }
func (p *PythonPlugin) GetRequiredImports() []string {
	return []string{
		"opentelemetry.sdk.trace",
		"opentelemetry.exporter.otlp.proto.http.trace_exporter",
	}
}
func (p *PythonPlugin) FormatImports(imports []string, hasExistingImports bool) string {
	if len(imports) == 0 {
		return ""
	}
	var result strings.Builder
	for _, importPath := range imports {
		result.WriteString(p.FormatSingleImport(importPath))
	}
	return result.String()
}
func (p *PythonPlugin) FormatSingleImport(importPath string) string {
	if strings.Contains(importPath, ".") {
		parts := strings.Split(importPath, ".")
		return fmt.Sprintf("from %s import %s\n", strings.Join(parts[:len(parts)-1], "."), parts[len(parts)-1])
	}
	return fmt.Sprintf("import %s\n", importPath)
}
func (p *PythonPlugin) AnalyzeImportCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis) {
	switch captureName {
	case "import_path":
		importPath := node.Content(content)
		analysis.ExistingImports[importPath] = true
		if strings.Contains(importPath, "opentelemetry") {
			analysis.HasOTELImports = true
		}
	case "import_location":
		insertionPoint := types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: 2}
		analysis.ImportLocations = append(analysis.ImportLocations, insertionPoint)
	}
}
func (p *PythonPlugin) AnalyzeFunctionCapture(captureName string, node *sitter.Node, content []byte, analysis *types.FileAnalysis, config *types.LanguageConfig) {
	switch captureName {
	case "main_if_block":
		var bodyNode *sitter.Node
		for i := uint32(0); i < node.ChildCount(); i++ {
			child := node.Child(int(i))
			if child.Type() == "block" {
				bodyNode = child
				break
			}
		}
		if bodyNode != nil {
			insertionPoint := p.findBestInsertionPoint(bodyNode, content, config)
			hasOTELSetup := p.detectExistingOTELSetup(bodyNode, content)
			entryPoint := types.EntryPointInfo{
				Name:         "if __name__ == '__main__'",
				LineNumber:   bodyNode.StartPoint().Row + 1,
				Column:       bodyNode.StartPoint().Column + 1,
				BodyStart:    insertionPoint,
				BodyEnd:      types.InsertionPoint{LineNumber: bodyNode.EndPoint().Row + 1, Column: bodyNode.EndPoint().Column + 1},
				HasOTELSetup: hasOTELSetup,
			}
			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
		}
	}
	p.findMainBlockWithRegex(content, analysis)
}
func (p *PythonPlugin) GetInsertionPointPriority(captureName string) int {
	switch captureName {
	case "after_variables":
		return 3
	case "before_function_calls":
		return 2
	case "function_start":
		return 1
	default:
		return 1
	}
}
func (p *PythonPlugin) findBestInsertionPoint(bodyNode *sitter.Node, content []byte, config *types.LanguageConfig) types.InsertionPoint {
	defaultPoint := types.InsertionPoint{LineNumber: bodyNode.StartPoint().Row + 1, Column: bodyNode.StartPoint().Column + 1, Priority: 1}
	if insertQuery, exists := config.InsertionQueries["optimal_insertion"]; exists {
		query, err := sitter.NewQuery([]byte(insertQuery), p.GetLanguage())
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
					bestPoint = types.InsertionPoint{LineNumber: node.EndPoint().Row + 1, Column: node.EndPoint().Column + 1, Context: node.Content(content), Priority: priority}
				}
			}
		}
		return bestPoint
	}
	return defaultPoint
}
func (p *PythonPlugin) detectExistingOTELSetup(bodyNode *sitter.Node, content []byte) bool {
	bodyContent := bodyNode.Content(content)
	return strings.Contains(bodyContent, "initialize_otel") || strings.Contains(bodyContent, "TracerProvider") || strings.Contains(bodyContent, "set_tracer_provider")
}
func (p *PythonPlugin) findMainBlockWithRegex(content []byte, analysis *types.FileAnalysis) {
	lines := strings.Split(string(content), "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.Contains(trimmed, `if __name__ == '__main__'`) || strings.Contains(trimmed, `if __name__ == "__main__"`) {
			insertionPoint := types.InsertionPoint{LineNumber: uint32(i + 2), Column: 1, Priority: 3}
			entryPoint := types.EntryPointInfo{
				Name:         "if __name__ == '__main__'",
				LineNumber:   uint32(i + 1),
				Column:       1,
				BodyStart:    insertionPoint,
				BodyEnd:      types.InsertionPoint{LineNumber: uint32(len(lines)), Column: 1},
				HasOTELSetup: false,
			}
			analysis.EntryPoints = append(analysis.EntryPoints, entryPoint)
			break
		}
	}
}
func (p *PythonPlugin) FallbackAnalyzeImports(content []byte, analysis *types.FileAnalysis) {}

// DetectionProvider implementation (replaces legacy PythonDetector)
func (p *PythonPlugin) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	var libraries []domain.Library

	// requirements.txt
	reqPath := filepath.Join(rootPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		libs, err := p.parseRequirements(reqPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// pyproject.toml
	pyprojectPath := filepath.Join(rootPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		libs, err := p.parsePyproject(pyprojectPath)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}

	// imports
	pyFiles, err := p.findPythonFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, f := range pyFiles {
		libs, err := p.parsePythonImports(f)
		if err != nil {
			return nil, err
		}
		libraries = append(libraries, libs...)
	}
	return p.deduplicateLibraries(libraries), nil
}

func (p *PythonPlugin) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	var packages []domain.Package

	reqPath := filepath.Join(rootPath, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		pkgs, err := p.parseAllRequirements(reqPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	pyprojectPath := filepath.Join(rootPath, "pyproject.toml")
	if _, err := os.Stat(pyprojectPath); err == nil {
		pkgs, err := p.parseAllPyproject(pyprojectPath)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}

	pyFiles, err := p.findPythonFiles(rootPath)
	if err != nil {
		return nil, err
	}
	for _, f := range pyFiles {
		pkgs, err := p.parseAllPythonImports(f)
		if err != nil {
			return nil, err
		}
		packages = append(packages, pkgs...)
	}
	return p.deduplicatePackages(packages), nil
}

// Helpers (ported from legacy detector)
func (p *PythonPlugin) parseRequirements(reqPath string) ([]domain.Library, error) {
	file, err := os.Open(reqPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	otelRegex := regexp.MustCompile(`^(opentelemetry[a-zA-Z0-9\-_]*)(==|>=|<=|>|<|~=)([^\s;#]+)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "#") || line == "" {
			continue
		}
		if matches := otelRegex.FindStringSubmatch(line); len(matches) >= 4 {
			libraries = append(libraries, domain.Library{
				Name: matches[1], Version: matches[3], Language: "python", ImportPath: matches[1], PackageFile: reqPath,
			})
		}
	}
	return libraries, scanner.Err()
}

func (p *PythonPlugin) parsePyproject(pyprojectPath string) ([]domain.Library, error) {
	file, err := os.Open(pyprojectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	inDependencies := false
	otelRegex := regexp.MustCompile(`"(opentelemetry[a-zA-Z0-9\-_]*)[^"]*"`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, "dependencies") && strings.Contains(line, "=") {
			inDependencies = true
			continue
		}
		if inDependencies && strings.HasPrefix(line, "[") {
			inDependencies = false
			continue
		}
		if inDependencies {
			if matches := otelRegex.FindStringSubmatch(line); len(matches) >= 2 {
				libraries = append(libraries, domain.Library{Name: matches[1], Language: "python", ImportPath: matches[1], PackageFile: pyprojectPath})
			}
		}
	}
	return libraries, scanner.Err()
}

func (p *PythonPlugin) findPythonFiles(rootPath string) ([]string, error) {
	var pyFiles []string
	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() && (info.Name() == "__pycache__" || info.Name() == ".git" || info.Name() == "venv" || info.Name() == ".venv") {
			return filepath.SkipDir
		}
		if strings.HasSuffix(path, ".py") {
			pyFiles = append(pyFiles, path)
		}
		return nil
	})
	return pyFiles, err
}

func (p *PythonPlugin) parsePythonImports(filePath string) ([]domain.Library, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var libraries []domain.Library
	scanner := bufio.NewScanner(file)
	importRegex := regexp.MustCompile(`^(?:from\s+)?(opentelemetry[a-zA-Z0-9\._]*)\s*(?:import|$)`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if matches := importRegex.FindStringSubmatch(line); len(matches) >= 2 {
			libraries = append(libraries, domain.Library{Name: matches[1], Language: "python", ImportPath: matches[1]})
		}
	}
	return libraries, scanner.Err()
}

func (p *PythonPlugin) parseAllRequirements(reqPath string) ([]domain.Package, error) {
	file, err := os.Open(reqPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	packageRegex := regexp.MustCompile(`^([a-zA-Z0-9\-\_\.]+)(?:[>=<!\s]+([^\s#]+))?`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		matches := packageRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			version := ""
			if len(matches) >= 3 {
				version = matches[2]
			}
			packages = append(packages, domain.Package{Name: matches[1], Version: version, Language: "python", ImportPath: matches[1], PackageFile: reqPath})
		}
	}
	return packages, scanner.Err()
}

func (p *PythonPlugin) parseAllPyproject(pyprojectPath string) ([]domain.Package, error) {
	file, err := os.Open(pyprojectPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	inDependencies := false
	depRegex := regexp.MustCompile(`^\s*"([^"]+)"\s*=`)
	depArrayRegex := regexp.MustCompile(`^\s*"([^"]+)"`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[tool.poetry.dependencies]" || line == "[project]" {
			inDependencies = true
			continue
		}
		if strings.HasPrefix(line, "[") && inDependencies {
			inDependencies = false
			continue
		}
		if inDependencies {
			if matches := depRegex.FindStringSubmatch(line); len(matches) >= 2 {
				packages = append(packages, domain.Package{Name: matches[1], Language: "python", ImportPath: matches[1], PackageFile: pyprojectPath})
			} else if matches := depArrayRegex.FindStringSubmatch(line); len(matches) >= 2 {
				packages = append(packages, domain.Package{Name: matches[1], Language: "python", ImportPath: matches[1], PackageFile: pyprojectPath})
			}
		}
	}
	return packages, scanner.Err()
}

func (p *PythonPlugin) parseAllPythonImports(filePath string) ([]domain.Package, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var packages []domain.Package
	scanner := bufio.NewScanner(file)
	fromImportRegex := regexp.MustCompile(`^from\s+([a-zA-Z0-9\._]+)\s+import`)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		var packageName string
		if matches := fromImportRegex.FindStringSubmatch(line); len(matches) >= 2 {
			packageName = matches[1]
		} else if strings.HasPrefix(line, "import ") {
			importLine := strings.TrimPrefix(line, "import ")
			parts := strings.Split(importLine, ",")
			if len(parts) > 0 {
				packageName = strings.TrimSpace(strings.Split(parts[0], " as ")[0])
			}
		}
		if packageName != "" && p.isThirdPartyPythonPackage(packageName) {
			rootPackage := strings.Split(packageName, ".")[0]
			packages = append(packages, domain.Package{Name: rootPackage, Language: "python", ImportPath: packageName})
		}
	}
	return packages, scanner.Err()
}

// Helpers ported from legacy detector
func (p *PythonPlugin) deduplicateLibraries(libraries []domain.Library) []domain.Library {
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

func (p *PythonPlugin) deduplicatePackages(packages []domain.Package) []domain.Package {
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

func (p *PythonPlugin) isThirdPartyPythonPackage(packageName string) bool {
	standardLibrary := []string{
		"os", "sys", "json", "re", "time", "datetime", "math", "random",
		"collections", "itertools", "functools", "operator", "copy",
		"pickle", "sqlite3", "threading", "multiprocessing", "subprocess",
		"urllib", "http", "email", "html", "xml", "logging", "unittest",
		"argparse", "configparser", "pathlib", "io", "csv", "base64",
		"hashlib", "hmac", "secrets", "uuid", "typing", "dataclasses",
		"enum", "contextlib", "warnings", "traceback", "__future__",
	}
	rootPackage := strings.Split(packageName, ".")[0]
	for _, stdLib := range standardLibrary {
		if rootPackage == stdLib {
			return false
		}
	}
	return !strings.HasPrefix(packageName, ".")
}

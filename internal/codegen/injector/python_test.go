package injector

import (
	"testing"

	"github.com/getlawrence/cli/internal/codegen/types"
)

func TestPythonInjector_FlaskDetection(t *testing.T) {
	injector := NewPythonInjector()

	// Test Flask detection
	flaskContent := []byte(`
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run()
`)

	if !injector.detectFlaskUsage(flaskContent) {
		t.Error("Expected Flask usage to be detected")
	}

	// Test non-Flask content
	nonFlaskContent := []byte(`
import requests

def fetch_data():
    response = requests.get('https://api.example.com/data')
    return response.json()
`)

	if injector.detectFlaskUsage(nonFlaskContent) {
		t.Error("Expected Flask usage to not be detected")
	}
}

func TestPythonInjector_FlaskInstrumentation(t *testing.T) {
	injector := NewPythonInjector()

	operationsData := &types.OperationsData{
		InstallOTEL:             true,
		InstallInstrumentations: []string{"flask"},
		InstallComponents:       make(map[string][]string),
	}

	// Test Flask instrumentation generation
	flaskContent := []byte(`
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run()
`)

	modifications := injector.GenerateFrameworkModifications(flaskContent, operationsData)
	if len(modifications) != 2 {
		t.Errorf("Expected 2 modifications (import + framework), got %d", len(modifications))
	}

	// Find the framework modification
	var frameworkMod *types.CodeModification
	for i := range modifications {
		if modifications[i].Type == types.ModificationAddFramework {
			frameworkMod = &modifications[i]
			break
		}
	}

	if frameworkMod == nil {
		t.Fatal("Expected to find ModificationAddFramework modification")
	}

	if frameworkMod.Framework != "flask" {
		t.Errorf("Expected framework 'flask', got %s", frameworkMod.Framework)
	}

	// Verify the content contains Flask instrumentation
	expectedContent := "from opentelemetry.instrumentation.flask import FlaskInstrumentor"
	if !contains(frameworkMod.Content, expectedContent) {
		t.Errorf("Expected content to contain '%s', got: %s", expectedContent, frameworkMod.Content)
	}
}

func TestPythonInjector_NoFlaskInstrumentation(t *testing.T) {
	injector := NewPythonInjector()

	operationsData := &types.OperationsData{
		InstallOTEL:             true,
		InstallInstrumentations: []string{"requests"}, // Not Flask
		InstallComponents:       make(map[string][]string),
	}

	// Test that no Flask instrumentation is generated when not requested
	flaskContent := []byte(`
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run()
`)

	modifications := injector.GenerateFrameworkModifications(flaskContent, operationsData)
	if len(modifications) != 1 {
		t.Errorf("Expected 1 modification (import only) when Flask instrumentation not requested, got %d", len(modifications))
	}

	// Should only have import modification, no framework modification
	for _, mod := range modifications {
		if mod.Type == types.ModificationAddFramework {
			t.Error("Expected no framework modification when Flask instrumentation not requested")
		}
	}
}

func TestPythonInjector_FlaskInstrumentationPoint(t *testing.T) {
	injector := NewPythonInjector()

	// Test finding the best place to inject Flask instrumentation
	flaskContent := []byte(`
from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run()
`)

	insertionPoint := injector.findFlaskInstrumentationPoint(flaskContent)
	if insertionPoint.LineNumber != 5 { // After app = Flask(__name__)
		t.Errorf("Expected insertion at line 5, got %d", insertionPoint.LineNumber)
	}

	if insertionPoint.Priority != 5 { // High priority for framework instrumentation
		t.Errorf("Expected priority 5, got %d", insertionPoint.Priority)
	}
}

func TestPythonInjector_FormatFrameworkImports(t *testing.T) {
	injector := NewPythonInjector()

	imports := []string{"opentelemetry.instrumentation.flask"}
	formatted := injector.FormatFrameworkImports(imports)

	expected := "from opentelemetry.instrumentation.flask import FlaskInstrumentor\n"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}
}

func TestPythonInjector_CompleteFlaskInstrumentation(t *testing.T) {
	injector := NewPythonInjector()

	// Test the complete flow: input Python file -> expected output
	inputContent := `from flask import Flask

app = Flask(__name__)

@app.route('/')
def hello():
    return 'Hello, World!'

if __name__ == '__main__':
    app.run()
`

	operationsData := &types.OperationsData{
		InstallOTEL:             true,
		InstallInstrumentations: []string{"flask"},
		InstallComponents:       make(map[string][]string),
	}

	// Generate modifications
	modifications := injector.GenerateFrameworkModifications([]byte(inputContent), operationsData)

	// Should have exactly 2 modifications (import + Flask instrumentation)
	if len(modifications) != 2 {
		t.Fatalf("Expected 2 modifications (import + framework), got %d", len(modifications))
	}

	// Find the framework modification
	var frameworkMod *types.CodeModification
	for i := range modifications {
		if modifications[i].Type == types.ModificationAddFramework {
			frameworkMod = &modifications[i]
			break
		}
	}

	if frameworkMod == nil {
		t.Fatal("Expected to find ModificationAddFramework modification")
	}

	if frameworkMod.Framework != "flask" {
		t.Errorf("Expected framework 'flask', got %s", frameworkMod.Framework)
	}

	// Verify the content contains the correct Flask instrumentation
	expectedInstrumentation := "from opentelemetry.instrumentation.flask import FlaskInstrumentor"
	if !contains(frameworkMod.Content, expectedInstrumentation) {
		t.Errorf("Expected content to contain '%s', got: %s", expectedInstrumentation, frameworkMod.Content)
	}

	expectedInstrumentationCall := "FlaskInstrumentor().instrument_app(app)"
	if !contains(frameworkMod.Content, expectedInstrumentationCall) {
		t.Errorf("Expected content to contain '%s', got: %s", expectedInstrumentationCall, frameworkMod.Content)
	}

	// Verify insertion point is correct (after Flask app creation)
	if frameworkMod.LineNumber != 4 { // After app = Flask(__name__) at line 3
		t.Errorf("Expected insertion at line 4, got %d", frameworkMod.LineNumber)
	}
}

func TestPythonInjector_RequiredImports(t *testing.T) {
	injector := NewPythonInjector()

	requiredImports := injector.GetRequiredImports()

	// Should include the otel import for the generated bootstrap file
	expectedImports := []string{"otel"}
	if len(requiredImports) != len(expectedImports) {
		t.Errorf("Expected %d required imports, got %d", len(expectedImports), len(requiredImports))
	}

	for i, expected := range expectedImports {
		if requiredImports[i] != expected {
			t.Errorf("Expected import '%s' at position %d, got '%s'", expected, i, requiredImports[i])
		}
	}
}

func TestPythonInjector_FormatSingleImport(t *testing.T) {
	injector := NewPythonInjector()

	// Test Flask instrumentation import
	flaskImport := "opentelemetry.instrumentation.flask"
	formatted := injector.FormatSingleImport(flaskImport)
	expected := "from opentelemetry.instrumentation.flask import FlaskInstrumentor\n"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}

	// Test regular import
	regularImport := "requests"
	formatted = injector.FormatSingleImport(regularImport)
	expected = "import requests\n"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}

	// Test dotted import
	dottedImport := "opentelemetry.trace"
	formatted = injector.FormatSingleImport(dottedImport)
	expected = "from opentelemetry import trace\n"
	if formatted != expected {
		t.Errorf("Expected '%s', got '%s'", expected, formatted)
	}
}

func TestPythonInjector_FlaskCleanup(t *testing.T) {
	injector := NewPythonInjector()

	operationsData := &types.OperationsData{
		InstallOTEL:             true,
		InstallInstrumentations: []string{"flask"},
		InstallComponents:       make(map[string][]string),
	}

	// Test content with duplicate imports and misplaced instrumentation
	problematicContent := []byte(`
from flask import Flask
from opentelemetry.instrumentation import flask

app = Flask(__name__)

    # Instrument Flask application
    from opentelemetry.instrumentation.flask import FlaskInstrumentor
    FlaskInstrumentor().instrument_app(app)

@app.route('/')
def hello():
    return 'Hello, World!'
`)

	modifications := injector.GenerateFrameworkModifications(problematicContent, operationsData)

	// Should generate cleanup modifications
	cleanupCount := 0
	for _, mod := range modifications {
		if mod.Type == types.ModificationRemoveLine {
			cleanupCount++
		}
	}

	if cleanupCount == 0 {
		t.Error("Expected cleanup modifications for problematic Flask code")
	}

	// Verify that we have both cleanup and injection modifications
	hasCleanup := false
	hasInjection := false
	for _, mod := range modifications {
		if mod.Type == types.ModificationRemoveLine {
			hasCleanup = true
		}
		if mod.Type == types.ModificationAddFramework {
			hasInjection = true
		}
	}

	if !hasCleanup {
		t.Error("Expected cleanup modifications")
	}

	if !hasInjection {
		t.Error("Expected injection modifications")
	}
}

// Helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) &&
		(s[:len(substr)] == substr || s[len(s)-len(substr):] == substr ||
			func() bool {
				for i := 0; i <= len(s)-len(substr); i++ {
					if s[i:i+len(substr)] == substr {
						return true
					}
				}
				return false
			}()))
}

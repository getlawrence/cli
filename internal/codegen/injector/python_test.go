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
	if len(modifications) != 1 {
		t.Errorf("Expected 1 modification, got %d", len(modifications))
	}

	mod := modifications[0]
	if mod.Type != types.ModificationAddFramework {
		t.Errorf("Expected ModificationAddFramework, got %s", mod.Type)
	}

	if mod.Framework != "flask" {
		t.Errorf("Expected framework 'flask', got %s", mod.Framework)
	}

	// Verify the content contains Flask instrumentation
	expectedContent := "from opentelemetry.instrumentation.flask import FlaskInstrumentor"
	if !contains(mod.Content, expectedContent) {
		t.Errorf("Expected content to contain '%s', got: %s", expectedContent, mod.Content)
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
	if len(modifications) != 0 {
		t.Errorf("Expected 0 modifications when Flask instrumentation not requested, got %d", len(modifications))
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

	expected := "from opentelemetry.instrumentation import flask\n"
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

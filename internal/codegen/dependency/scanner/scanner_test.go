package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGoModScanner(t *testing.T) {
	scanner := NewGoModScanner()

	t.Run("Detect", func(t *testing.T) {
		// Test with go.mod present
		dir := t.TempDir()
		goModPath := filepath.Join(dir, "go.mod")
		if err := os.WriteFile(goModPath, []byte("module test\n\ngo 1.21\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if !scanner.Detect(dir) {
			t.Error("Expected to detect go.mod")
		}

		// Test without go.mod
		emptyDir := t.TempDir()
		if scanner.Detect(emptyDir) {
			t.Error("Should not detect go.mod in empty directory")
		}
	})

	t.Run("Scan", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				name: "single require",
				content: `module test
go 1.21
require github.com/example/pkg v1.0.0`,
				expected: []string{"github.com/example/pkg"},
			},
			{
				name: "require block",
				content: `module test
go 1.21

require (
	github.com/pkg1/lib v1.0.0
	github.com/pkg2/lib v2.0.0
)`,
				expected: []string{"github.com/pkg1/lib", "github.com/pkg2/lib"},
			},
			{
				name: "with comments",
				content: `module test
go 1.21

require (
	github.com/pkg1/lib v1.0.0
	// github.com/commented/out v1.0.0
	github.com/pkg2/lib v2.0.0 // indirect
)`,
				expected: []string{"github.com/pkg1/lib", "github.com/pkg2/lib"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				goModPath := filepath.Join(dir, "go.mod")
				if err := os.WriteFile(goModPath, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}

				deps, err := scanner.Scan(dir)
				if err != nil {
					t.Fatal(err)
				}

				if len(deps) != len(tt.expected) {
					t.Fatalf("Expected %d dependencies, got %d", len(tt.expected), len(deps))
				}

				for i, dep := range deps {
					if dep != tt.expected[i] {
						t.Errorf("Expected dependency %s, got %s", tt.expected[i], dep)
					}
				}
			})
		}
	})
}

func TestNpmScanner(t *testing.T) {
	scanner := NewNpmScanner()

	t.Run("Detect", func(t *testing.T) {
		// Test with package.json present
		dir := t.TempDir()
		pkgPath := filepath.Join(dir, "package.json")
		if err := os.WriteFile(pkgPath, []byte(`{"name":"test","version":"1.0.0"}`), 0644); err != nil {
			t.Fatal(err)
		}

		if !scanner.Detect(dir) {
			t.Error("Expected to detect package.json")
		}

		// Test without package.json
		emptyDir := t.TempDir()
		if scanner.Detect(emptyDir) {
			t.Error("Should not detect package.json in empty directory")
		}
	})

	t.Run("Scan", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				name: "dependencies only",
				content: `{
					"name": "test",
					"version": "1.0.0",
					"dependencies": {
						"express": "^4.18.0",
						"lodash": "^4.17.21"
					}
				}`,
				expected: []string{"express", "lodash"},
			},
			{
				name: "dependencies and devDependencies",
				content: `{
					"name": "test",
					"version": "1.0.0",
					"dependencies": {
						"express": "^4.18.0"
					},
					"devDependencies": {
						"jest": "^29.0.0"
					}
				}`,
				expected: []string{"express", "jest"},
			},
			{
				name:     "no dependencies",
				content:  `{"name": "test", "version": "1.0.0"}`,
				expected: []string{},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				pkgPath := filepath.Join(dir, "package.json")
				if err := os.WriteFile(pkgPath, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}

				deps, err := scanner.Scan(dir)
				if err != nil {
					t.Fatal(err)
				}

				// Create a map for easier comparison (order doesn't matter)
				depMap := make(map[string]bool)
				for _, dep := range deps {
					depMap[dep] = true
				}

				if len(deps) != len(tt.expected) {
					t.Fatalf("Expected %d dependencies, got %d", len(tt.expected), len(deps))
				}

				for _, exp := range tt.expected {
					if !depMap[exp] {
						t.Errorf("Expected dependency %s not found", exp)
					}
				}
			})
		}
	})
}

func TestPipScanner(t *testing.T) {
	scanner := NewPipScanner()

	t.Run("Detect", func(t *testing.T) {
		// Test with requirements.txt present
		dir := t.TempDir()
		reqPath := filepath.Join(dir, "requirements.txt")
		if err := os.WriteFile(reqPath, []byte("flask==2.0.0\n"), 0644); err != nil {
			t.Fatal(err)
		}

		if !scanner.Detect(dir) {
			t.Error("Expected to detect requirements.txt")
		}

		// Test without requirements.txt
		emptyDir := t.TempDir()
		if scanner.Detect(emptyDir) {
			t.Error("Should not detect requirements.txt in empty directory")
		}
	})

	t.Run("Scan", func(t *testing.T) {
		tests := []struct {
			name     string
			content  string
			expected []string
		}{
			{
				name: "simple requirements",
				content: `flask==2.0.0
requests>=2.28.0
django~=4.0`,
				expected: []string{"flask", "requests", "django"},
			},
			{
				name: "with comments and empty lines",
				content: `# Web framework
flask==2.0.0

# HTTP library
requests>=2.28.0
# django~=4.0  # commented out`,
				expected: []string{"flask", "requests"},
			},
			{
				name: "various version specifiers",
				content: `pkg1==1.0.0
pkg2>=2.0.0
pkg3<=3.0.0
pkg4>4.0.0
pkg5<5.0.0
pkg6~=6.0.0
pkg7!=7.0.0`,
				expected: []string{"pkg1", "pkg2", "pkg3", "pkg4", "pkg5", "pkg6", "pkg7"},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				dir := t.TempDir()
				reqPath := filepath.Join(dir, "requirements.txt")
				if err := os.WriteFile(reqPath, []byte(tt.content), 0644); err != nil {
					t.Fatal(err)
				}

				deps, err := scanner.Scan(dir)
				if err != nil {
					t.Fatal(err)
				}

				if len(deps) != len(tt.expected) {
					t.Fatalf("Expected %d dependencies, got %d", len(tt.expected), len(deps))
				}

				for i, dep := range deps {
					if dep != tt.expected[i] {
						t.Errorf("Expected dependency %s, got %s", tt.expected[i], dep)
					}
				}
			})
		}
	})
}

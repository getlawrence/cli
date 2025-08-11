package injector

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
)

func TestInjectOtelInitialization_PerLanguage(t *testing.T) {
	t.Parallel()

	type testCase struct {
		name            string
		language        string
		filename        string
		source          string
		expectInitSub   string
		expectImportSub string
	}

	cases := []testCase{
		{
			name:     "Go",
			language: "go",
			filename: "main.go",
			source: `package main

import (
    "fmt"
)

func main() {
    logger.Log("hi")
}
`,
			expectInitSub:   "SetupOTEL()",
			expectImportSub: "",
		},
		{
			name:     "JavaScript",
			language: "javascript",
			filename: "index.js",
			// Ensure at least 3 lines so default import insertion at line 3 applies
			source: `// line1
// line2
console.log('hi');
`,
			expectInitSub:   "require('./otel')",
			expectImportSub: "",
		},
		{
			name:     "Python",
			language: "python",
			filename: "app.py",
			// Ensure at least 3 lines so default import insertion at line 3 applies when no imports exist
			source: `# line1
# line2
if __name__ == '__main__':
    print('hi')
`,
			expectInitSub:   "init_tracer()",
			expectImportSub: "from opentelemetry",
		},
		{
			name:     "Java",
			language: "java",
			filename: "App.java",
			source: `public class App {
    public static void main(String[] args) {
        System.out.println("hi");
    }
}
`,
			expectInitSub:   "GlobalOpenTelemetry.get()",
			expectImportSub: "import io.opentelemetry.api.GlobalOpenTelemetry;",
		},
		{
			name:     "CSharp",
			language: "csharp",
			filename: "Program.cs",
			source: `using System;

public class Program {
    public static void Main(string[] args) {
        Console.WriteLine("hi");
    }
}
`,
			expectInitSub:   "AddOpenTelemetry(",
			expectImportSub: "using OpenTelemetry;",
		},
		{
			name:     "Ruby",
			language: "ruby",
			filename: "app.rb",
			source: `# ruby app
puts 'hi'
`,
			expectInitSub:   "Lawrence::OTel.setup",
			expectImportSub: "require \"opentelemetry-sdk\"",
		},
		{
			name:     "PHP",
			language: "php",
			filename: "index.php",
			source: `<?php
// php app
echo "hi";
`,
			expectInitSub:   "setup_otel();",
			expectImportSub: "require_once './otel.php';",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			tmpDir := t.TempDir()
			filePath := filepath.Join(tmpDir, tc.filename)
			if err := os.WriteFile(filePath, []byte(tc.source), 0o644); err != nil {
				t.Fatalf("failed writing temp source: %v", err)
			}

			injector := NewCodeInjector()
			ctx := context.Background()

			entry := &domain.EntryPoint{
				FilePath: filePath,
				Language: tc.language,
			}

			ops := &types.OperationsData{
				InstallOTEL:             true,
				InstallInstrumentations: []string{},
				InstallComponents:       map[string][]string{},
			}

			req := types.GenerationRequest{
				CodebasePath: tmpDir,
				Method:       "code",
				AgentType:    "",
				Config: types.StrategyConfig{
					Mode:   types.TemplateMode, // not used by injector, but set for completeness
					DryRun: false,              // write to file so we can assert contents
				},
			}

			_, err := injector.InjectOtelInitialization(ctx, entry, ops, req)
			if err != nil {
				t.Fatalf("injection failed for %s: %v", tc.name, err)
			}

			// Read modified file
			out, err := os.ReadFile(filePath)
			if err != nil {
				t.Fatalf("failed reading modified file: %v", err)
			}
			content := string(out)

			if !strings.Contains(content, tc.expectInitSub) {
				t.Fatalf("expected initialization snippet for %s to contain %q; got:\n%s", tc.name, tc.expectInitSub, content)
			}
			if !strings.Contains(content, tc.expectImportSub) {
				t.Fatalf("expected imports for %s to contain %q; got:\n%s", tc.name, tc.expectImportSub, content)
			}
		})
	}
}

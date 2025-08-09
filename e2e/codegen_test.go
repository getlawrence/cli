package e2e

import (
	"context"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestCodegenTemplateDryRunOnGoSample(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()

	// Build binary
	binaryName := "lawrence"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, string(out))
		}
	}

	// Run codegen in template dry-run mode against examples/go
	samplePath := filepath.Join(repoRoot, "examples", "go")
	outputDir := filepath.Join(tmpDir, "out")

	{
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		args := []string{
			"codegen",
			"--mode", "template",
			"--method", "code",
			"--path", samplePath,
			"--output", outputDir,
			"--dry-run",
		}
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codegen failed: %v\n%s", err, string(out))
		}
		stdout := string(out)
		// Basic smoke assertions that confirm template strategy ran and produced output paths
		if !strings.Contains(stdout, "Using Template-based generation strategy") && !strings.Contains(stdout, "Template-based") {
			t.Fatalf("expected output to indicate template strategy; got: %s", stdout)
		}
		if !strings.Contains(stdout, "Generated Go instrumentation code (dry run)") && !strings.Contains(stdout, "Successfully generated") {
			t.Fatalf("expected output to show generated code paths; got: %s", stdout)
		}
		if !strings.Contains(stdout, outputDir) && !strings.Contains(stdout, "examples/go") {
			t.Fatalf("expected output paths to be under %s or examples/go; got: %s", outputDir, stdout)
		}
	}
}

func TestCodegenTemplateDryRunOnOtherSamples(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()

	// Build binary once
	binaryName := "lawrence"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, string(out))
		}
	}

	cases := []struct{ rel string }{
		{filepath.Join("examples", "js")},
		{filepath.Join("examples", "python")},
		{filepath.Join("examples", "java")},
		{filepath.Join("examples", "ruby")},
		{filepath.Join("examples", "csharp")},
		{filepath.Join("examples", "php")},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.rel, func(t *testing.T) {
			t.Parallel()
			samplePath := filepath.Join(repoRoot, tc.rel)
			ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
			defer cancel()
			args := []string{
				"codegen",
				"--mode", "template",
				"--method", "code",
				"--path", samplePath,
				"--dry-run",
			}
			cmd := exec.CommandContext(ctx, binaryPath, args...)
			cmd.Dir = repoRoot
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("codegen failed for %s: %v\n%s", tc.rel, err, string(out))
			}
			stdout := string(out)
			// Accept either that we ran the template strategy or there were no opportunities
			if strings.Contains(stdout, "Generate: No code generation opportunities found") {
				return
			}
			if !strings.Contains(stdout, "Using Template-based generation strategy") && !strings.Contains(stdout, "Template-based") {
				t.Fatalf("[%s] expected template strategy notice or no-op; got: %s", tc.rel, stdout)
			}
			if !strings.Contains(stdout, "Successfully generated") {
				// Some languages print a specific generation line; accept that as success too
				if !strings.Contains(strings.ToLower(stdout), "generated") {
					t.Fatalf("[%s] expected generated output mention; got: %s", tc.rel, stdout)
				}
			}
		})
	}
}

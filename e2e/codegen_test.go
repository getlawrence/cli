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
		if !strings.Contains(stdout, outputDir) {
			t.Fatalf("expected output paths to be under %s; got: %s", outputDir, stdout)
		}
	}
}

package e2e

import (
	"bufio"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestExamplesStackCodegenAndOTEL spins up docker-compose for examples after running codegen outside
func TestExamplesStackCodegenAndOTEL(t *testing.T) {
	t.Parallel()

	// Skip on Windows CI to simplify docker/network differences
	if runtime.GOOS == "windows" {
		t.Skip("docker-compose test skipped on Windows")
	}

	repoRoot := findRepoRoot(t)
	examplesDir := filepath.Join(repoRoot, "examples")

	// Build CLI
	tmpDir := t.TempDir()
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

	// Run codegen (template) for selected example projects outside of docker
	projects := []string{"go", "js", "python"}
	for _, p := range projects {
		projPath := filepath.Join(examplesDir, p)
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		args := []string{"codegen", projPath, "--mode", "template"}
		cmd := exec.CommandContext(ctx, binaryPath, args...)
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("codegen failed for %s: %v\n%s", p, err, string(out))
		}
	}

	// Ensure docker CLI is available
	{
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := exec.CommandContext(ctx, "docker", "version").Run(); err != nil {
			t.Skip("docker not available: " + err.Error())
		}
	}

	// docker compose build and up
	composeFile := filepath.Join(examplesDir, "docker-compose.yml")
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "build")
		cmd.Dir = examplesDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("docker compose build failed: %v\n%s", err, string(out))
		}
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "up", "-d")
		cmd.Dir = examplesDir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("docker compose up failed: %v\n%s", err, string(out))
		}
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "down", "-v").Run()
		// Best-effort: restore examples directory to clean state
		// Only run if inside a git repo
		if err := exec.CommandContext(ctx, "git", "rev-parse", "--is-inside-work-tree").Run(); err == nil {
			_ = exec.CommandContext(ctx, "git", "checkout", "--", "examples").Run()
		}
		// Remove collector data dir
		_ = os.RemoveAll(filepath.Join(examplesDir, ".otel-data"))
	}()

	// Give services a moment
	time.Sleep(5 * time.Second)

	// Stimulate each service to emit at least one request
	hits := []struct{ url string }{
		{"http://localhost:8080/"},
		{"http://localhost:3000/"},
		{"http://localhost:5000/"},
	}
	for _, h := range hits {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		cmd := exec.CommandContext(ctx, "curl", "-sf", h.url)
		out, err := cmd.CombinedOutput()
		cancel()
		if err != nil {
			t.Fatalf("failed to hit %s: %v\n%s", h.url, err, string(out))
		}
	}

	// Wait for collector to flush
	time.Sleep(5 * time.Second)

	// Verify traces file has content
	tracesPath := filepath.Join(examplesDir, ".otel-data", "traces.jsonl")
	f, err := os.Open(tracesPath)
	if err != nil {
		t.Fatalf("failed to open traces file: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	found := false
	// scan a few lines only
	for i := 0; i < 100 && scanner.Scan(); i++ {
		line := scanner.Text()
		if strings.Contains(line, "resourceSpans") || strings.Contains(line, "scopeSpans") || strings.Contains(line, "spanId") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("no OTEL trace signals detected in %s", tracesPath)
	}
}

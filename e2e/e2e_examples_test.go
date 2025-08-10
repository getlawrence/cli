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

	// Work entirely in a temp copy so repo files are never modified
	tempRoot := t.TempDir()
	examplesSrc := filepath.Join(repoRoot, "examples")
	examplesDir := filepath.Join(tempRoot, "examples")
	t.Logf("Copying examples to temp dir: %s -> %s", examplesSrc, examplesDir)
	if err := copyDir(examplesSrc, examplesDir); err != nil {
		t.Fatalf("failed to copy examples to temp dir: %v", err)
	}
	t.Logf("Copied examples to temp directory")

	// Build CLI
	_, binaryPath := buildCLIBinary(t)

	// Run codegen for the examples directory
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	t.Logf("Running codegen for examples: %s", examplesDir)
	args := []string{"codegen", examplesDir, "--mode", "template"}
	if err := runAndStreamOutput(t, ctx, repoRoot, binaryPath, args...); err != nil {
		t.Fatalf("codegen failed: %v", err)
	}
	t.Logf("Codegen succeeded for project: %s", examplesDir)

	// Ensure docker CLI is available
	requireDocker(t)

	// docker compose build and up
	composeFile := filepath.Join(examplesDir, "docker-compose.yml")
	// Ensure host traces directory exists for bind mount
	if err := os.MkdirAll(filepath.Join(examplesDir, ".otel-data"), 0o755); err != nil {
		t.Fatalf("failed to create traces output directory: %v", err)
	}
	{
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		t.Logf("Running docker compose build (this may take a while)...")
		if err := dockerCompose(t, ctx, examplesDir, composeFile, "build"); err != nil {
			t.Fatalf("docker compose build failed: %v", err)
		}
		t.Logf("docker compose build completed")
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		t.Logf("Running docker compose up -d...")
		if err := dockerCompose(t, ctx, examplesDir, composeFile, "up", "-d"); err != nil {
			t.Fatalf("docker compose up failed: %v", err)
		}
		t.Logf("docker compose up completed")
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		_ = exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "down", "-v").Run()
		// Remove collector data dir inside temp examples (temp dir will be removed automatically)
		_ = os.RemoveAll(filepath.Join(examplesDir, ".otel-data"))
	}()

	// Give services a moment
	t.Logf("Waiting 10s for services to start...")
	time.Sleep(10 * time.Second)

	// Stimulate each service to emit multiple requests
	hits := []struct{ url string }{
		{"http://localhost:8080/"}, // go-service
		{"http://localhost:3000/"}, // js-service
		{"http://localhost:5001/"}, // python-service
		{"http://localhost:8000/"}, // php-service
		{"http://localhost:4567/"}, // ruby-service
		{"http://localhost:8083/"}, // csharp-service
	}
	for _, h := range hits {
		// First ensure service is up
		if err := waitForURLWithRetry(t, h.url, 15, 3*time.Second, 1*time.Second); err != nil {
			// Diagnostic: show compose ps and python-service logs
			{
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "ps")
				cmd.Dir = examplesDir
				out, _ := cmd.CombinedOutput()
				t.Logf("docker compose ps\n%s", string(out))
			}
			{
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "logs", "python-service")
				cmd.Dir = examplesDir
				out, _ := cmd.CombinedOutput()
				t.Logf("python-service logs:\n%s", string(out))
			}
			{
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "logs", "ruby-service")
				cmd.Dir = examplesDir
				out, _ := cmd.CombinedOutput()
				t.Logf("ruby-service logs:\n%s", string(out))
			}
			t.Fatalf("failed to hit %s after retries: %v", h.url, err)
		}
		t.Logf("Service is up: %s", h.url)

		// Then send a few more requests with small sleeps in between
		for i := 0; i < 3; i++ {
			if err := hitURL(h.url, 2*time.Second); err != nil {
				t.Logf("warning: request %d to %s failed: %v", i+1, h.url, err)
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// Wait for collector to flush and file to be created
	tracesPath := filepath.Join(examplesDir, ".otel-data", "traces.jsonl")
	t.Logf("Waiting for OTEL traces file at: %s", tracesPath)
	f, err := waitForFileExists(tracesPath, 30*time.Second)
	if err != nil {
		// dump collector logs for debugging
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		cmd := exec.CommandContext(ctx, "docker", "compose", "-f", composeFile, "logs", "otel-collector")
		cmd.Dir = examplesDir
		out, _ := cmd.CombinedOutput()
		t.Logf("otel-collector logs:\n%s", string(out))
		t.Fatalf("failed to open traces file after waiting: %v", err)
	}
	t.Logf("Traces file opened successfully")
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
	t.Logf("Detected OTEL trace signals in %s", tracesPath)
}

package e2e

import (
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
	f.Close()

	// Verify we have at least one span for each expected service
	expectedServices := map[string]bool{
		"examples-go":     false,
		"examples-js":     false,
		"examples-python": false,
		"examples-php":    false,
		"examples-ruby":   false,
		"examples-csharp": false,
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		// Re-read the file to capture newly written lines
		data, err := os.ReadFile(tracesPath)
		if err != nil {
			t.Fatalf("failed to read traces file: %v", err)
		}
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			// Fast-path string checks to avoid brittleness across SDKs
			for svc, found := range expectedServices {
				if found {
					continue
				}
				if strings.Contains(line, "\"service.name\"") && strings.Contains(line, svc) && strings.Contains(line, "\"spans\":") {
					// Heuristic: ensure the line is not an empty spans array for this resource
					if !strings.Contains(line, "\"spans\":[]") {
						expectedServices[svc] = true
					}
				}
			}
		}

		// Check if all services have been observed
		allFound := true
		for _, found := range expectedServices {
			if !found {
				allFound = false
				break
			}
		}
		if allFound {
			break
		}
		time.Sleep(1 * time.Second)
	}

	// Report any missing services
	missing := make([]string, 0)
	for svc, found := range expectedServices {
		if !found {
			missing = append(missing, svc)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("did not observe spans for services: %v", strings.Join(missing, ", "))
	}
	t.Logf("Observed spans for all expected services")
}

package e2e

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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
		{"http://localhost:8082/"}, // java-service
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

	// Wait for traces to appear in Jaeger
	t.Logf("Waiting for traces to appear in Jaeger...")
	expectedServices := map[string]bool{
		"examples-go":     false,
		"examples-js":     false,
		"examples-python": false,
		"examples-php":    false,
		"examples-ruby":   false,
		"examples-csharp": false,
		"examples-java":   false,
	}

	deadline := time.Now().Add(60 * time.Second)
	for time.Now().Before(deadline) {
		// Query Jaeger for services
		response, err := queryJaegerServices()
		if err != nil {
			t.Logf("Failed to query Jaeger: %v, retrying...", err)
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if we have the expected services in the response
		for _, serviceName := range response.Data {
			if expected, exists := expectedServices[serviceName]; exists && !expected {
				expectedServices[serviceName] = true
				t.Logf("Found service: %s", serviceName)
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

func queryJaegerServices() (*JaegerServicesResponse, error) {
	url := "http://localhost:16686/api/services"

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to query Jaeger API: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Jaeger API returned status: %s", resp.Status)
	}

	var response JaegerServicesResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("failed to decode Jaeger response: %v", err)
	}

	return &response, nil
}

// JaegerServicesResponse represents the response from Jaeger services API
type JaegerServicesResponse struct {
	Data   []string `json:"data"`
	Total  int      `json:"total"`
	Limit  int      `json:"limit"`
	Offset int      `json:"offset"`
	Errors []string `json:"errors"`
}

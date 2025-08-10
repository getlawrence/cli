package e2e

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
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
	tmpDir := t.TempDir()
	binaryName := "lawrence"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		t.Logf("Building CLI binary: %s", binaryPath)
		cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, ".")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, string(out))
		}
		t.Logf("CLI build completed")
	}

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
	{
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		t.Logf("Checking docker availability")
		if err := exec.CommandContext(ctx, "docker", "version").Run(); err != nil {
			t.Skip("docker not available: " + err.Error())
		}
		t.Logf("Docker is available")
	}

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
		if err := runAndStreamOutput(t, ctx, examplesDir, "docker", "compose", "-f", composeFile, "build"); err != nil {
			t.Fatalf("docker compose build failed: %v", err)
		}
		t.Logf("docker compose build completed")
	}

	{
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()
		t.Logf("Running docker compose up -d...")
		if err := runAndStreamOutput(t, ctx, examplesDir, "docker", "compose", "-f", composeFile, "up", "-d"); err != nil {
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

	// Stimulate each service to emit at least one request
	hits := []struct{ url string }{
		{"http://localhost:8080/"}, // go-service
		{"http://localhost:3000/"}, // js-service
		{"http://localhost:5001/"}, // python-service
		{"http://localhost:8000/"}, // php-service
		{"http://localhost:4567/"}, // ruby-service
		{"http://localhost:8083/"}, // csharp-service
	}
	for _, h := range hits {
		var lastErr error
		for attempt := 0; attempt < 15; attempt++ {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			t.Logf("Hitting %s (attempt %d/15)", h.url, attempt+1)
			cmd := exec.CommandContext(ctx, "curl", "-sf", h.url)
			out, err := cmd.CombinedOutput()
			cancel()
			if err == nil {
				lastErr = nil
				t.Logf("Service responded OK: %s", h.url)
				break
			}
			lastErr = fmt.Errorf("%v: %s", err, string(out))
			time.Sleep(1 * time.Second)
		}
		if lastErr != nil {
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
			t.Fatalf("failed to hit %s after retries: %v", h.url, lastErr)
		}
	}

	// Wait for collector to flush and file to be created
	tracesPath := filepath.Join(examplesDir, ".otel-data", "traces.jsonl")
	t.Logf("Waiting for OTEL traces file at: %s", tracesPath)
	var f *os.File
	var err error
	for i := 0; i < 30; i++ { // up to ~30s
		f, err = os.Open(tracesPath)
		if err == nil {
			break
		}
		time.Sleep(1 * time.Second)
	}
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

// runAndStreamOutput executes a command and streams its stdout/stderr to the test logs.
func runAndStreamOutput(t *testing.T, ctx context.Context, dir string, name string, args ...string) error {
	t.Helper()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		scanAndLog(t, stdout)
	}()
	go func() {
		defer wg.Done()
		scanAndLog(t, stderr)
	}()

	wg.Wait()
	return cmd.Wait()
}

func scanAndLog(t *testing.T, r io.Reader) {
	t.Helper()
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		t.Logf("%s", scanner.Text())
	}
}

// copyDir recursively copies a directory tree from src to dst.
// It preserves file modes and creates directories as needed.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if !srcInfo.IsDir() {
		return &os.PathError{Op: "copy", Path: src, Err: os.ErrInvalid}
	}
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		targetPath := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(targetPath, info.Mode())
		}
		// Handle symlinks
		if info.Mode()&os.ModeSymlink != 0 {
			linkTarget, err := os.Readlink(path)
			if err != nil {
				return err
			}
			return os.Symlink(linkTarget, targetPath)
		}
		if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(targetPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode())
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

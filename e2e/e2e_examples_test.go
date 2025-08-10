package e2e

import (
	"bufio"
	"context"
	"io"
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
	if err := copyDir(examplesSrc, examplesDir); err != nil {
		t.Fatalf("failed to copy examples to temp dir: %v", err)
	}

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
		// Remove collector data dir inside temp examples (temp dir will be removed automatically)
		_ = os.RemoveAll(filepath.Join(examplesDir, ".otel-data"))
	}()

	// Give services a moment
	time.Sleep(5 * time.Second)

	// Stimulate each service to emit at least one request
	hits := []struct{ url string }{
		{"http://localhost:8080/"},
		{"http://localhost:3000/"},
		{"http://localhost:5001/"},
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

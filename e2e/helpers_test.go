package e2e

import (
	"bufio"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
	"time"
)

// buildCLIBinary builds the CLI into a temp dir and returns (repoRoot, binaryPath).
func buildCLIBinary(t *testing.T) (string, string) {
	t.Helper()
	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()
	binaryName := "lawrence"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

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
	return repoRoot, binaryPath
}

// requireDocker verifies docker is available or skips the test.
func requireDocker(t *testing.T) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	t.Logf("Checking docker availability")
	if err := exec.CommandContext(ctx, "docker", "version").Run(); err != nil {
		t.Skip("docker not available: " + err.Error())
	}
	t.Logf("Docker is available")
}

// dockerCompose runs `docker compose -f <composeFile> <args...>` in the given dir with streaming logs.
func dockerCompose(t *testing.T, ctx context.Context, dir, composeFile string, args ...string) error {
	t.Helper()
	base := []string{"compose", "-f", composeFile}
	full := append(base, args...)
	return runAndStreamOutput(t, ctx, dir, "docker", full...)
}

// waitForURLWithRetry issues GET requests until success or attempts exhausted.
func waitForURLWithRetry(t *testing.T, url string, attempts int, timeoutPerAttempt, backoff time.Duration) error {
	t.Helper()
	client := &http.Client{Timeout: timeoutPerAttempt}
	var lastErr error
	for attempt := 0; attempt < attempts; attempt++ {
		t.Logf("Hitting %s (attempt %d/%d)", url, attempt+1, attempts)
		resp, err := client.Get(url)
		if err == nil && resp != nil && resp.Body != nil {
			// Drain and close
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
		}
		if err == nil && resp != nil && resp.StatusCode >= 200 && resp.StatusCode < 400 {
			return nil
		}
		if err != nil {
			lastErr = err
		} else if resp != nil {
			lastErr = errors.New(resp.Status)
		}
		time.Sleep(backoff)
	}
	if lastErr == nil {
		lastErr = errors.New("exhausted attempts without success")
	}
	return lastErr
}

// hitURL performs a single GET request with the provided timeout.
func hitURL(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: timeout}
	resp, err := client.Get(url)
	if err == nil && resp != nil && resp.Body != nil {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
	if err != nil {
		return err
	}
	if resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 400 {
		if resp == nil {
			return errors.New("no response")
		}
		return errors.New(resp.Status)
	}
	return nil
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

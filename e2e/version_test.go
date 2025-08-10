package e2e

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// findRepoRoot walks up from the current working directory to locate go.mod
func findRepoRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("could not locate go.mod from %s", dir)
		}
		dir = parent
	}
}

func TestVersionFlagOutputsInjectedVersion(t *testing.T) {
	t.Parallel()

	repoRoot := findRepoRoot(t)
	tmpDir := t.TempDir()

	binaryName := "lawrence"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tmpDir, binaryName)

	injectedVersion := "e2e-smoke"
	ldflags := fmt.Sprintf("-X github.com/getlawrence/cli/cmd.Version=%s", injectedVersion)

	// Build the CLI binary with injected version
	{
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cmd := exec.CommandContext(ctx, "go", "build", "-o", binaryPath, "-ldflags", ldflags, ".")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("build failed: %v\n%s", err, string(out))
		}
	}

	// Run the binary with --version and verify the output contains the injected version
	{
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		cmd := exec.CommandContext(ctx, binaryPath, "--version")
		cmd.Dir = repoRoot
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("running --version failed: %v\n%s", err, string(out))
		}
		output := string(out)
		if !strings.Contains(output, injectedVersion) {
			t.Fatalf("expected version output to contain %q, got: %q", injectedVersion, output)
		}
	}
}

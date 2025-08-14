package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// MockCommander implements types.Commander for testing
type MockCommander struct {
	lookPathFunc func(string) (string, error)
	runFunc      func(context.Context, string, []string, string) (string, error)
}

func (m *MockCommander) LookPath(file string) (string, error) {
	if m.lookPathFunc != nil {
		return m.lookPathFunc(file)
	}
	return "", os.ErrNotExist
}

func (m *MockCommander) Run(ctx context.Context, command string, args []string, dir string) (string, error) {
	if m.runFunc != nil {
		output, err := m.runFunc(ctx, command, args, dir)
		return output, err
	}
	return "", nil
}

func TestPipInstaller_Install(t *testing.T) {
	tests := []struct {
		name         string
		dependencies []string
		lookPathFunc func(string) (string, error)
		runFunc      func(context.Context, string, []string, string) (string, error)
		expectError  bool
	}{
		{
			name:         "empty dependencies should not error",
			dependencies: []string{},
			expectError:  false,
		},
		{
			name:         "pip available should use pip install",
			dependencies: []string{"opentelemetry-api", "opentelemetry-sdk"},
			lookPathFunc: func(file string) (string, error) {
				if file == "pip" {
					return "/usr/bin/pip", nil
				}
				return "", os.ErrNotExist
			},
			runFunc: func(ctx context.Context, command string, args []string, dir string) (string, error) {
				if command == "pip" && len(args) > 0 && args[0] == "install" {
					return "success", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:         "pip3 available should use pip3 install",
			dependencies: []string{"opentelemetry-api"},
			lookPathFunc: func(file string) (string, error) {
				if file == "pip" {
					return "", os.ErrNotExist
				}
				if file == "pip3" {
					return "/usr/bin/pip3", nil
				}
				return "", os.ErrNotExist
			},
			runFunc: func(ctx context.Context, command string, args []string, dir string) (string, error) {
				if command == "pip3" && len(args) > 0 && args[0] == "install" {
					return "success", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:         "python available should use python -m pip",
			dependencies: []string{"opentelemetry-api"},
			lookPathFunc: func(file string) (string, error) {
				if file == "python" {
					return "/usr/bin/python", nil
				}
				return "", os.ErrNotExist
			},
			runFunc: func(ctx context.Context, command string, args []string, dir string) (string, error) {
				if command == "python" && len(args) > 0 && args[0] == "-m" {
					return "success", nil
				}
				return "", nil
			},
			expectError: false,
		},
		{
			name:         "no pip available should fallback to requirements.txt",
			dependencies: []string{"opentelemetry-api"},
			lookPathFunc: func(file string) (string, error) {
				return "", os.ErrNotExist
			},
			expectError: false,
		},
		{
			name:         "pip install failure should fallback to requirements.txt",
			dependencies: []string{"opentelemetry-api"},
			lookPathFunc: func(file string) (string, error) {
				if file == "pip" {
					return "/usr/bin/pip", nil
				}
				return "", os.ErrNotExist
			},
			runFunc: func(ctx context.Context, command string, args []string, dir string) (string, error) {
				return "", os.ErrPermission
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempDir := t.TempDir()

			mockCommander := &MockCommander{
				lookPathFunc: tt.lookPathFunc,
				runFunc:      tt.runFunc,
			}

			installer := NewPipInstaller(mockCommander)

			err := installer.Install(context.Background(), tempDir, tt.dependencies, false)

			if tt.expectError && err == nil {
				t.Errorf("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPipInstaller_Install_DryRun(t *testing.T) {
	tempDir := t.TempDir()

	mockCommander := &MockCommander{}
	installer := NewPipInstaller(mockCommander)

	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk"}

	// Dry run should not create any files or run commands
	err := installer.Install(context.Background(), tempDir, dependencies, true)
	if err != nil {
		t.Errorf("dry run should not error: %v", err)
	}

	// Verify no requirements.txt was created
	reqPath := filepath.Join(tempDir, "requirements.txt")
	if _, err := os.Stat(reqPath); err == nil {
		t.Error("requirements.txt should not be created in dry run mode")
	}
}

func TestPipInstaller_EditRequirements_EmptyFile(t *testing.T) {
	tempDir := t.TempDir()
	reqPath := filepath.Join(tempDir, "requirements.txt")

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)
	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk"}

	err := installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Verify requirements.txt was created with dependencies
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)
	for _, dep := range dependencies {
		if !strings.Contains(contentStr, dep) {
			t.Errorf("requirements.txt should contain %s", dep)
		}
	}
}

func TestPipInstaller_EditRequirements_ExistingLibraries(t *testing.T) {
	tempDir := t.TempDir()
	reqPath := filepath.Join(tempDir, "requirements.txt")

	// Create requirements.txt with existing libraries
	existingContent := `# Existing dependencies
requests==2.28.0
flask>=2.0.0
# Comment line
`
	err := os.WriteFile(reqPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to create requirements.txt: %v", err)
	}

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)
	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk"}

	err = installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Verify requirements.txt contains both existing and new dependencies
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)

	// Check existing dependencies are preserved
	if !strings.Contains(contentStr, "requests==2.28.0") {
		t.Error("existing dependency 'requests==2.28.0' should be preserved")
	}
	if !strings.Contains(contentStr, "flask>=2.0.0") {
		t.Error("existing dependency 'flask>=2.0.0' should be preserved")
	}

	// Check new dependencies are added
	for _, dep := range dependencies {
		if !strings.Contains(contentStr, dep) {
			t.Errorf("requirements.txt should contain %s", dep)
		}
	}

	// Check comment is preserved
	if !strings.Contains(contentStr, "# Existing dependencies") {
		t.Error("comment should be preserved")
	}
}

func TestPipInstaller_EditRequirements_DuplicatePrevention(t *testing.T) {
	tempDir := t.TempDir()
	reqPath := filepath.Join(tempDir, "requirements.txt")

	// Create requirements.txt with existing libraries
	existingContent := `opentelemetry-api==1.20.0
opentelemetry-sdk>=1.20.0
`
	err := os.WriteFile(reqPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to create requirements.txt: %v", err)
	}

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)
	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk", "new-package"}

	err = installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Verify requirements.txt doesn't have duplicates
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)

	// Count occurrences of each dependency
	apiCount := strings.Count(contentStr, "opentelemetry-api")
	sdkCount := strings.Count(contentStr, "opentelemetry-sdk")
	newPkgCount := strings.Count(contentStr, "new-package")

	if apiCount != 1 {
		t.Errorf("opentelemetry-api should appear exactly once, got %d", apiCount)
	}
	if sdkCount != 1 {
		t.Errorf("opentelemetry-sdk should appear exactly once, got %d", sdkCount)
	}
	if newPkgCount != 1 {
		t.Errorf("new-package should appear exactly once, got %d", newPkgCount)
	}
}

func TestPipInstaller_ResolveVersions(t *testing.T) {
	tempDir := t.TempDir()

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)

	// Test dependencies with and without version specifiers
	dependencies := []string{
		"opentelemetry-api",
		"opentelemetry-sdk==1.20.0",
		"requests>=2.28.0",
		"flask",
	}

	err := installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Verify all dependencies are in requirements.txt
	reqPath := filepath.Join(tempDir, "requirements.txt")
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)
	for _, dep := range dependencies {
		if !strings.Contains(contentStr, dep) {
			t.Errorf("requirements.txt should contain %s", dep)
		}
	}
}

func TestPipInstaller_UpdateRequirements(t *testing.T) {
	tempDir := t.TempDir()
	reqPath := filepath.Join(tempDir, "requirements.txt")

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)
	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk"}

	// First install
	err := installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Second install with same dependencies
	err = installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to reinstall: %v", err)
	}

	// Verify requirements.txt still contains all dependencies
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)
	for _, dep := range dependencies {
		if !strings.Contains(contentStr, dep) {
			t.Errorf("requirements.txt should contain %s", dep)
		}
	}
}

func TestPipInstaller_EditRequirements_NoTrailingNewline(t *testing.T) {
	tempDir := t.TempDir()
	reqPath := filepath.Join(tempDir, "requirements.txt")

	// Create requirements.txt with existing libraries but NO trailing newline
	// This simulates the bug where dependencies get concatenated
	existingContent := `flask==2.3.3
requests==2.28.0`
	// Note: no trailing newline - this is the bug trigger

	err := os.WriteFile(reqPath, []byte(existingContent), 0644)
	if err != nil {
		t.Fatalf("failed to create requirements.txt: %v", err)
	}

	mockCommander := &MockCommander{
		lookPathFunc: func(file string) (string, error) {
			return "", os.ErrNotExist
		},
	}

	installer := NewPipInstaller(mockCommander)
	dependencies := []string{"opentelemetry-api", "opentelemetry-sdk"}

	err = installer.Install(context.Background(), tempDir, dependencies, false)
	if err != nil {
		t.Errorf("failed to install: %v", err)
	}

	// Verify requirements.txt contains dependencies on separate lines
	content, err := os.ReadFile(reqPath)
	if err != nil {
		t.Errorf("failed to read requirements.txt: %v", err)
	}

	contentStr := string(content)

	// Check that existing dependencies are preserved correctly
	if !strings.Contains(contentStr, "flask==2.3.3") {
		t.Error("existing dependency 'flask==2.3.3' should be preserved")
	}
	if !strings.Contains(contentStr, "requests==2.28.0") {
		t.Error("existing dependency 'requests==2.28.0' should be preserved")
	}

	// Check that new dependencies are added on separate lines
	if !strings.Contains(contentStr, "opentelemetry-api") {
		t.Error("requirements.txt should contain opentelemetry-api")
	}
	if !strings.Contains(contentStr, "opentelemetry-sdk") {
		t.Error("requirements.txt should contain opentelemetry-sdk")
	}

	// CRITICAL: Check that dependencies are NOT concatenated on the same line
	lines := strings.Split(strings.TrimSpace(contentStr), "\n")

	// Check that flask line is not corrupted
	flaskLine := ""
	for _, line := range lines {
		if strings.Contains(line, "flask") {
			flaskLine = line
			break
		}
	}
	if flaskLine != "flask==2.3.3" {
		t.Errorf("flask line should be exactly 'flask==2.3.3', got '%s'", flaskLine)
	}

	// Check that requests line is not corrupted
	requestsLine := ""
	for _, line := range lines {
		if strings.Contains(line, "requests") {
			requestsLine = line
			break
		}
	}
	if requestsLine != "requests==2.28.0" {
		t.Errorf("requests line should be exactly 'requests==2.28.0', got '%s'", requestsLine)
	}

	// Check that opentelemetry dependencies are on their own lines
	apiLine := ""
	sdkLine := ""
	for _, line := range lines {
		if strings.Contains(line, "opentelemetry-api") {
			apiLine = line
		}
		if strings.Contains(line, "opentelemetry-sdk") {
			sdkLine = line
		}
	}

	if apiLine != "opentelemetry-api" {
		t.Errorf("opentelemetry-api should be on its own line, got '%s'", apiLine)
	}
	if sdkLine != "opentelemetry-sdk" {
		t.Errorf("opentelemetry-sdk should be on its own line, got '%s'", sdkLine)
	}

	// Verify no malformed concatenated lines exist
	if strings.Contains(contentStr, "flask==2.3.3opentelemetry-api") {
		t.Error("dependencies should not be concatenated on the same line")
	}
	if strings.Contains(contentStr, "requests==2.28.0opentelemetry-sdk") {
		t.Error("dependencies should not be concatenated on the same line")
	}

	// Verify the total number of lines is correct
	// Should have: flask, requests, opentelemetry-api, opentelemetry-sdk = 4 lines
	expectedLines := 4
	if len(lines) != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, len(lines))
	}
}

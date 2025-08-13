package template

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/types"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/templates"
)

// fakeLogger captures logs for assertions
type fakeLogger struct{ logs []string }

func (f *fakeLogger) Logf(format string, args ...interface{}) {
	f.logs = append(f.logs, sprintf(format, args...))
}
func (f *fakeLogger) Log(msg string) { f.logs = append(f.logs, msg) }

// sprintf local helper to avoid importing fmt everywhere
func sprintf(format string, args ...interface{}) string { return fmt.Sprintf(format, args...) }

// fakeTemplateEngine captures calls
type fakeTemplateEngine struct {
	calls []struct {
		lang string
		data templates.TemplateData
	}
}

func (f *fakeTemplateEngine) GenerateInstructions(lang string, data templates.TemplateData) (string, error) {
	f.calls = append(f.calls, struct {
		lang string
		data templates.TemplateData
	}{lang: lang, data: data})
	return "CODE-" + lang, nil
}

// compile-time assertions
var _ TemplateRenderer = (*fakeTemplateEngine)(nil)

func TestGenerateCode_DryRun_GroupsByDirAndLanguageAndCallsDepsAndInjector(t *testing.T) {
	t.Helper()

	// Arrange
	flog := &fakeLogger{}
	fte := &fakeTemplateEngine{}
	strat := &TemplateGenerationStrategy{logger: flog, templateEngine: fte}

	tmpRoot := t.TempDir()

	req := types.GenerationRequest{
		CodebasePath: tmpRoot,
		Config:       types.StrategyConfig{DryRun: true},
		OTEL:         &types.OTELConfig{ServiceName: "override-name"},
	}

	// Two directories: root (csharp) and python subdir
	opps := []domain.Opportunity{
		{
			Type:          domain.OpportunityInstallComponent,
			ComponentType: domain.ComponentTypeInstrumentation,
			Component:     "aspnetcore",
			Language:      "csharp",
			FilePath:      "root",
		},
		{
			Type:          domain.OpportunityInstallComponent,
			ComponentType: domain.ComponentTypeInstrumentation,
			Component:     "flask",
			Language:      "python",
			FilePath:      "python",
		},
	}

	// Act
	if err := strat.GenerateCode(context.Background(), opps, req); err != nil {
		t.Fatalf("GenerateCode error: %v", err)
	}

	// Assert template engine calls
	if len(fte.calls) != 2 {
		t.Fatalf("expected 2 template calls, got %d", len(fte.calls))
	}
	// csharp normalized to dotnet
	foundDotnet := false
	foundPython := false
	for _, c := range fte.calls {
		if c.lang == "dotnet" {
			foundDotnet = true
			if c.data.ServiceName != "override-name" {
				t.Fatalf("expected ServiceName override to propagate, got %q", c.data.ServiceName)
			}
			if !c.data.InstallOTEL {
				t.Fatalf("expected InstallOTEL true when instrumentations present")
			}
		}
		if c.lang == "python" {
			foundPython = true
		}
	}
	if !foundDotnet || !foundPython {
		t.Fatalf("expected template calls for dotnet and python, got %+v", fte.calls)
	}

	// Assert dry-run logs include output paths
	rootOut := filepath.Join(tmpRoot, getOutputFilenameForLanguage("dotnet"))
	pyOut := filepath.Join(tmpRoot, "python", getOutputFilenameForLanguage("python"))
	joinedLogs := strings.Join(flog.logs, "\n")
	if !strings.Contains(joinedLogs, rootOut) || !strings.Contains(joinedLogs, pyOut) {
		t.Fatalf("expected logs to mention output paths %q and %q. Logs: %s", rootOut, pyOut, joinedLogs)
	}
	if !strings.Contains(joinedLogs, "CODE-dotnet") || !strings.Contains(joinedLogs, "CODE-python") {
		t.Fatalf("expected logs to include generated code content, got: %s", joinedLogs)
	}
}

func TestGenerateCode_NoOpportunities_NoWork(t *testing.T) {
	flog := &fakeLogger{}
	strat := &TemplateGenerationStrategy{logger: flog, templateEngine: &fakeTemplateEngine{}}
	req := types.GenerationRequest{CodebasePath: t.TempDir(), Config: types.StrategyConfig{DryRun: true}}
	if err := strat.GenerateCode(context.Background(), nil, req); err != nil {
		t.Fatalf("GenerateCode returned error: %v", err)
	}
	// Should log that no opportunities found
	joined := strings.Join(flog.logs, "\n")
	if !strings.Contains(joined, "No opportunities to process") {
		t.Fatalf("expected no opportunities log, got: %s", joined)
	}
}

func TestGenerateCode_FallbackLanguageDirectories_DryRun(t *testing.T) {
	flog := &fakeLogger{}
	fte := &fakeTemplateEngine{}
	strat := &TemplateGenerationStrategy{logger: flog, templateEngine: fte}

	root := t.TempDir()
	// Create a python subdir with a marker file to trigger fallback
	pyDir := filepath.Join(root, "python")
	if err := os.MkdirAll(pyDir, 0o755); err != nil {
		t.Fatalf("mkdir python: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pyDir, "requirements.txt"), []byte("opentelemetry-sdk\n"), 0o644); err != nil {
		t.Fatalf("write requirements: %v", err)
	}

	req := types.GenerationRequest{CodebasePath: root, Config: types.StrategyConfig{DryRun: true}}
	if err := strat.GenerateCode(context.Background(), nil, req); err != nil {
		t.Fatalf("GenerateCode error: %v", err)
	}

	// Expect at least one python template render due to fallback
	seenPython := false
	for _, c := range fte.calls {
		if c.lang == "python" {
			seenPython = true
		}
	}
	if !seenPython {
		t.Fatalf("expected fallback to trigger python generation, got calls: %+v", fte.calls)
	}
}

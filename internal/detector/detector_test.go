package detector

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/getlawrence/cli/internal/domain"
)

type fakeLanguage struct{}

func (f *fakeLanguage) Name() string { return "fake" }
func (f *fakeLanguage) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	return nil, nil
}
func (f *fakeLanguage) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	return nil, nil
}
func (f *fakeLanguage) GetFilePatterns() []string { return []string{"**/*.fake"} }

type errorLanguage struct{ fakeLanguage }

func (e *errorLanguage) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	return nil, errors.New("boom")
}

type noOpDetector struct{}

func (n *noOpDetector) ID() string                { return "noop" }
func (n *noOpDetector) Name() string              { return "noop" }
func (n *noOpDetector) Description() string       { return "noop" }
func (n *noOpDetector) Category() domain.Category { return domain.CategoryBestPractice }
func (n *noOpDetector) Languages() []string       { return []string{} }
func (n *noOpDetector) Detect(ctx context.Context, analysis *DirectoryAnalysis) ([]domain.Issue, error) {
	return nil, nil
}

func TestCalculateDirectoryPath(t *testing.T) {
	ca := NewCodebaseAnalyzer(nil, nil)
	root := "/path/to/root"
	if got := ca.calculateDirectoryPath(root, "root"); got != root {
		t.Fatalf("expected %s, got %s", root, got)
	}
	exp := filepath.Join(root, "svc")
	if got := ca.calculateDirectoryPath(root, "svc"); got != exp {
		t.Fatalf("expected %s, got %s", exp, got)
	}
}

func TestAnalyzeCodebase_NoLanguages(t *testing.T) {
	dir := t.TempDir()

	ca := NewCodebaseAnalyzer([]IssueDetector{&noOpDetector{}}, map[string]Language{
		"go": &fakeLanguage{},
	})

	// With empty dir and enry detection, expect an error: no languages detected
	_, err := ca.AnalyzeCodebase(context.Background(), dir)
	if err == nil {
		t.Fatalf("expected error when no languages detected")
	}
}

func TestProcessDirectory_ErrorFromPackages(t *testing.T) {
	dir := t.TempDir()
	// put a trivial file to satisfy walker
	_ = os.WriteFile(filepath.Join(dir, "main.fake"), []byte(""), 0o644)

	ca := NewCodebaseAnalyzer(nil, map[string]Language{"fake": &errorLanguage{}})
	_, err := ca.processDirectory(context.Background(), "root", dir, "fake", &errorLanguage{})
	if err == nil {
		t.Fatalf("expected error from package collection")
	}
}

package generator

import (
	"context"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/types"
	det "github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
)

// fakeLanguage implements detector.Language with static responses
type fakeLanguage struct{}

func (f *fakeLanguage) Name() string { return "go" }
func (f *fakeLanguage) GetOTelLibraries(ctx context.Context, rootPath string) ([]domain.Library, error) {
	return nil, nil
}
func (f *fakeLanguage) GetAllPackages(ctx context.Context, rootPath string) ([]domain.Package, error) {
	return nil, nil
}
func (f *fakeLanguage) GetFilePatterns() []string { return []string{"**/*.go"} }

func TestNewGenerator_DefaultsAndListings(t *testing.T) {
	ca := det.NewCodebaseAnalyzer(nil, map[string]det.Language{}, (*storage.Storage)(nil), &logger.StdoutLogger{})
	store, err := storage.NewStorage("test.db", &logger.StdoutLogger{})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	kb := knowledge.NewKnowledge(*store, &logger.StdoutLogger{})
	g, err := NewGenerator(ca, &logger.StdoutLogger{}, kb)
	if err != nil {
		t.Fatalf("NewGenerator error: %v", err)
	}

	if g.GetDefaultStrategy() != types.TemplateMode {
		t.Fatalf("expected default strategy TemplateMode")
	}

	templates := g.ListAvailableTemplates()
	if len(templates) == 0 {
		t.Fatalf("expected templates to be available")
	}

	strategies := g.ListAvailableStrategies()
	if _, ok := strategies[types.TemplateMode]; !ok {
		t.Fatalf("expected TemplateMode key in strategies map")
	}
	if _, ok := strategies[types.AgentMode]; !ok {
		t.Fatalf("expected AgentMode key in strategies map")
	}
}

func TestGenerator_ConvertIssuesToOpportunities(t *testing.T) {
	ca := det.NewCodebaseAnalyzer(nil, nil, (*storage.Storage)(nil), &logger.StdoutLogger{})
	store, err := storage.NewStorage("test.db", &logger.StdoutLogger{})
	if err != nil {
		t.Fatalf("Failed to create storage: %v", err)
	}
	kb := knowledge.NewKnowledge(*store, &logger.StdoutLogger{})
	g, err := NewGenerator(ca, &logger.StdoutLogger{}, kb)
	if err != nil {
		t.Fatalf("NewGenerator error: %v", err)
	}

	analysis := &det.Analysis{DirectoryAnalyses: map[string]*det.DirectoryAnalysis{
		"root": {Language: "go", Issues: []domain.Issue{{Category: domain.CategoryMissingOtel, Language: "go"}}},
	}}

	opps := g.convertIssuesToOpportunities(analysis)
	if len(opps) != 1 {
		t.Fatalf("expected 1 opportunity, got %d", len(opps))
	}
	if opps[0].Type != domain.OpportunityInstallOTEL {
		t.Fatalf("expected OpportunityInstallOTEL, got %s", opps[0].Type)
	}
	if opps[0].Language != "go" {
		t.Fatalf("expected language go, got %s", opps[0].Language)
	}

	filtered := g.filterByLanguage(opps, "go")
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered opportunity, got %d", len(filtered))
	}
}

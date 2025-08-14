package issues

import (
	"context"
	"testing"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
)

func TestMissingOTelDetector_Detect_NoLibraries_AddsIssue(t *testing.T) {
	det := NewMissingOTelDetector()
	dir := &detector.DirectoryAnalysis{Language: "go"}
	issues, err := det.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue, got %d", len(issues))
	}
	if issues[0].Category != domain.CategoryMissingOtel {
		t.Fatalf("unexpected category: %s", issues[0].Category)
	}
}

func TestMissingOTelDetector_hasOTELInitialization(t *testing.T) {
	det := NewMissingOTelDetector()
	cases := []struct {
		ctx string
		ok  bool
	}{
		{"... TracerProvider(...) ...", true},
		{"... set_tracer_provider ...", true},
		{"just some text without keywords", false},
	}
	for _, tc := range cases {
		if got := det.hasOTELInitialization(domain.EntryPoint{Context: tc.ctx}); got != tc.ok {
			t.Fatalf("hasOTELInitialization(%q) = %v, want %v", tc.ctx, got, tc.ok)
		}
	}
}

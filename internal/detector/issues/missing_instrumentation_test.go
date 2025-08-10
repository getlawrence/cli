package issues

import (
	"context"
	"testing"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
)

func TestMissingInstrumentationDetector_Detect_Basic(t *testing.T) {
	det := NewMissingInstrumentationDetector()
	dir := &detector.DirectoryAnalysis{
		Language: "python",
		Packages: []domain.Package{{Name: "flask", Language: "python"}},
		AvailableInstrumentations: []domain.InstrumentationInfo{{
			Package:     domain.Package{Name: "flask", Language: "python"},
			Language:    "python",
			IsAvailable: true,
		}},
	}
	issues, err := det.Detect(context.Background(), dir)
	if err != nil {
		t.Fatalf("Detect returned error: %v", err)
	}
	if len(issues) != 1 {
		t.Fatalf("expected 1 issue for missing instrumentation, got %d", len(issues))
	}
}

func TestMissingInstrumentationDetector_isPackageInstrumented(t *testing.T) {
	det := NewMissingInstrumentationDetector()
	pkg := domain.Package{Name: "gin-gonic/gin", Language: "go"}
	// Matching OTEL contrib pattern in libraries should mark as instrumented
	libs := []domain.Library{{Name: "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin"}}
	if !det.isPackageInstrumented(pkg, libs) {
		t.Fatalf("expected package to be considered instrumented by matching library")
	}
}

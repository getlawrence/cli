package languages

import (
	"testing"

	"github.com/getlawrence/cli/internal/domain"
)

func TestJavaScriptDetector_NameAndPatterns(t *testing.T) {
	d := NewJavaScriptDetector()
	if d.Name() != "javascript" {
		t.Fatalf("unexpected name: %s", d.Name())
	}
	patterns := d.GetFilePatterns()
	if len(patterns) == 0 {
		t.Fatalf("expected patterns")
	}
}

func TestJavaScriptDetector_IsThirdPartyAndDedup(t *testing.T) {
	d := NewJavaScriptDetector()

	if !d.isThirdParty("express") {
		t.Fatalf("express should be third-party")
	}
	if d.isThirdParty("./local") || d.isThirdParty("/abs/path") {
		t.Fatalf("relative/absolute paths should not be third-party")
	}

	libs := []domain.Library{
		{Name: "@opentelemetry/api", Version: "1"},
		{Name: "@opentelemetry/api", Version: "1"},
	}
	dedupLibs := d.deduplicateLibraries(libs)
	if len(dedupLibs) != 1 {
		t.Fatalf("expected 1 deduped lib, got %d", len(dedupLibs))
	}

	pkgs := []domain.Package{
		{Name: "express", Version: "4"},
		{Name: "express", Version: "4"},
	}
	dedupPkgs := d.deduplicatePackages(pkgs)
	if len(dedupPkgs) != 1 {
		t.Fatalf("expected 1 deduped pkg, got %d", len(dedupPkgs))
	}
}

package issues

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/internal/detector"
	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeBasedDetector uses the knowledge base to detect sophisticated issues
type KnowledgeBasedDetector struct {
	storage *storage.Storage
}

// NewKnowledgeBasedDetector creates a new knowledge-based issue detector
func NewKnowledgeBasedDetector() (*KnowledgeBasedDetector, error) {
	// Use the same database file as the knowledge update command
	storageClient, err := storage.NewStorage("knowledge.db")
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge storage: %w", err)
	}

	return &KnowledgeBasedDetector{
		storage: storageClient,
	}, nil
}

// ID returns a unique identifier for this detector
func (d *KnowledgeBasedDetector) ID() string {
	return "knowledge_based_detector"
}

// Name returns a human-readable name
func (d *KnowledgeBasedDetector) Name() string {
	return "Knowledge-Based Issue Detector"
}

// Description returns what this detector looks for
func (d *KnowledgeBasedDetector) Description() string {
	return "Detects issues using the OpenTelemetry knowledge base including outdated packages, breaking changes, and compatibility problems"
}

// Category returns the issue category
func (d *KnowledgeBasedDetector) Category() domain.Category {
	return domain.CategoryBestPractice
}

// Languages returns which languages this detector applies to (empty = all)
func (d *KnowledgeBasedDetector) Languages() []string {
	return []string{} // Applies to all languages
}

// Detect finds issues using the knowledge base
func (d *KnowledgeBasedDetector) Detect(ctx context.Context, analysis *detector.DirectoryAnalysis) ([]domain.Issue, error) {
	var issues []domain.Issue

	// Load knowledge base
	kb, err := d.storage.LoadKnowledgeBase("")
	if err != nil {
		// If we can't load the knowledge base, return no issues rather than failing
		return nil, nil
	}

	// Check each package for issues
	for _, pkg := range analysis.Packages {
		pkgIssues := d.detectPackageIssues(ctx, pkg, kb)
		issues = append(issues, pkgIssues...)
	}

	return issues, nil
}

// detectPackageIssues detects issues for a specific package
func (d *KnowledgeBasedDetector) detectPackageIssues(ctx context.Context, pkg domain.Package, kb *types.KnowledgeBase) []domain.Issue {
	var issues []domain.Issue

	// Find the package in the knowledge base
	component := d.storage.GetComponentByName(kb, pkg.Name)
	if component == nil {
		// Package not found in knowledge base - could be a custom or private package
		return issues
	}

	// Check for deprecated packages
	if component.Status == types.ComponentStatusDeprecated {
		issues = append(issues, domain.Issue{
			Title:       fmt.Sprintf("Package %s is deprecated", pkg.Name),
			Description: fmt.Sprintf("The package %s has been deprecated and may not receive updates or security patches.", pkg.Name),
			Category:    domain.CategoryDeprecated,
			Severity:    domain.SeverityWarning,
			Suggestion:  fmt.Sprintf("Consider migrating to a supported alternative or check the package documentation for migration guidance."),
			File:        pkg.PackageFile,
			References:  []string{component.Repository, component.DocumentationURL},
		})
	}

	// Check for experimental packages
	if component.Status == types.ComponentStatusExperimental {
		issues = append(issues, domain.Issue{
			Title:       fmt.Sprintf("Package %s is experimental", pkg.Name),
			Description: fmt.Sprintf("The package %s is marked as experimental and may have breaking changes or be removed in future versions.", pkg.Name),
			Category:    domain.CategoryBestPractice,
			Severity:    domain.SeverityInfo,
			Suggestion:  fmt.Sprintf("Consider the stability implications for production use. Check the package documentation for stability guarantees."),
			File:        pkg.PackageFile,
			References:  []string{component.Repository, component.DocumentationURL},
		})
	}

	// Check for breaking changes in the current version
	breakingChanges := d.getBreakingChangesForVersion(component, pkg.Version)
	if len(breakingChanges) > 0 {
		for _, breaking := range breakingChanges {
			issues = append(issues, domain.Issue{
				Title:       fmt.Sprintf("Breaking change in %s version %s", pkg.Name, breaking.Version),
				Description: breaking.Description,
				Category:    domain.CategoryBestPractice,
				Severity:    domain.SeverityWarning,
				Suggestion:  fmt.Sprintf("Review the breaking changes and update your code accordingly. Check the migration guide: %s", breaking.MigrationGuideURL),
				File:        pkg.PackageFile,
				References:  []string{breaking.MigrationGuideURL, component.Repository},
			})
		}
	}

	// Check for outdated versions
	latestVersion := d.getLatestStableVersion(component)
	if latestVersion != nil && latestVersion.Name != pkg.Version {
		issues = append(issues, domain.Issue{
			Title:       fmt.Sprintf("Package %s is outdated", pkg.Name),
			Description: fmt.Sprintf("You're using version %s, but the latest stable version is %s.", pkg.Version, latestVersion.Name),
			Category:    domain.CategoryBestPractice,
			Severity:    domain.SeverityWarning,
			Suggestion:  fmt.Sprintf("Consider updating to version %s for the latest features, bug fixes, and security updates.", latestVersion.Name),
			File:        pkg.PackageFile,
			References:  []string{component.Repository, latestVersion.ChangelogURL},
		})
	}

	// Check for compatibility issues
	compatibilityIssues := d.checkCompatibilityIssues(component, pkg)
	issues = append(issues, compatibilityIssues...)

	return issues
}

// getBreakingChangesForVersion returns breaking changes for a specific version
func (d *KnowledgeBasedDetector) getBreakingChangesForVersion(component *types.Component, version string) []types.BreakingChange {
	for _, ver := range component.Versions {
		if ver.Name == version {
			return ver.BreakingChanges
		}
	}
	return nil
}

// getLatestStableVersion returns the latest stable version of a component
func (d *KnowledgeBasedDetector) getLatestStableVersion(component *types.Component) *types.Version {
	for _, version := range component.Versions {
		if version.Status == types.VersionStatusLatest && !version.Deprecated {
			return &version
		}
	}
	return nil
}

// checkCompatibilityIssues checks for compatibility problems
func (d *KnowledgeBasedDetector) checkCompatibilityIssues(component *types.Component, pkg domain.Package) []domain.Issue {
	var issues []domain.Issue

	// Find the version we're using
	var currentVersion *types.Version
	for _, version := range component.Versions {
		if version.Name == pkg.Version {
			currentVersion = &version
			break
		}
	}

	if currentVersion == nil {
		return issues
	}

	// Check runtime version compatibility
	if currentVersion.MinRuntimeVersion != "" {
		issues = append(issues, domain.Issue{
			Title:       fmt.Sprintf("Runtime version compatibility check for %s", pkg.Name),
			Description: fmt.Sprintf("Package %s version %s requires runtime version %s or higher.", pkg.Name, pkg.Version, currentVersion.MinRuntimeVersion),
			Category:    domain.CategoryBestPractice,
			Severity:    domain.SeverityInfo,
			Suggestion:  "Verify that your runtime environment meets the minimum version requirements.",
			File:        pkg.PackageFile,
			References:  []string{component.Repository, component.DocumentationURL},
		})
	}

	// Check for incompatible dependencies
	if len(currentVersion.Compatible) > 0 {
		issues = append(issues, domain.Issue{
			Title:       fmt.Sprintf("Dependency compatibility for %s", pkg.Name),
			Description: fmt.Sprintf("Package %s version %s has specific compatibility requirements with other packages.", pkg.Name, pkg.Version),
			Category:    domain.CategoryBestPractice,
			Severity:    domain.SeverityInfo,
			Suggestion:  "Review the compatibility matrix and ensure all dependencies are compatible.",
			File:        pkg.PackageFile,
			References:  []string{component.Repository, component.DocumentationURL},
		})
	}

	return issues
}

// Close closes the underlying storage connection
func (d *KnowledgeBasedDetector) Close() error {
	return d.storage.Close()
}

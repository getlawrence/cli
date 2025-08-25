package matcher

import (
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/pkg/knowledge/client"
	kbtypes "github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeEnhancedMatcher extends the base matcher with knowledge base version resolution
type KnowledgeEnhancedMatcher struct {
	*PlanMatcher
	knowledgeClient *client.KnowledgeClient
}

// NewKnowledgeEnhancedMatcher creates a new knowledge-enhanced matcher
func NewKnowledgeEnhancedMatcher(knowledgeClient *client.KnowledgeClient) Matcher {
	return &KnowledgeEnhancedMatcher{
		PlanMatcher:     &PlanMatcher{},
		knowledgeClient: knowledgeClient,
	}
}

// Match computes missing dependencies with specific versions from the knowledge base
func (m *KnowledgeEnhancedMatcher) Match(existingDeps []string, plan types.InstallPlan, kb *client.KnowledgeClient) []string {
	// First get the basic missing packages
	basicMissing := m.PlanMatcher.Match(existingDeps, plan, kb)
	if len(basicMissing) == 0 {
		return nil
	}

	// Enhance with specific versions and filter out packages with invalid versions
	var enhancedMissing []string
	for _, pkg := range basicMissing {
		// Check if package already has a version specifier
		if hasVersionSpecifier(pkg) {
			enhancedMissing = append(enhancedMissing, pkg)
			continue
		}

		// Try to find the package in the knowledge base and get latest stable version
		if versionedPkg := m.getPackageWithVersion(pkg, plan.Language); versionedPkg != "" {
			enhancedMissing = append(enhancedMissing, versionedPkg)
		} else {
			// Check if this package has "unknown" version - if so, skip it entirely
			if m.hasUnknownVersion(pkg) {
				// Skip packages that exist in registry but not in package manager
				continue
			}
			// Fallback to package without version for packages not in knowledge base
			enhancedMissing = append(enhancedMissing, pkg)
		}
	}

	return enhancedMissing
}

// getPackageWithVersion finds a package in the knowledge base and returns it with the latest stable version
func (m *KnowledgeEnhancedMatcher) getPackageWithVersion(packageName, language string) string {
	// Find the component in the knowledge base
	component, err := m.knowledgeClient.GetComponentByName(packageName)
	if err != nil || component == nil {
		return ""
	}

	// Check if this component is for the right language
	if !strings.EqualFold(string(component.Language), language) {
		return ""
	}

	// Find the latest stable version
	latestVersion := m.GetLatestStableVersion(component)
	if latestVersion == nil {
		return ""
	}

	// Format package with version based on language
	return m.FormatPackageWithVersion(packageName, latestVersion.Name, language)
}

// GetLatestStableVersion returns the latest stable version of a component
func (m *KnowledgeEnhancedMatcher) GetLatestStableVersion(component *kbtypes.Component) *kbtypes.Version {
	// First, try to find a version marked as "latest"
	for _, version := range component.Versions {
		if version.Status == kbtypes.VersionStatusLatest && !version.Deprecated && version.Name != "unknown" {
			return &version
		}
	}

	// If no "latest" version found, find the most recent non-deprecated stable version
	var latestVersion *kbtypes.Version
	for _, version := range component.Versions {
		if version.Deprecated || version.Name == "unknown" {
			continue
		}

		// Skip experimental or alpha versions for stability
		if version.Status == kbtypes.VersionStatusAlpha || version.Status == kbtypes.VersionStatusBeta {
			continue
		}

		if latestVersion == nil || version.ReleaseDate.After(latestVersion.ReleaseDate) {
			latestVersion = &version
		}
	}

	return latestVersion
}

// FormatPackageWithVersion formats a package name with version based on language conventions
func (m *KnowledgeEnhancedMatcher) FormatPackageWithVersion(packageName, version, language string) string {
	switch strings.ToLower(language) {
	case "javascript", "typescript":
		// npm format: package@version
		return packageName + "@" + version
	case "python":
		// pip format: package==version
		return packageName + "==" + version
	case "go":
		// go mod format: package@version
		return packageName + "@" + version
	case "java":
		// maven format is handled differently, but for string representation
		return packageName + ":" + version
	case "csharp", "dotnet":
		// nuget format: package@version
		return packageName + "@" + version
	case "php":
		// composer format: package:version
		return packageName + ":" + version
	case "ruby":
		// gem format: package:version
		return packageName + ":" + version
	default:
		// Default to @ separator
		return packageName + "@" + version
	}
}

// hasVersionSpecifier checks if a package name already includes a version
func hasVersionSpecifier(pkg string) bool {
	// Check for common version specifiers
	versionSeparators := []string{"@", "==", ">=", "<=", ">", "<", "~=", "!=", ":"}

	for _, sep := range versionSeparators {
		if strings.Contains(pkg, sep) {
			// For scoped npm packages like @opentelemetry/api, ignore the first @
			if sep == "@" && strings.HasPrefix(pkg, "@") {
				// Check if there's another @ after the first one
				if strings.Count(pkg, "@") > 1 {
					return true
				}
			} else {
				return true
			}
		}
	}

	return false
}

// hasUnknownVersion checks if a package exists in the knowledge base but only has "unknown" versions
func (m *KnowledgeEnhancedMatcher) hasUnknownVersion(packageName string) bool {
	component, err := m.knowledgeClient.GetComponentByName(packageName)
	if err != nil || component == nil {
		return false
	}

	// Check if all versions are "unknown"
	for _, version := range component.Versions {
		if version.Name != "unknown" {
			return false
		}
	}

	// All versions are "unknown" - this package exists in registry but not in package manager
	return len(component.Versions) > 0
}

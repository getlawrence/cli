package matcher

import (
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/pkg/knowledge"
)

// Matcher computes required dependencies based on plan and existing deps
type Matcher interface {
	Match(existingDeps []string, plan types.InstallPlan, kb *knowledge.Knowledge) []string
}

// PlanMatcher implements Matcher using InstallPlan and prerequisites
type PlanMatcher struct{}

// NewPlanMatcher creates a new plan-based matcher
func NewPlanMatcher() Matcher {
	return &PlanMatcher{}
}

// Match computes missing dependencies based on the install plan
func (m *PlanMatcher) Match(existingDeps []string, plan types.InstallPlan, kb *knowledge.Knowledge) []string {
	// Build set of existing dependencies (normalized)
	existing := make(map[string]bool)
	for _, dep := range existingDeps {
		existing[normalizePackage(dep)] = true
	}

	// Collect all required packages
	required := make(map[string]string) // normalized -> actual

	// Add core packages if requested
	if plan.InstallOTEL {
		corePackages, err := kb.GetCorePackages(plan.Language)
		if err == nil {
			for _, pkg := range corePackages {
				required[normalizePackage(pkg)] = pkg
			}
		}
	}

	// Add component packages
	for compType, components := range plan.InstallComponents {
		// For instrumentation components, expand with prerequisites
		if compType == "instrumentation" {
			// Get prerequisite rules for this language
			rules, err := kb.GetPrerequisites(plan.Language)
			if err == nil {
				components = m.expandPrerequisites(components, rules)
			}
		}

		for _, comp := range components {
			pkg, err := kb.GetComponentPackage(plan.Language, compType, comp)
			if err == nil && pkg != "" {
				required[normalizePackage(pkg)] = pkg
			}
		}
	}

	// Compute missing packages
	var missing []string
	for norm, pkg := range required {
		if !existing[norm] {
			missing = append(missing, pkg)
		}
	}

	return missing
}

// expandPrerequisites applies prerequisite rules to expand instrumentation list
func (m *PlanMatcher) expandPrerequisites(instrumentations []string, rules []knowledge.PrerequisiteRule) []string {
	// Build set of current instrumentations
	instSet := make(map[string]bool)
	for _, inst := range instrumentations {
		instSet[inst] = true
	}

	// Apply each rule
	for _, rule := range rules {
		// Check if any "if" condition matches
		matches := false
		for _, ifInst := range rule.If {
			if instSet[ifInst] {
				matches = true
				break
			}
		}

		if !matches {
			continue
		}

		// Check if any "unless" condition is present
		skip := false
		for _, unlessInst := range rule.Unless {
			if instSet[unlessInst] {
				skip = true
				break
			}
		}

		if skip {
			continue
		}

		// Add required instrumentations
		for _, req := range rule.Requires {
			instSet[req] = true
		}
	}

	// Convert back to slice
	var result []string
	for inst := range instSet {
		result = append(result, inst)
	}
	return result
}

// normalizePackage normalizes a package name for comparison
func normalizePackage(pkg string) string {
	return strings.ToLower(strings.TrimSpace(pkg))
}

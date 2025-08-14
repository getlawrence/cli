package matcher

import (
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// Matcher computes required dependencies based on plan and existing deps
type Matcher interface {
	Match(existingDeps []string, plan types.InstallPlan, kb *knowledge.KnowledgeBase) []string
}

// PlanMatcher implements Matcher using InstallPlan and prerequisites
type PlanMatcher struct{}

// NewPlanMatcher creates a new plan-based matcher
func NewPlanMatcher() Matcher {
	return &PlanMatcher{}
}

// Match computes missing dependencies based on the install plan
func (m *PlanMatcher) Match(existingDeps []string, plan types.InstallPlan, kb *knowledge.KnowledgeBase) []string {
	// Build set of existing dependencies (normalized)
	existing := make(map[string]bool)
	for _, dep := range existingDeps {
		existing[normalizePackage(dep)] = true
	}

	// Collect all required packages
	required := make(map[string]string) // normalized -> actual

	// Add core packages if requested
	if plan.InstallOTEL {
		for _, pkg := range kb.GetCorePackages(plan.Language) {
			required[normalizePackage(pkg)] = pkg
		}
	}

	// Expand instrumentations with prerequisites
	instrumentations := m.expandPrerequisites(plan.InstallInstrumentations, kb.GetPrerequisites(plan.Language))

	// Add instrumentation packages
	for _, inst := range instrumentations {
		if pkg := kb.GetInstrumentationPackage(plan.Language, inst); pkg != "" {
			required[normalizePackage(pkg)] = pkg
		}
	}

	// Add component packages
	for compType, components := range plan.InstallComponents {
		for _, comp := range components {
			if pkg := kb.GetComponentPackage(plan.Language, compType, comp); pkg != "" {
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

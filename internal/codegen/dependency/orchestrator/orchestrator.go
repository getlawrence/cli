package orchestrator

import (
	"context"
	"fmt"

	"github.com/getlawrence/cli/internal/codegen/dependency/knowledge"
	"github.com/getlawrence/cli/internal/codegen/dependency/matcher"
	"github.com/getlawrence/cli/internal/codegen/dependency/registry"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
)

// Orchestrator coordinates scanning -> matching -> installing
type Orchestrator struct {
	registry         *registry.Registry
	matcher          matcher.Matcher
	kb               *knowledge.KnowledgeBase
	knowledgeStorage *storage.Storage
}

// New creates a new orchestrator
func New(registry *registry.Registry, kb *knowledge.KnowledgeBase) *Orchestrator {
	// Try to create knowledge-enhanced matcher
	knowledgeStorage, err := storage.NewStorage("knowledge.db")
	var matcherInstance matcher.Matcher
	if err == nil {
		// Use knowledge-enhanced matcher
		matcherInstance = matcher.NewKnowledgeEnhancedMatcher(knowledgeStorage)
	} else {
		// Fallback to basic matcher
		matcherInstance = matcher.NewPlanMatcher()
	}

	return &Orchestrator{
		registry:         registry,
		matcher:          matcherInstance,
		kb:               kb,
		knowledgeStorage: knowledgeStorage,
	}
}

// Run executes the dependency installation pipeline
func (o *Orchestrator) Run(ctx context.Context, projectPath string, plan types.InstallPlan, dryRun bool) ([]string, error) {
	// Get language-specific components
	scanner, err := o.registry.GetScanner(plan.Language)
	if err != nil {
		return nil, err
	}

	installer, err := o.registry.GetInstaller(plan.Language)
	if err != nil {
		return nil, err
	}

	// Detect if project is valid for this scanner
	if !scanner.Detect(projectPath) {
		return nil, fmt.Errorf("no dependency file found for %s project", plan.Language)
	}

	// Scan existing dependencies
	existing, err := scanner.Scan(projectPath)
	if err != nil {
		return nil, fmt.Errorf("scan dependencies: %w", err)
	}

	// Match against plan to find missing
	missing := o.matcher.Match(existing, plan, o.kb)
	if len(missing) == 0 {
		return nil, nil
	}

	// Install missing dependencies
	if err := installer.Install(ctx, projectPath, missing, dryRun); err != nil {
		return nil, fmt.Errorf("install dependencies: %w", err)
	}

	return missing, nil
}

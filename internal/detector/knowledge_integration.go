package detector

import (
	"context"
	"fmt"
	"strings"

	"github.com/getlawrence/cli/internal/domain"
	"github.com/getlawrence/cli/internal/logger"
	"github.com/getlawrence/cli/pkg/knowledge/storage"
	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// KnowledgeBasedInstrumentationService integrates the new knowledge base with the detector system
type KnowledgeBasedInstrumentationService struct {
	storage *storage.Storage
}

// NewKnowledgeBasedInstrumentationService creates a new knowledge-based instrumentation service
func NewKnowledgeBasedInstrumentationService(logger logger.Logger) (*KnowledgeBasedInstrumentationService, error) {
	storageClient, err := storage.NewStorage("knowledge.db", logger)
	if err != nil {
		return nil, fmt.Errorf("failed to create knowledge storage: %w", err)
	}

	return &KnowledgeBasedInstrumentationService{
		storage: storageClient,
	}, nil
}

// Close closes the underlying storage connection
func (s *KnowledgeBasedInstrumentationService) Close() error {
	return s.storage.Close()
}

// GetInstrumentation finds instrumentation information using the knowledge base
func (s *KnowledgeBasedInstrumentationService) GetInstrumentation(ctx context.Context, pkg domain.Package) (*domain.InstrumentationInfo, error) {
	// Try to find the package in the knowledge base
	component := s.storage.GetComponentByName(pkg.Name)
	if component == nil {
		// Package not found in knowledge base, return nil (no instrumentation available)
		return nil, nil
	}

	// Check if this is an instrumentation component
	if component.Type != types.ComponentTypeInstrumentation {
		return nil, nil
	}

	// Convert to domain.InstrumentationInfo
	info := &domain.InstrumentationInfo{
		Package:      pkg,
		Title:        component.Description,
		Description:  component.Description,
		RegistryType: string(component.Category),
		Language:     string(component.Language),
		Tags:         component.Tags,
		License:      component.License,
		CreatedAt:    component.LastUpdated.Format("2006-01-02"),
		IsFirstParty: component.SupportLevel == types.SupportLevelOfficial,
		IsAvailable:  true,
		RegistryURL:  component.RegistryURL,
	}

	// Set URLs
	info.URLs = domain.URLs{
		Repo: component.Repository,
	}

	// Add maintainers as authors
	for _, maintainer := range component.Maintainers {
		info.Authors = append(info.Authors, domain.Author{Name: maintainer})
	}

	return info, nil
}

// GetRecommendedInstrumentations returns recommended instrumentations for a given package
func (s *KnowledgeBasedInstrumentationService) GetRecommendedInstrumentations(ctx context.Context, pkg domain.Package) ([]domain.InstrumentationInfo, error) {
	var recommendations []domain.InstrumentationInfo

	// Query for instrumentations that might be relevant
	query := storage.Query{
		Language: string(convertLanguage(pkg.Language)),
		Type:     string(types.ComponentTypeInstrumentation),
		Name:     pkg.Name, // Partial match
	}

	result := s.storage.QueryKnowledgeBase(query)

	// Convert to domain.InstrumentationInfo
	for _, component := range result.Components {
		// Skip if this is the exact package we're looking for
		if strings.EqualFold(component.Name, pkg.Name) {
			continue
		}

		// Check if this instrumentation targets the framework/library we're using
		if s.isRelevantInstrumentation(component, pkg) {
			info := s.convertComponentToInstrumentationInfo(component, pkg)
			recommendations = append(recommendations, *info)
		}
	}

	return recommendations, nil
}

// GetCompatibleVersions returns compatible versions for a given package
func (s *KnowledgeBasedInstrumentationService) GetCompatibleVersions(ctx context.Context, pkg domain.Package) ([]types.CompatibleComponent, error) {
	component := s.storage.GetComponentByName(pkg.Name)
	if component == nil {
		return nil, nil
	}

	// Find the version that matches our package version
	for _, version := range component.Versions {
		if version.Name == pkg.Version {
			return version.Compatible, nil
		}
	}

	return nil, nil
}

// GetBreakingChanges returns breaking changes for a given package
func (s *KnowledgeBasedInstrumentationService) GetBreakingChanges(ctx context.Context, pkg domain.Package) ([]types.BreakingChange, error) {
	component := s.storage.GetComponentByName(pkg.Name)
	if component == nil {
		return nil, nil
	}

	var allBreakingChanges []types.BreakingChange
	for _, version := range component.Versions {
		allBreakingChanges = append(allBreakingChanges, version.BreakingChanges...)
	}

	return allBreakingChanges, nil
}

// GetLatestVersion returns the latest stable version of a package
func (s *KnowledgeBasedInstrumentationService) GetLatestVersion(ctx context.Context, pkg domain.Package) (*types.Version, error) {
	component := s.storage.GetComponentByName(pkg.Name)
	if component == nil {
		return nil, nil
	}

	// Find the latest stable version
	for _, version := range component.Versions {
		if version.Status == types.VersionStatusLatest && !version.Deprecated {
			return &version, nil
		}
	}

	return nil, nil
}

// isRelevantInstrumentation checks if an instrumentation is relevant to the given package
func (s *KnowledgeBasedInstrumentationService) isRelevantInstrumentation(component types.Component, pkg domain.Package) bool {
	// Check if the instrumentation targets the framework/library we're using
	for _, target := range component.InstrumentationTargets {
		if strings.Contains(strings.ToLower(target.Framework), strings.ToLower(pkg.Name)) {
			return true
		}
	}

	// Check tags for relevance
	for _, tag := range component.Tags {
		if strings.Contains(strings.ToLower(tag), strings.ToLower(pkg.Name)) {
			return true
		}
	}

	return false
}

// convertComponentToInstrumentationInfo converts a knowledge base component to domain.InstrumentationInfo
func (s *KnowledgeBasedInstrumentationService) convertComponentToInstrumentationInfo(component types.Component, pkg domain.Package) *domain.InstrumentationInfo {
	info := &domain.InstrumentationInfo{
		Package:      pkg,
		Title:        component.Description,
		Description:  component.Description,
		RegistryType: string(component.Category),
		Language:     string(component.Language),
		Tags:         component.Tags,
		License:      component.License,
		CreatedAt:    component.LastUpdated.Format("2006-01-02"),
		IsFirstParty: component.SupportLevel == types.SupportLevelOfficial,
		IsAvailable:  true,
		RegistryURL:  component.RegistryURL,
	}

	// Set URLs
	info.URLs = domain.URLs{
		Repo: component.Repository,
	}

	// Add maintainers as authors
	for _, maintainer := range component.Maintainers {
		info.Authors = append(info.Authors, domain.Author{Name: maintainer})
	}

	return info
}

// convertLanguage converts domain language to knowledge base language
func convertLanguage(lang string) types.ComponentLanguage {
	switch strings.ToLower(lang) {
	case "javascript", "js", "typescript", "ts":
		return types.ComponentLanguageJavaScript
	case "python", "py":
		return types.ComponentLanguagePython
	case "go":
		return types.ComponentLanguageGo
	case "java":
		return types.ComponentLanguageJava
	case "csharp", "c#", "dotnet":
		return types.ComponentLanguageCSharp
	case "php":
		return types.ComponentLanguagePHP
	case "ruby":
		return types.ComponentLanguageRuby
	default:
		return types.ComponentLanguageJavaScript // Default fallback
	}
}

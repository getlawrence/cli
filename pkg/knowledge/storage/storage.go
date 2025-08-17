package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/getlawrence/cli/pkg/knowledge/types"
)

// Storage represents the knowledge base storage interface
type Storage struct {
	basePath string
}

// NewStorage creates a new storage instance
func NewStorage(basePath string) *Storage {
	return &Storage{
		basePath: basePath,
	}
}

// SaveKnowledgeBase saves the knowledge base to a JSON file
func (s *Storage) SaveKnowledgeBase(kb *types.KnowledgeBase, filename string) error {
	// Ensure the directory exists
	dir := filepath.Dir(filename)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Create a backup if the file already exists
	if _, err := os.Stat(filename); err == nil {
		backupName := fmt.Sprintf("%s.backup.%s", filename, time.Now().Format("20060102-150405"))
		if err := os.Rename(filename, backupName); err != nil {
			return fmt.Errorf("failed to create backup: %w", err)
		}
	}

	// Marshal with pretty printing
	data, err := json.MarshalIndent(kb, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal knowledge base: %w", err)
	}

	// Write to file
	if err := os.WriteFile(filename, data, 0644); err != nil {
		return fmt.Errorf("failed to write knowledge base: %w", err)
	}

	return nil
}

// LoadKnowledgeBase loads the knowledge base from a JSON file
func (s *Storage) LoadKnowledgeBase(filename string) (*types.KnowledgeBase, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read knowledge base: %w", err)
	}

	var kb types.KnowledgeBase
	if err := json.Unmarshal(data, &kb); err != nil {
		return nil, fmt.Errorf("failed to unmarshal knowledge base: %w", err)
	}

	return &kb, nil
}

// Query represents a query against the knowledge base
type Query struct {
	Language    string
	Type        string
	Category    string
	Status      string
	SupportLevel string
	Name        string
	Version     string
	MinDate     time.Time
	MaxDate     time.Time
	Tags        []string
	Maintainers []string
	Framework   string // For instrumentation targets
}

// QueryResult represents the result of a query
type QueryResult struct {
	Components []types.Component
	Total      int
	Query      Query
}

// QueryKnowledgeBase queries the knowledge base based on criteria
func (s *Storage) QueryKnowledgeBase(kb *types.KnowledgeBase, query Query) *QueryResult {
	var results []types.Component

	for _, component := range kb.Components {
		if s.matchesQuery(component, query) {
			results = append(results, component)
		}
	}

	return &QueryResult{
		Components: results,
		Total:      len(results),
		Query:      query,
	}
}

// matchesQuery checks if a component matches the query criteria
func (s *Storage) matchesQuery(component types.Component, query Query) bool {
	// Language filter
	if query.Language != "" && string(component.Language) != query.Language {
		return false
	}

	// Type filter
	if query.Type != "" && string(component.Type) != query.Type {
		return false
	}

	// Category filter
	if query.Category != "" && string(component.Category) != query.Category {
		return false
	}

	// Component status filter
	if query.Status != "" && string(component.Status) != query.Status {
		return false
	}

	// Support level filter
	if query.SupportLevel != "" && string(component.SupportLevel) != query.SupportLevel {
		return false
	}

	// Name filter (partial match)
	if query.Name != "" && !strings.Contains(strings.ToLower(component.Name), strings.ToLower(query.Name)) {
		return false
	}

	// Version filter
	if query.Version != "" {
		found := false
		for _, version := range component.Versions {
			if version.Name == query.Version {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Version status filter (for backward compatibility)
	// Note: This conflicts with component status, so we need to handle it carefully
	if query.Status != "" {
		// First check if it's a component status
		if string(component.Status) == query.Status {
			// Component status matches, continue
		} else {
			// Check version status as fallback
			found := false
			for _, version := range component.Versions {
				if string(version.Status) == query.Status {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	// Framework filter for instrumentation targets
	if query.Framework != "" {
		found := false
		for _, target := range component.InstrumentationTargets {
			if strings.Contains(strings.ToLower(target.Framework), strings.ToLower(query.Framework)) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Date range filter
	if !query.MinDate.IsZero() || !query.MaxDate.IsZero() {
		componentDate := component.LastUpdated
		if !query.MinDate.IsZero() && componentDate.Before(query.MinDate) {
			return false
		}
		if !query.MaxDate.IsZero() && componentDate.After(query.MaxDate) {
			return false
		}
	}

	// Tags filter
	if len(query.Tags) > 0 {
		found := false
		for _, queryTag := range query.Tags {
			for _, componentTag := range component.Tags {
				if componentTag == queryTag {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	// Maintainers filter
	if len(query.Maintainers) > 0 {
		found := false
		for _, queryMaintainer := range query.Maintainers {
			for _, componentMaintainer := range component.Maintainers {
				if componentMaintainer == queryMaintainer {
					found = true
					break
				}
			}
			if found {
				break
			}
		}
		if !found {
			return false
		}
	}

	return true
}

// GetComponentsByType returns all components of a specific type
func (s *Storage) GetComponentsByType(kb *types.KnowledgeBase, componentType types.ComponentType) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.Type == componentType {
			results = append(results, component)
		}
	}
	return results
}

// GetComponentsByLanguage returns all components for a specific language
func (s *Storage) GetComponentsByLanguage(kb *types.KnowledgeBase, language types.ComponentLanguage) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.Language == language {
			results = append(results, component)
		}
	}
	return results
}

// GetLatestVersions returns the latest version of each component
func (s *Storage) GetLatestVersions(kb *types.KnowledgeBase) map[string]types.Version {
	latestVersions := make(map[string]types.Version)

	for _, component := range kb.Components {
		var latestVersion *types.Version

		for _, version := range component.Versions {
			if latestVersion == nil || version.ReleaseDate.After(latestVersion.ReleaseDate) {
				latestVersion = &version
			}
		}

		if latestVersion != nil {
			latestVersions[component.Name] = *latestVersion
		}
	}

	return latestVersions
}

// GetComponentByName returns a component by name
func (s *Storage) GetComponentByName(kb *types.KnowledgeBase, name string) *types.Component {
	for _, component := range kb.Components {
		if component.Name == name {
			return &component
		}
	}
	return nil
}

// GetComponentsByCategory returns all components of a specific category
func (s *Storage) GetComponentsByCategory(kb *types.KnowledgeBase, category types.ComponentCategory) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.Category == category {
			results = append(results, component)
		}
	}
	return results
}

// GetComponentsByStatus returns all components with a specific status
func (s *Storage) GetComponentsByStatus(kb *types.KnowledgeBase, status types.ComponentStatus) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.Status == status {
			results = append(results, component)
		}
	}
	return results
}

// GetComponentsBySupportLevel returns all components with a specific support level
func (s *Storage) GetComponentsBySupportLevel(kb *types.KnowledgeBase, supportLevel types.SupportLevel) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.SupportLevel == supportLevel {
			results = append(results, component)
		}
	}
	return results
}

// GetInstrumentationsByFramework returns all instrumentations for a specific framework
func (s *Storage) GetInstrumentationsByFramework(kb *types.KnowledgeBase, framework string) []types.Component {
	var results []types.Component
	for _, component := range kb.Components {
		if component.Type == types.ComponentTypeInstrumentation {
			for _, target := range component.InstrumentationTargets {
				if strings.Contains(strings.ToLower(target.Framework), strings.ToLower(framework)) {
					results = append(results, component)
					break
				}
			}
		}
	}
	return results
}

// GetCompatibleVersions returns compatible versions for a given component and version
func (s *Storage) GetCompatibleVersions(kb *types.KnowledgeBase, componentName, version string) []types.CompatibleComponent {
	component := s.GetComponentByName(kb, componentName)
	if component == nil {
		return nil
	}

	for _, v := range component.Versions {
		if v.Name == version {
			return v.Compatible
		}
	}
	return nil
}

// GetBreakingChanges returns breaking changes for a given component
func (s *Storage) GetBreakingChanges(kb *types.KnowledgeBase, componentName string) []types.BreakingChange {
	component := s.GetComponentByName(kb, componentName)
	if component == nil {
		return nil
	}

	var allBreakingChanges []types.BreakingChange
	for _, version := range component.Versions {
		allBreakingChanges = append(allBreakingChanges, version.BreakingChanges...)
	}
	return allBreakingChanges
}

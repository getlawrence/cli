package types

import "time"

// ComponentType represents the type of OpenTelemetry component
type ComponentType string

const (
	ComponentTypeAPI              ComponentType = "API"
	ComponentTypeSDK              ComponentType = "SDK"
	ComponentTypeInstrumentation  ComponentType = "Instrumentation"
	ComponentTypeExporter         ComponentType = "Exporter"
	ComponentTypePropagator       ComponentType = "Propagator"
	ComponentTypeSampler          ComponentType = "Sampler"
	ComponentTypeProcessor        ComponentType = "Processor"
	ComponentTypeResource         ComponentType = "Resource"
	ComponentTypeResourceDetector ComponentType = "ResourceDetector"
)

// ComponentLanguage represents the programming language
type ComponentLanguage string

const (
	ComponentLanguageJavaScript ComponentLanguage = "javascript"
	ComponentLanguageGo         ComponentLanguage = "go"
	ComponentLanguagePython     ComponentLanguage = "python"
	ComponentLanguageJava       ComponentLanguage = "java"
	ComponentLanguageCSharp     ComponentLanguage = "csharp"
	ComponentLanguagePHP        ComponentLanguage = "php"
	ComponentLanguageRuby       ComponentLanguage = "ruby"
)

// ComponentCategory represents the category of a component
type ComponentCategory string

const (
	ComponentCategoryStableSDK    ComponentCategory = "STABLE_SDK"
	ComponentCategoryExperimental ComponentCategory = "EXPERIMENTAL"
	ComponentCategoryAPI          ComponentCategory = "API"
	ComponentCategoryCore         ComponentCategory = "CORE"
	ComponentCategoryContrib      ComponentCategory = "CONTRIB"
)

// ComponentStatus represents the status of a component
type ComponentStatus string

const (
	ComponentStatusStable       ComponentStatus = "stable"
	ComponentStatusExperimental ComponentStatus = "experimental"
	ComponentStatusDeprecated   ComponentStatus = "deprecated"
	ComponentStatusBeta         ComponentStatus = "beta"
	ComponentStatusAlpha        ComponentStatus = "alpha"
)

// SupportLevel represents the support level of a component
type SupportLevel string

const (
	SupportLevelOfficial  SupportLevel = "official"
	SupportLevelCommunity SupportLevel = "community"
	SupportLevelVendor    SupportLevel = "vendor"
)

// VersionStatus represents the status of a version
type VersionStatus string

const (
	VersionStatusLatest     VersionStatus = "latest"
	VersionStatusStable     VersionStatus = "stable"
	VersionStatusBeta       VersionStatus = "beta"
	VersionStatusAlpha      VersionStatus = "alpha"
	VersionStatusDeprecated VersionStatus = "deprecated"
)

// Dependency represents a package dependency
type Dependency struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Type    string `json:"type,omitempty"` // peer, dev, optional, etc.
}

// CompatibleComponent represents a compatible component and version
type CompatibleComponent struct {
	Name     string            `json:"name"`
	Version  string            `json:"version"`
	Category ComponentCategory `json:"category,omitempty"`
	Type     ComponentType     `json:"type,omitempty"`
}

// BreakingChange represents a breaking change in a version
type BreakingChange struct {
	Version           string   `json:"version"`
	Description       string   `json:"description"`
	MigrationGuideURL string   `json:"migration_guide_url,omitempty"`
	AffectedFeatures  []string `json:"affected_features,omitempty"`
	Severity          string   `json:"severity,omitempty"` // high, medium, low
}

// InstrumentationTarget represents a target framework/library for instrumentation
type InstrumentationTarget struct {
	Framework    string `json:"framework"`
	VersionRange string `json:"version_range"`
	MinVersion   string `json:"min_version,omitempty"`
	MaxVersion   string `json:"max_version,omitempty"`
	Notes        string `json:"notes,omitempty"`
}

// Version represents a specific version of a component
type Version struct {
	Name                string                 `json:"name"`
	ReleaseDate         time.Time              `json:"release_date"`
	Dependencies        map[string]Dependency  `json:"dependencies,omitempty"`
	MinRuntimeVersion   string                 `json:"min_runtime_version,omitempty"`
	MaxRuntimeVersion   string                 `json:"max_runtime_version,omitempty"`
	Status              VersionStatus          `json:"status,omitempty"`
	Deprecated          bool                   `json:"deprecated,omitempty"`
	BreakingChanges     []BreakingChange       `json:"breaking_changes,omitempty"`
	Metadata            map[string]interface{} `json:"metadata,omitempty"`
	RegistryURL         string                 `json:"registry_url,omitempty"`
	NPMURL              string                 `json:"npm_url,omitempty"`
	GitHubURL           string                 `json:"github_url,omitempty"`
	ChangelogURL        string                 `json:"changelog_url,omitempty"`
	Changelog           string                 `json:"changelog,omitempty"`
	CoreVersion         string                 `json:"core_version,omitempty"`
	ExperimentalVersion string                 `json:"experimental_version,omitempty"`
	Compatible          []CompatibleComponent  `json:"compatible,omitempty"`
}

// Component represents an OpenTelemetry component
type Component struct {
	Name                   string                  `json:"name"`
	Type                   ComponentType           `json:"type"`
	Category               ComponentCategory       `json:"category,omitempty"`
	Status                 ComponentStatus         `json:"status,omitempty"`
	SupportLevel           SupportLevel            `json:"support_level,omitempty"`
	Language               ComponentLanguage       `json:"language"`
	Description            string                  `json:"description,omitempty"`
	Repository             string                  `json:"repository"`
	RegistryURL            string                  `json:"registry_url,omitempty"`
	Homepage               string                  `json:"homepage,omitempty"`
	Versions               []Version               `json:"versions"`
	Tags                   []string                `json:"tags,omitempty"`
	Maintainers            []string                `json:"maintainers,omitempty"`
	License                string                  `json:"license,omitempty"`
	LastUpdated            time.Time               `json:"last_updated"`
	InstrumentationTargets []InstrumentationTarget `json:"instrumentation_targets,omitempty"`
	DocumentationURL       string                  `json:"documentation_url,omitempty"`
	ExamplesURL            string                  `json:"examples_url,omitempty"`
	MigrationGuideURL      string                  `json:"migration_guide_url,omitempty"`
}

// KnowledgeBase represents the complete knowledge base
type KnowledgeBase struct {
	SchemaVersion string                 `json:"schema_version"`
	GeneratedAt   time.Time              `json:"generated_at"`
	Components    []Component            `json:"components"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
	Statistics    Statistics             `json:"statistics"`
}

// Statistics provides summary information about the knowledge base
type Statistics struct {
	TotalComponents int                    `json:"total_components"`
	ByLanguage      map[string]int         `json:"by_language"`
	ByType          map[string]int         `json:"by_type"`
	ByCategory      map[string]int         `json:"by_category"`
	ByStatus        map[string]int         `json:"by_status"`
	BySupportLevel  map[string]int         `json:"by_support_level"`
	TotalVersions   int                    `json:"total_versions"`
	LastUpdate      time.Time              `json:"last_update"`
	Source          string                 `json:"source"`
	Additional      map[string]interface{} `json:"additional,omitempty"`
}

package domain

// Issue represents a detected problem or recommendation
type Issue struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Category    Category `json:"category"`
	Language    string   `json:"language,omitempty"`
	File        string   `json:"file,omitempty"`
	Line        int      `json:"line,omitempty"`
	Column      int      `json:"column,omitempty"`
	Suggestion  string   `json:"suggestion,omitempty"`
	References  []string `json:"references,omitempty"`
}

// Severity levels for issues
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Category represents the type of issue
type Category string

const (
	CategoryMissingOtel     Category = "missing_otel"
	CategoryConfiguration   Category = "configuration"
	CategoryInstrumentation Category = "instrumentation"
	CategoryPerformance     Category = "performance"
	CategorySecurity        Category = "security"
	CategoryBestPractice    Category = "best_practice"
	CategoryDeprecated      Category = "deprecated"
)

// OpportunityType represents different types of instrumentation opportunities
type OpportunityType string

const (
	OpportunityInstallOTEL      OpportunityType = "install_otel"
	OpportunityInstallComponent OpportunityType = "install_component"
	OpportunityRemoveComponent  OpportunityType = "remove_component"
)

// ComponentType represents different types of OTEL components
type ComponentType string

const (
	ComponentTypeInstrumentation ComponentType = "instrumentation"
	ComponentTypeSDK             ComponentType = "sdk"
	ComponentTypePropagator      ComponentType = "propagator"
	ComponentTypeExporter        ComponentType = "exporter"
)

// Opportunity represents an instrumentation opportunity in the codebase
type Opportunity struct {
	Type          OpportunityType `json:"type"`
	Language      string          `json:"language"`
	Framework     string          `json:"framework"`
	ComponentType ComponentType   `json:"componentType"`
	Component     string          `json:"component"`
	FilePath      string          `json:"file_path"`
	Suggestion    string          `json:"suggestion"`
	Issue         *Issue          `json:"issue,omitempty"`
}

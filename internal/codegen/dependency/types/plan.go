package types

import "context"

// Dependency represents a package dependency that needs to be added
type Dependency struct {
	Name        string `json:"name"`        // Package name
	Version     string `json:"version"`     // Version constraint (optional)
	Language    string `json:"language"`    // Programming language
	ImportPath  string `json:"import_path"` // Import path/module name
	Category    string `json:"category"`    // Category: core, instrumentation, exporter, etc.
	Description string `json:"description"` // Human-readable description
	Required    bool   `json:"required"`    // Whether this dependency is required or optional
}

// InstallPlan represents the installation request from the generator
type InstallPlan struct {
	Language                string
	InstallOTEL             bool
	InstallInstrumentations []string
	InstallComponents       map[string][]string // componentType -> []componentName
}

// Commander abstracts command execution for testing
type Commander interface {
	LookPath(name string) (string, error)
	Run(ctx context.Context, name string, args []string, dir string) (output string, err error)
}

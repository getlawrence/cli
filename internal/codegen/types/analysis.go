package types

// FileAnalysis contains the analysis results for a source file
type FileAnalysis struct {
	Language        string                    `json:"language"`
	FilePath        string                    `json:"file_path"`
	HasOTELImports  bool                      `json:"has_otel_imports"`
	HasOTELSetup    bool                      `json:"has_otel_setup"`
	EntryPoints     []EntryPointInfo          `json:"entry_points"`
	ImportLocations []InsertionPoint          `json:"import_locations"`
	FunctionBodies  map[string]InsertionPoint `json:"function_bodies"`
	ExistingImports map[string]bool           `json:"existing_imports"`
}

// EntryPointInfo contains information about an entry point in the file
type EntryPointInfo struct {
	Name         string         `json:"name"`
	LineNumber   uint32         `json:"line_number"`
	Column       uint32         `json:"column"`
	BodyStart    InsertionPoint `json:"body_start"`
	BodyEnd      InsertionPoint `json:"body_end"`
	HasOTELSetup bool           `json:"has_otel_setup"`
}

// InsertionPoint represents a location where code can be inserted
type InsertionPoint struct {
	LineNumber uint32 `json:"line_number"`
	Column     uint32 `json:"column"`
	Context    string `json:"context"`
	Priority   int    `json:"priority"` // Higher priority = better insertion point
}

// OperationsData contains the analysis of opportunities organized by operation type
type OperationsData struct {
	InstallOTEL             bool                `json:"install_otel"`             // Whether OTEL needs to be installed
	InstallInstrumentations []string            `json:"install_instrumentations"` // Instrumentations to install
	InstallComponents       map[string][]string `json:"install_components"`       // Components to install by type (sdk, propagator, exporter)
	RemoveComponents        map[string][]string `json:"remove_components"`        // Components to remove by type
}

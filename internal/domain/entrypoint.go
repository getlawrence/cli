package domain

// EntryPoint represents an entry point in the codebase
type EntryPoint struct {
	FilePath     string  `json:"file_path"`
	Language     string  `json:"language"`
	FunctionName string  `json:"function_name"`
	LineNumber   uint32  `json:"line_number"`
	Column       uint32  `json:"column"`
	NodeType     string  `json:"node_type"`
	Confidence   float64 `json:"confidence"`
	Context      string  `json:"context"`
}

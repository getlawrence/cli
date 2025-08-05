package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config represents the Lawrence configuration
type Config struct {
	// Analysis settings
	Analysis AnalysisConfig `json:"analysis" yaml:"analysis"`

	// Output settings
	Output OutputConfig `json:"output" yaml:"output"`

	// Custom detector configurations
	CustomDetectors []CustomDetectorConfig `json:"custom_detectors" yaml:"custom_detectors"`

	// Language-specific settings
	Languages map[string]LanguageConfig `json:"languages" yaml:"languages"`
}

// AnalysisConfig contains analysis-specific settings
type AnalysisConfig struct {
	// Paths to exclude from analysis
	ExcludePaths []string `json:"exclude_paths" yaml:"exclude_paths"`

	// Maximum depth for directory traversal
	MaxDepth int `json:"max_depth" yaml:"max_depth"`

	// Whether to follow symbolic links
	FollowSymlinks bool `json:"follow_symlinks" yaml:"follow_symlinks"`

	// Severity threshold for reporting issues
	MinSeverity string `json:"min_severity" yaml:"min_severity"`
}

// OutputConfig contains output formatting settings
type OutputConfig struct {
	// Default output format
	Format string `json:"format" yaml:"format"`

	// Whether to show detailed information by default
	Detailed bool `json:"detailed" yaml:"detailed"`

	// Whether to colorize output
	Color bool `json:"color" yaml:"color"`
}

// CustomDetectorConfig allows users to define custom issue patterns
type CustomDetectorConfig struct {
	// Unique identifier for the detector
	ID string `json:"id" yaml:"id"`

	// Human-readable name
	Name string `json:"name" yaml:"name"`

	// Description of what this detector looks for
	Description string `json:"description" yaml:"description"`

	// Issue category
	Category string `json:"category" yaml:"category"`

	// Applicable languages (empty = all)
	Languages []string `json:"languages" yaml:"languages"`

	// File patterns to search
	FilePatterns []string `json:"file_patterns" yaml:"file_patterns"`

	// Search patterns
	Patterns []PatternConfig `json:"patterns" yaml:"patterns"`

	// Issue severity
	Severity string `json:"severity" yaml:"severity"`

	// Suggestion for fixing the issue
	Suggestion string `json:"suggestion" yaml:"suggestion"`

	// Reference links
	References []string `json:"references" yaml:"references"`
}

// PatternConfig defines a search pattern
type PatternConfig struct {
	// Type of pattern (regex, literal, etc.)
	Type string `json:"type" yaml:"type"`

	// The pattern to search for
	Pattern string `json:"pattern" yaml:"pattern"`

	// Whether pattern is case sensitive
	CaseSensitive bool `json:"case_sensitive" yaml:"case_sensitive"`

	// Whether to match whole words only
	WholeWord bool `json:"whole_word" yaml:"whole_word"`
}

// LanguageConfig contains language-specific settings
type LanguageConfig struct {
	// Whether this language is enabled
	Enabled bool `json:"enabled" yaml:"enabled"`

	// Additional file patterns for this language
	FilePatterns []string `json:"file_patterns" yaml:"file_patterns"`

	// Package manager files to check
	PackageFiles []string `json:"package_files" yaml:"package_files"`

	// Known OpenTelemetry library patterns
	OTelPatterns []string `json:"otel_patterns" yaml:"otel_patterns"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() *Config {
	return &Config{
		Analysis: AnalysisConfig{
			ExcludePaths: []string{
				".git",
				"node_modules",
				"vendor",
				"__pycache__",
				".venv",
				"venv",
				"env",
				"build",
				"dist",
				"target",
			},
			MaxDepth:       10,
			FollowSymlinks: false,
			MinSeverity:    "info",
		},
		Output: OutputConfig{
			Format:   "text",
			Detailed: false,
			Color:    true,
		},
		CustomDetectors: []CustomDetectorConfig{},
		Languages: map[string]LanguageConfig{
			"go": {
				Enabled:      true,
				FilePatterns: []string{"**/*.go"},
				PackageFiles: []string{"go.mod", "go.sum"},
				OTelPatterns: []string{"go.opentelemetry.io/*"},
			},
			"python": {
				Enabled:      true,
				FilePatterns: []string{"**/*.py"},
				PackageFiles: []string{"requirements.txt", "pyproject.toml", "setup.py", "Pipfile"},
				OTelPatterns: []string{"opentelemetry*"},
			},
		},
	}
}

// LoadConfig loads configuration from a file
func LoadConfig(configPath string) (*Config, error) {
	// Start with default config
	config := DefaultConfig()

	// If no config file specified, try to find one
	if configPath == "" {
		configPath = findConfigFile()
	}

	// If still no config file, return default
	if configPath == "" {
		return config, nil
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse JSON (for now, could add YAML support later)
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// SaveConfig saves configuration to a file
func SaveConfig(config *Config, configPath string) error {
	// Create directory if it doesn't exist
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(configPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// findConfigFile looks for config files in common locations
func findConfigFile() string {
	// Current directory
	candidates := []string{
		".lawrence.json",
		".lawrence.yaml",
		".lawrence.yml",
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	// Home directory
	homeDir, err := os.UserHomeDir()
	if err == nil {
		homeCandidates := []string{
			filepath.Join(homeDir, ".lawrence.json"),
			filepath.Join(homeDir, ".lawrence.yaml"),
			filepath.Join(homeDir, ".lawrence.yml"),
		}

		for _, candidate := range homeCandidates {
			if _, err := os.Stat(candidate); err == nil {
				return candidate
			}
		}
	}

	return ""
}

// GetConfigPath returns the config file path to use
func GetConfigPath(explicitPath string) string {
	if explicitPath != "" {
		return explicitPath
	}

	found := findConfigFile()
	if found != "" {
		return found
	}

	// Default location
	homeDir, err := os.UserHomeDir()
	if err == nil {
		return filepath.Join(homeDir, ".lawrence.json")
	}

	return ".lawrence.json"
}

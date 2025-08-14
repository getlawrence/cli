package registry

import (
	"fmt"

	"github.com/getlawrence/cli/internal/codegen/dependency/installer"
	"github.com/getlawrence/cli/internal/codegen/dependency/scanner"
	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// Registry manages language-specific components
type Registry struct {
	scanners   map[string]scanner.Scanner
	installers map[string]installer.Installer
}

// New creates a new registry with all language components
func New(commander types.Commander) *Registry {
	return &Registry{
		scanners: map[string]scanner.Scanner{
			"go":         scanner.NewGoModScanner(),
			"javascript": scanner.NewNpmScanner(),
			"python":     scanner.NewPipScanner(),
			"ruby":       scanner.NewGemfileScanner(),
			"php":        scanner.NewComposerScanner(),
			"java":       scanner.NewMavenScanner(),
			"csharp":     scanner.NewCsprojScanner(),
			"dotnet":     scanner.NewCsprojScanner(),
		},
		installers: map[string]installer.Installer{
			"go":         installer.NewGoInstaller(commander),
			"javascript": installer.NewNpmInstaller(commander),
			"python":     installer.NewPipInstaller(commander),
			"ruby":       installer.NewBundleInstaller(commander),
			"php":        installer.NewComposerInstaller(commander),
			"java":       installer.NewMavenInstaller(commander),
			"csharp":     installer.NewDotNetInstaller(commander),
			"dotnet":     installer.NewDotNetInstaller(commander),
		},
	}
}

// GetScanner returns scanner for a language
func (r *Registry) GetScanner(language string) (scanner.Scanner, error) {
	s, ok := r.scanners[language]
	if !ok {
		return nil, fmt.Errorf("no scanner for language: %s", language)
	}
	return s, nil
}

// GetInstaller returns installer for a language
func (r *Registry) GetInstaller(language string) (installer.Installer, error) {
	i, ok := r.installers[language]
	if !ok {
		return nil, fmt.Errorf("no installer for language: %s", language)
	}
	return i, nil
}

package installer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/getlawrence/cli/internal/codegen/dependency/types"
)

// MavenInstaller installs Java dependencies using Maven
type MavenInstaller struct {
	commander types.Commander
}

// NewMavenInstaller creates a new Maven installer
func NewMavenInstaller(commander types.Commander) Installer {
	return &MavenInstaller{commander: commander}
}

// Install installs Java dependencies
func (i *MavenInstaller) Install(ctx context.Context, projectPath string, dependencies []string, dryRun bool) error {
	if len(dependencies) == 0 {
		return nil
	}

	pomPath := filepath.Join(projectPath, "pom.xml")
	hasPom := false
	if _, err := os.Stat(pomPath); err == nil {
		hasPom = true
	}

	// Check for Gradle
	hasGradle := false
	if _, err := os.Stat(filepath.Join(projectPath, "build.gradle")); err == nil {
		hasGradle = true
	} else if _, err := os.Stat(filepath.Join(projectPath, "build.gradle.kts")); err == nil {
		hasGradle = true
	}

	if !hasPom && !hasGradle {
		return fmt.Errorf("no pom.xml or build.gradle found in %s", projectPath)
	}

	// Resolve versions
	resolved, err := i.resolveVersions(dependencies)
	if err != nil {
		return err
	}

	if dryRun {
		return nil
	}

	// For Maven projects, try to fetch dependencies
	if hasPom {
		if _, err := i.commander.LookPath("mvn"); err == nil {
			// Use mvn dependency:get to fetch artifacts
			for _, dep := range resolved {
				args := []string{"dependency:get", fmt.Sprintf("-Dartifact=%s", dep)}
				if _, err := i.commander.Run(ctx, "mvn", args, projectPath); err != nil {
					// Log but don't fail - user needs to add to pom.xml manually
					fmt.Printf("Note: mvn dependency:get failed for %s: %v\n", dep, err)
				}
			}
		}
	}

	// For now, we don't auto-edit pom.xml or build.gradle
	// Just provide instructions
	if hasPom {
		fmt.Println("Please add the following dependencies to your pom.xml:")
		for _, dep := range resolved {
			parts := strings.Split(dep, ":")
			if len(parts) >= 2 {
				fmt.Printf("  <dependency>\n")
				fmt.Printf("    <groupId>%s</groupId>\n", parts[0])
				fmt.Printf("    <artifactId>%s</artifactId>\n", parts[1])
				if len(parts) >= 3 {
					fmt.Printf("    <version>%s</version>\n", parts[2])
				}
				fmt.Printf("  </dependency>\n")
			}
		}
	} else if hasGradle {
		fmt.Println("Please add the following dependencies to your build.gradle:")
		for _, dep := range resolved {
			fmt.Printf("  implementation '%s'\n", dep)
		}
	}

	return nil
}

// resolveVersions adds LATEST version to dependencies without version
func (i *MavenInstaller) resolveVersions(deps []string) ([]string, error) {
	var resolved []string

	for _, dep := range deps {
		// Check if already has version (group:artifact:version)
		parts := strings.Split(dep, ":")
		if len(parts) >= 3 {
			resolved = append(resolved, dep)
		} else if len(parts) == 2 {
			// Add LATEST version
			resolved = append(resolved, dep+":LATEST")
		} else {
			return nil, fmt.Errorf("invalid Maven coordinate: %s", dep)
		}
	}

	return resolved, nil
}

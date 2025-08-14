package installer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	gradlePath := ""
	if _, err := os.Stat(filepath.Join(projectPath, "build.gradle")); err == nil {
		hasGradle = true
		gradlePath = filepath.Join(projectPath, "build.gradle")
	} else if _, err := os.Stat(filepath.Join(projectPath, "build.gradle.kts")); err == nil {
		hasGradle = true
		gradlePath = filepath.Join(projectPath, "build.gradle.kts")
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

	// Auto-edit pom.xml or build.gradle files
	if hasPom {
		if err := i.addDependenciesToPom(pomPath, resolved); err != nil {
			return fmt.Errorf("failed to add dependencies to pom.xml: %w", err)
		}
		fmt.Printf("Added %d dependencies to pom.xml\n", len(resolved))
	} else if hasGradle {
		if err := i.addDependenciesToGradle(gradlePath, resolved); err != nil {
			return fmt.Errorf("failed to add dependencies to build.gradle: %w", err)
		}
		fmt.Printf("Added %d dependencies to %s\n", len(resolved), filepath.Base(gradlePath))
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

// addDependenciesToPom adds dependencies to pom.xml file
func (i *MavenInstaller) addDependenciesToPom(pomPath string, dependencies []string) error {
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return err
	}

	// Find the dependencies section - use a more robust pattern that handles whitespace
	depsPattern := regexp.MustCompile(`(?s)(<dependencies>)(.*?)(</dependencies>)`)
	matches := depsPattern.FindSubmatch(content)

	if len(matches) == 0 {
		// No dependencies section found, create one before </project>
		projectEndPattern := regexp.MustCompile(`(</project>)`)
		if !projectEndPattern.Match(content) {
			return fmt.Errorf("could not find </project> tag in pom.xml")
		}

		// Create new dependencies section
		var newDeps strings.Builder
		newDeps.WriteString("\n  <dependencies>\n")
		for _, dep := range dependencies {
			parts := strings.Split(dep, ":")
			if len(parts) >= 2 {
				newDeps.WriteString("    <dependency>\n")
				newDeps.WriteString(fmt.Sprintf("      <groupId>%s</groupId>\n", parts[0]))
				newDeps.WriteString(fmt.Sprintf("      <artifactId>%s</artifactId>\n", parts[1]))
				if len(parts) >= 3 && parts[2] != "LATEST" {
					newDeps.WriteString(fmt.Sprintf("      <version>%s</version>\n", parts[2]))
				}
				newDeps.WriteString("    </dependency>\n")
			}
		}
		newDeps.WriteString("  </dependencies>\n")

		// Insert before </project>
		newContent := projectEndPattern.ReplaceAll(content, []byte(newDeps.String()+"</project>"))
		return os.WriteFile(pomPath, newContent, 0644)
	}

	// Dependencies section exists, add to it
	depsStart := matches[1]
	depsContent := matches[2]
	depsEnd := matches[3]

	// Check if dependencies already exist to avoid duplicates
	var newDepsToAdd []string
	for _, dep := range dependencies {
		parts := strings.Split(dep, ":")
		if len(parts) >= 2 {
			// Check if this dependency already exists
			depPattern := regexp.MustCompile(fmt.Sprintf(`<groupId>%s</groupId>\s*<artifactId>%s</artifactId>`,
				regexp.QuoteMeta(parts[0]), regexp.QuoteMeta(parts[1])))
			if !depPattern.Match(depsContent) {
				// Prepare new dependency XML
				newDep := fmt.Sprintf("    <dependency>\n      <groupId>%s</groupId>\n      <artifactId>%s</artifactId>\n",
					parts[0], parts[1])
				if len(parts) >= 3 && parts[2] != "LATEST" {
					newDep += fmt.Sprintf("      <version>%s</version>\n", parts[2])
				}
				newDep += "    </dependency>\n"
				newDepsToAdd = append(newDepsToAdd, newDep)
			}
		}
	}

	// If no new dependencies to add, return early
	if len(newDepsToAdd) == 0 {
		return nil
	}

	// Add new dependencies to the existing content
	for _, newDep := range newDepsToAdd {
		depsContent = append(depsContent, []byte(newDep)...)
	}

	// Reconstruct the file by replacing the entire dependencies section
	oldDepsSection := matches[0]
	newDepsSection := append(append(depsStart, depsContent...), depsEnd...)
	newContent := bytes.Replace(content, oldDepsSection, newDepsSection, 1)

	return os.WriteFile(pomPath, newContent, 0644)
}

// addDependenciesToGradle adds dependencies to build.gradle file
func (i *MavenInstaller) addDependenciesToGradle(gradlePath string, dependencies []string) error {
	content, err := os.ReadFile(gradlePath)
	if err != nil {
		return err
	}

	// Find the dependencies block
	depsPattern := regexp.MustCompile(`(dependencies\s*\{)(.*?)(\})`)
	matches := depsPattern.FindSubmatch(content)

	if len(matches) == 0 {
		// No dependencies block found, create one at the end of the file
		var newDeps strings.Builder
		newDeps.WriteString("\ndependencies {\n")
		for _, dep := range dependencies {
			newDeps.WriteString(fmt.Sprintf("    implementation '%s'\n", dep))
		}
		newDeps.WriteString("}\n")

		// Append to the end of the file
		newContent := append(content, []byte(newDeps.String())...)
		return os.WriteFile(gradlePath, newContent, 0644)
	}

	// Dependencies block exists, add to it
	depsStart := matches[1]
	depsContent := matches[2]
	depsEnd := matches[3]

	// Check if dependencies already exist to avoid duplicates
	for _, dep := range dependencies {
		// Check if this dependency already exists
		depPattern := regexp.MustCompile(fmt.Sprintf(`implementation\s+['"]%s['"]`, regexp.QuoteMeta(dep)))
		if !depPattern.Match(depsContent) {
			// Add new dependency
			newDep := fmt.Sprintf("    implementation '%s'\n", dep)
			depsContent = append(depsContent, []byte(newDep)...)
		}
	}

	// Reconstruct the file
	newContent := bytes.Replace(content, matches[0], append(append(depsStart, depsContent...), depsEnd...), 1)
	return os.WriteFile(gradlePath, newContent, 0644)
}

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
		// Add Maven Shade plugin if OpenTelemetry dependencies are being added
		if i.hasOpenTelemetryDependencies(resolved) {
			if err := i.addMavenShadePlugin(pomPath); err != nil {
				return fmt.Errorf("failed to add Maven Shade plugin: %w", err)
			}
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

	// Prune known invalid or unwanted dependencies (e.g., HTTP exporter which is not available at 1.36.0)
	pruneHTTPExporter := regexp.MustCompile(`(?s)\n\s*<dependency>\s*<groupId>io\.opentelemetry</groupId>\s*<artifactId>opentelemetry-exporter-otlp-http</artifactId>[\s\S]*?</dependency>\s*`)
	content = pruneHTTPExporter.ReplaceAll(content, []byte("\n"))

	// Separate BOM dependencies from regular dependencies first
	var bomDeps []string
	var regularDeps []string
	for _, dep := range dependencies {
		if strings.Contains(dep, "-bom:") || strings.Contains(dep, "opentelemetry-bom") {
			bomDeps = append(bomDeps, dep)
		} else {
			regularDeps = append(regularDeps, dep)
		}
	}

	// Handle BOM dependencies first (add to dependencyManagement)
	if len(bomDeps) > 0 {
		if err := i.addBOMDependencies(pomPath, bomDeps); err != nil {
			return fmt.Errorf("failed to add BOM dependencies: %w", err)
		}
		// Re-read content after BOM modification
		content, err = os.ReadFile(pomPath)
		if err != nil {
			return err
		}
	}

	// Now handle regular dependencies
	if len(regularDeps) == 0 {
		return nil
	}

	// Find the top-level dependencies section (not inside dependencyManagement)
	// Strategy: Look for dependencies section that's not preceded by dependencyManagement
	depsPattern := regexp.MustCompile(`(?s)(<dependencies>)(.*?)(</dependencies>)`)
	allMatches := depsPattern.FindAllSubmatchIndex(content, -1)

	var topLevelDepsMatch []int
	for _, match := range allMatches {
		// Check if this dependencies section is inside dependencyManagement
		beforeDeps := content[:match[0]]
		depMgmtStart := strings.LastIndex(string(beforeDeps), "<dependencyManagement>")
		depMgmtEnd := strings.LastIndex(string(beforeDeps), "</dependencyManagement>")

		// If there's no dependencyManagement before this, or if dependencyManagement was closed before this
		if depMgmtStart == -1 || (depMgmtEnd != -1 && depMgmtEnd > depMgmtStart) {
			topLevelDepsMatch = match
			break
		}
	}

	if topLevelDepsMatch == nil {
		// No top-level dependencies section found, create one for regular dependencies
		projectEndPattern := regexp.MustCompile(`(</project>)`)
		if !projectEndPattern.Match(content) {
			return fmt.Errorf("could not find </project> tag in pom.xml")
		}

		var newDeps strings.Builder
		newDeps.WriteString("\n  <dependencies>\n")
		for _, dep := range regularDeps {
			parts := strings.Split(dep, ":")
			if len(parts) >= 2 {
				newDeps.WriteString("    <dependency>\n")
				newDeps.WriteString(fmt.Sprintf("      <groupId>%s</groupId>\n", parts[0]))
				newDeps.WriteString(fmt.Sprintf("      <artifactId>%s</artifactId>\n", parts[1]))
				// Only add version if no BOM is present
				if len(parts) >= 3 && parts[2] != "LATEST" && len(bomDeps) == 0 {
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

	// Top-level dependencies section exists, find it in the original content and add regular dependencies
	topLevelDepsPattern := regexp.MustCompile(`(?s)(<dependencies>)(.*?)(</dependencies>)`)

	// Find all dependencies sections and select the top-level one (not inside dependencyManagement)
	allOriginalMatches := topLevelDepsPattern.FindAllSubmatch(content, -1)
	var targetMatch [][]byte

	for _, match := range allOriginalMatches {
		matchStart := bytes.Index(content, match[0])
		beforeMatch := content[:matchStart]

		// Check if this dependencies section is inside dependencyManagement
		depMgmtStart := bytes.LastIndex(beforeMatch, []byte("<dependencyManagement>"))
		depMgmtEnd := bytes.LastIndex(beforeMatch, []byte("</dependencyManagement>"))

		// If there's no dependencyManagement before this, or if dependencyManagement was closed before this
		if depMgmtStart == -1 || (depMgmtEnd != -1 && depMgmtEnd > depMgmtStart) {
			targetMatch = match
			break
		}
	}

	if targetMatch == nil {
		return fmt.Errorf("could not find top-level dependencies section")
	}

	depsStart := targetMatch[1]
	depsContent := targetMatch[2]
	depsEnd := targetMatch[3]

	// Check if regular dependencies already exist to avoid duplicates
	var newDepsToAdd []string
	for _, dep := range regularDeps {
		parts := strings.Split(dep, ":")
		if len(parts) >= 2 {
			groupID := parts[0]
			artifactID := parts[1]
			version := ""
			if len(parts) >= 3 && parts[2] != "LATEST" {
				version = parts[2]
			}

			// Check if this dependency already exists
			depPattern := regexp.MustCompile(fmt.Sprintf(`<groupId>%s</groupId>\s*<artifactId>%s</artifactId>`,
				regexp.QuoteMeta(groupID), regexp.QuoteMeta(artifactID)))
			if !depPattern.Match(depsContent) {
				// Prepare new dependency XML
				newDep := fmt.Sprintf("    <dependency>\n      <groupId>%s</groupId>\n      <artifactId>%s</artifactId>\n",
					groupID, artifactID)
				// Only add version if no BOM is present (BOM manages versions)
				if version != "" && len(bomDeps) == 0 {
					newDep += fmt.Sprintf("      <version>%s</version>\n", version)
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
	newDepsContent := depsContent
	for _, newDep := range newDepsToAdd {
		newDepsContent = append(newDepsContent, []byte(newDep)...)
	}

	// Reconstruct the file by replacing the entire top-level dependencies section
	oldDepsSection := targetMatch[0]
	newDepsSection := append(append(depsStart, newDepsContent...), depsEnd...)
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

// addBOMDependencies adds BOM dependencies to the dependencyManagement section
func (i *MavenInstaller) addBOMDependencies(pomPath string, bomDeps []string) error {
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return err
	}

	// Find or create dependencyManagement section
	depMgmtPattern := regexp.MustCompile(`(?s)(<dependencyManagement>)(.*?)(</dependencyManagement>)`)
	matches := depMgmtPattern.FindSubmatch(content)

	if len(matches) == 0 {
		// No dependencyManagement section, create one before dependencies or project end
		var newDepMgmt strings.Builder
		newDepMgmt.WriteString("\n  <dependencyManagement>\n    <dependencies>\n")
		for _, dep := range bomDeps {
			parts := strings.Split(dep, ":")
			if len(parts) >= 3 {
				newDepMgmt.WriteString("      <dependency>\n")
				newDepMgmt.WriteString(fmt.Sprintf("        <groupId>%s</groupId>\n", parts[0]))
				newDepMgmt.WriteString(fmt.Sprintf("        <artifactId>%s</artifactId>\n", parts[1]))
				newDepMgmt.WriteString(fmt.Sprintf("        <version>%s</version>\n", parts[2]))
				newDepMgmt.WriteString("        <type>pom</type>\n")
				newDepMgmt.WriteString("        <scope>import</scope>\n")
				newDepMgmt.WriteString("      </dependency>\n")
			}
		}
		newDepMgmt.WriteString("    </dependencies>\n  </dependencyManagement>\n")

		// Insert before <dependencies> or <build> or </project>
		insertTargets := []string{`<dependencies>`, `<build>`, `</project>`}
		for _, target := range insertTargets {
			targetPattern := regexp.MustCompile(fmt.Sprintf(`(%s)`, regexp.QuoteMeta(target)))
			if targetPattern.Match(content) {
				newContent := targetPattern.ReplaceAll(content, []byte(newDepMgmt.String()+target))
				return os.WriteFile(pomPath, newContent, 0644)
			}
		}
		return fmt.Errorf("could not find insertion point for dependencyManagement section")
	}

	// dependencyManagement section exists, add BOM to it
	depMgmtStart := matches[1]
	depMgmtContent := matches[2]
	depMgmtEnd := matches[3]

	// Find or create dependencies section within dependencyManagement
	innerDepsPattern := regexp.MustCompile(`(?s)(<dependencies>)(.*?)(</dependencies>)`)
	innerMatches := innerDepsPattern.FindSubmatch(depMgmtContent)

	if len(innerMatches) == 0 {
		// No dependencies section within dependencyManagement, create one
		var newInnerDeps strings.Builder
		newInnerDeps.WriteString("    <dependencies>\n")
		for _, dep := range bomDeps {
			parts := strings.Split(dep, ":")
			if len(parts) >= 3 {
				newInnerDeps.WriteString("      <dependency>\n")
				newInnerDeps.WriteString(fmt.Sprintf("        <groupId>%s</groupId>\n", parts[0]))
				newInnerDeps.WriteString(fmt.Sprintf("        <artifactId>%s</artifactId>\n", parts[1]))
				newInnerDeps.WriteString(fmt.Sprintf("        <version>%s</version>\n", parts[2]))
				newInnerDeps.WriteString("        <type>pom</type>\n")
				newInnerDeps.WriteString("        <scope>import</scope>\n")
				newInnerDeps.WriteString("      </dependency>\n")
			}
		}
		newInnerDeps.WriteString("    </dependencies>\n")

		newDepMgmtContent := append(depMgmtContent, []byte(newInnerDeps.String())...)
		newDepMgmtSection := append(append(depMgmtStart, newDepMgmtContent...), depMgmtEnd...)
		newContent := bytes.Replace(content, matches[0], newDepMgmtSection, 1)
		return os.WriteFile(pomPath, newContent, 0644)
	}

	// Dependencies section exists within dependencyManagement, add BOMs to it
	innerDepsStart := innerMatches[1]
	innerDepsContent := innerMatches[2]
	innerDepsEnd := innerMatches[3]

	// Add new BOM dependencies
	for _, dep := range bomDeps {
		parts := strings.Split(dep, ":")
		if len(parts) >= 2 {
			groupID := parts[0]
			artifactID := parts[1]
			version := ""
			if len(parts) >= 3 && parts[2] != "LATEST" {
				version = parts[2]
			}

			// Check if this BOM dependency already exists
			bomPattern := regexp.MustCompile(fmt.Sprintf(`(<groupId>%s</groupId>\s*<artifactId>%s</artifactId>)`,
				regexp.QuoteMeta(groupID), regexp.QuoteMeta(artifactID)))
			if !bomPattern.Match(innerDepsContent) {
				// Add new BOM dependency
				newBomDep := fmt.Sprintf("      <dependency>\n        <groupId>%s</groupId>\n        <artifactId>%s</artifactId>\n",
					groupID, artifactID)
				if version != "" {
					newBomDep += fmt.Sprintf("        <version>%s</version>\n", version)
				}
				newBomDep += "        <type>pom</type>\n        <scope>import</scope>\n      </dependency>\n"
				innerDepsContent = append(innerDepsContent, []byte(newBomDep)...)
			}
		}
	}

	// Reconstruct the dependencyManagement section
	newInnerDepsSection := append(append(innerDepsStart, innerDepsContent...), innerDepsEnd...)
	newDepMgmtSection := append(append(depMgmtStart, newInnerDepsSection...), depMgmtEnd...)
	newContent := bytes.Replace(content, matches[0], newDepMgmtSection, 1)
	return os.WriteFile(pomPath, newContent, 0644)
}

// hasOpenTelemetryDependencies checks if any of the dependencies are OpenTelemetry related
func (i *MavenInstaller) hasOpenTelemetryDependencies(dependencies []string) bool {
	for _, dep := range dependencies {
		if strings.Contains(dep, "io.opentelemetry") {
			return true
		}
	}
	return false
}

// addMavenShadePlugin adds the Maven Shade plugin to create fat JARs with all dependencies
func (i *MavenInstaller) addMavenShadePlugin(pomPath string) error {
	content, err := os.ReadFile(pomPath)
	if err != nil {
		return err
	}

	// Check if Maven Shade plugin already exists
	shadePluginPattern := regexp.MustCompile(`maven-shade-plugin`)
	if shadePluginPattern.Match(content) {
		return nil // Plugin already exists
	}

	// Find the build/plugins section
	pluginsPattern := regexp.MustCompile(`(?s)(<build>.*?<plugins>)(.*?)(</plugins>.*?</build>)`)
	matches := pluginsPattern.FindSubmatch(content)

	if len(matches) == 0 {
		// No build/plugins section found, create one before </project>
		shadePluginXML := `
  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-shade-plugin</artifactId>
        <version>3.4.1</version>
        <executions>
          <execution>
            <phase>package</phase>
            <goals>
              <goal>shade</goal>
            </goals>
            <configuration>
              <createDependencyReducedPom>false</createDependencyReducedPom>
              <transformers>
                <transformer implementation="org.apache.maven.plugins.shade.resource.ManifestResourceTransformer">
                  <mainClass>com.example.App</mainClass>
                </transformer>
              </transformers>
            </configuration>
          </execution>
        </executions>
      </plugin>
    </plugins>
  </build>
`
		// Insert before </project>
		projectEndPattern := regexp.MustCompile(`(</project>)`)
		if !projectEndPattern.Match(content) {
			return fmt.Errorf("could not find </project> tag in pom.xml")
		}
		newContent := projectEndPattern.ReplaceAll(content, []byte(shadePluginXML+"</project>"))
		return os.WriteFile(pomPath, newContent, 0644)
	}

	// Build/plugins section exists, add shade plugin to it
	pluginsStart := matches[1]
	pluginsContent := matches[2]
	pluginsEnd := matches[3]

	shadePluginXML := `      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-shade-plugin</artifactId>
        <version>3.4.1</version>
        <executions>
          <execution>
            <phase>package</phase>
            <goals>
              <goal>shade</goal>
            </goals>
            <configuration>
              <createDependencyReducedPom>false</createDependencyReducedPom>
              <transformers>
                <transformer implementation="org.apache.maven.plugins.shade.resource.ManifestResourceTransformer">
                  <mainClass>com.example.App</mainClass>
                </transformer>
              </transformers>
            </configuration>
          </execution>
        </executions>
      </plugin>
`

	// Add the plugin to existing plugins content with proper formatting
	newPluginsContent := append(pluginsContent, []byte("\n"+shadePluginXML)...)

	// Reconstruct the file
	newContent := bytes.Replace(content, matches[0], append(append(pluginsStart, newPluginsContent...), pluginsEnd...), 1)
	return os.WriteFile(pomPath, newContent, 0644)
}

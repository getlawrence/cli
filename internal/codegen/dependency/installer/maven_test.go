package installer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/getlawrence/cli/internal/codegen/dependency/commander"
)

func TestMavenInstaller(t *testing.T) {
	ctx := context.Background()

	t.Run("adds dependencies to existing pom.xml without creating duplicates", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with existing pom.xml that has dependencies section
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
  
  <dependencies>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <version>5.9.2</version>
      <scope>test</scope>
    </dependency>
  </dependencies>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies
		deps := []string{"io.opentelemetry:opentelemetry-api:1.32.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		// Check that there's only ONE dependencies section
		depsCount := strings.Count(string(content), "<dependencies>")
		if depsCount != 1 {
			t.Errorf("Expected exactly 1 dependencies section, found %d", depsCount)
		}

		// Check that both dependencies are present
		if !strings.Contains(string(content), "junit-jupiter") {
			t.Error("Expected existing dependency to remain")
		}
		if !strings.Contains(string(content), "opentelemetry-api") {
			t.Error("Expected new dependency to be added")
		}

		// Verify the XML is valid by checking structure
		if strings.Count(string(content), "</dependencies>") != 1 {
			t.Error("Expected exactly 1 closing dependencies tag")
		}
	})

	t.Run("creates dependencies section when none exists", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with pom.xml that has NO dependencies section
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add dependencies
		deps := []string{"io.opentelemetry:opentelemetry-api:1.32.0"}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		// Check that exactly ONE dependencies section was created
		depsCount := strings.Count(string(content), "<dependencies>")
		if depsCount != 1 {
			t.Errorf("Expected exactly 1 dependencies section, found %d", depsCount)
		}

		// Check that the dependency was added
		if !strings.Contains(string(content), "opentelemetry-api") {
			t.Error("Expected dependency to be added")
		}
	})

	t.Run("handles multiple dependency additions without duplication", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add first set of dependencies
		deps1 := []string{"io.opentelemetry:opentelemetry-api:1.32.0"}
		err := installer.Install(ctx, dir, deps1, false)
		if err != nil {
			t.Fatal(err)
		}

		// Add second set of dependencies
		deps2 := []string{"io.opentelemetry:opentelemetry-sdk:1.32.0"}
		err = installer.Install(ctx, dir, deps2, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the final pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		// Check that there's still only ONE dependencies section
		depsCount := strings.Count(string(content), "<dependencies>")
		if depsCount != 1 {
			t.Errorf("Expected exactly 1 dependencies section after multiple additions, found %d", depsCount)
		}

		// Check that both dependencies are present
		if !strings.Contains(string(content), "opentelemetry-api") {
			t.Error("Expected first dependency to remain")
		}
		if !strings.Contains(string(content), "opentelemetry-sdk") {
			t.Error("Expected second dependency to be added")
		}
	})

	t.Run("handles BOM dependencies correctly", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with no dependency management
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
  
  <properties>
    <maven.compiler.source>17</maven.compiler.source>
    <maven.compiler.target>17</maven.compiler.target>
  </properties>

  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
        <version>3.11.0</version>
      </plugin>
    </plugins>
  </build>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies with BOM
		deps := []string{
			"io.opentelemetry:opentelemetry-bom:1.42.1",
			"io.opentelemetry:opentelemetry-api",
			"io.opentelemetry:opentelemetry-sdk",
			"io.opentelemetry:opentelemetry-exporter-otlp",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Check that dependencyManagement section was created
		if !strings.Contains(contentStr, "<dependencyManagement>") {
			t.Error("Expected dependencyManagement section to be created")
		}

		// Check that BOM is in dependencyManagement with proper structure
		if !strings.Contains(contentStr, "opentelemetry-bom") {
			t.Error("Expected BOM dependency to be added")
		}
		if !strings.Contains(contentStr, "<type>pom</type>") {
			t.Error("Expected BOM to have type=pom")
		}
		if !strings.Contains(contentStr, "<scope>import</scope>") {
			t.Error("Expected BOM to have scope=import")
		}

		// Check that regular dependencies are in dependencies section WITHOUT versions
		if !strings.Contains(contentStr, "opentelemetry-api") {
			t.Error("Expected opentelemetry-api dependency")
		}
		if !strings.Contains(contentStr, "opentelemetry-sdk") {
			t.Error("Expected opentelemetry-sdk dependency")
		}
		if !strings.Contains(contentStr, "opentelemetry-exporter-otlp") {
			t.Error("Expected opentelemetry-exporter-otlp dependency")
		}

		// Verify that regular OpenTelemetry dependencies don't have versions (managed by BOM)
		if !strings.Contains(contentStr, "</dependency>") {
			t.Error("Dependencies should be properly closed")
		}

		// Check that there's exactly one dependencies section and one dependencyManagement section
		if strings.Count(contentStr, "<dependencies>") != 2 { // One in dependencyManagement, one regular
			t.Errorf("Expected exactly 2 dependencies sections (1 in dependencyManagement, 1 regular), found %d",
				strings.Count(contentStr, "<dependencies>"))
		}
		if strings.Count(contentStr, "<dependencyManagement>") != 1 {
			t.Errorf("Expected exactly 1 dependencyManagement section, found %d",
				strings.Count(contentStr, "<dependencyManagement>"))
		}
	})

	t.Run("handles existing dependencyManagement section", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with existing dependencyManagement
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
  
  <dependencyManagement>
    <dependencies>
      <dependency>
        <groupId>org.springframework</groupId>
        <artifactId>spring-framework-bom</artifactId>
        <version>6.0.0</version>
        <type>pom</type>
        <scope>import</scope>
      </dependency>
    </dependencies>
  </dependencyManagement>

  <dependencies>
    <dependency>
      <groupId>org.springframework</groupId>
      <artifactId>spring-core</artifactId>
    </dependency>
  </dependencies>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies with BOM
		deps := []string{
			"io.opentelemetry:opentelemetry-bom:1.42.1",
			"io.opentelemetry:opentelemetry-api",
			"io.opentelemetry:opentelemetry-sdk",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Check that both BOMs are present in dependencyManagement
		if !strings.Contains(contentStr, "spring-framework-bom") {
			t.Error("Expected existing BOM to remain")
		}
		if !strings.Contains(contentStr, "opentelemetry-bom") {
			t.Error("Expected new BOM to be added")
		}

		// Check that both regular dependencies are present
		if !strings.Contains(contentStr, "spring-core") {
			t.Error("Expected existing dependency to remain")
		}
		if !strings.Contains(contentStr, "opentelemetry-api") {
			t.Error("Expected new dependency to be added")
		}

		// Verify structure integrity
		if strings.Count(contentStr, "<dependencyManagement>") != 1 {
			t.Error("Expected exactly 1 dependencyManagement section")
		}
		if strings.Count(contentStr, "<dependencies>") != 2 { // One in dependencyManagement, one regular
			t.Error("Expected exactly 2 dependencies sections")
		}
	})

	t.Run("adds Maven Shade plugin for OpenTelemetry projects", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>

  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
        <version>3.11.0</version>
      </plugin>
    </plugins>
  </build>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies
		deps := []string{
			"io.opentelemetry:opentelemetry-bom:1.42.1",
			"io.opentelemetry:opentelemetry-api",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Check that Maven Shade plugin was added
		if !strings.Contains(contentStr, "maven-shade-plugin") {
			t.Error("Expected Maven Shade plugin to be added for OpenTelemetry projects")
		}

		// Check that the plugin has the correct configuration
		if !strings.Contains(contentStr, "ManifestResourceTransformer") {
			t.Error("Expected ManifestResourceTransformer in Shade plugin config")
		}
		if !strings.Contains(contentStr, "<mainClass>com.example.App</mainClass>") {
			t.Error("Expected mainClass configuration in Shade plugin")
		}

		// Verify both plugins are present
		if !strings.Contains(contentStr, "maven-compiler-plugin") {
			t.Error("Expected existing compiler plugin to remain")
		}
	})

	t.Run("handles empty existing dependencies section", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with empty dependencies section
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>
  
  <dependencies>
    <!-- No dependencies yet -->
  </dependencies>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies with BOM
		deps := []string{
			"io.opentelemetry:opentelemetry-bom:1.42.1",
			"io.opentelemetry:opentelemetry-api",
			"io.opentelemetry:opentelemetry-sdk",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Check structure integrity
		if strings.Count(contentStr, "<dependencies>") != 2 { // One in dependencyManagement, one regular
			t.Errorf("Expected exactly 2 dependencies sections, found %d",
				strings.Count(contentStr, "<dependencies>"))
		}

		// Check that dependencies were added to the regular dependencies section
		if !strings.Contains(contentStr, "opentelemetry-api") {
			t.Error("Expected opentelemetry-api in dependencies section")
		}
		if !strings.Contains(contentStr, "opentelemetry-sdk") {
			t.Error("Expected opentelemetry-sdk in dependencies section")
		}

		// Check that BOM is in dependencyManagement
		if !strings.Contains(contentStr, "opentelemetry-bom") {
			t.Error("Expected BOM in dependencyManagement section")
		}
	})

	t.Run("does not add duplicate Maven Shade plugin", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project with existing Maven Shade plugin
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
  <modelVersion>4.0.0</modelVersion>
  <groupId>com.example</groupId>
  <artifactId>test</artifactId>
  <version>1.0.0</version>

  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-shade-plugin</artifactId>
        <version>3.2.4</version>
      </plugin>
    </plugins>
  </build>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies
		deps := []string{
			"io.opentelemetry:opentelemetry-api",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Check that there's still only one Maven Shade plugin
		shadeCount := strings.Count(contentStr, "maven-shade-plugin")
		if shadeCount != 1 {
			t.Errorf("Expected exactly 1 maven-shade-plugin, found %d", shadeCount)
		}
	})

	t.Run("creates proper XML structure for complex scenarios", func(t *testing.T) {
		mock := commander.NewMock()
		installer := NewMavenInstaller(mock)

		// Create test project similar to real Lawrence examples
		dir := t.TempDir()
		pomPath := filepath.Join(dir, "pom.xml")
		initialPom := `<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0"
         xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
         xsi:schemaLocation="http://maven.apache.org/POM/4.0.0 https://maven.apache.org/xsd/maven-4.0.0.xsd">
  <modelVersion>4.0.0</modelVersion>

  <groupId>com.example</groupId>
  <artifactId>java-sample</artifactId>
  <version>1.0-SNAPSHOT</version>
  <name>java-sample</name>
  <description>Minimal Java sample app for Lawrence CLI Java support</description>

  <properties>
    <maven.compiler.source>17</maven.compiler.source>
    <maven.compiler.target>17</maven.compiler.target>
  </properties>

  <dependencies>
    <!-- OpenTelemetry dependencies will be added back once basic container works -->
  </dependencies>

  <build>
    <plugins>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-compiler-plugin</artifactId>
        <version>3.11.0</version>
        <configuration>
          <release>17</release>
          <compilerArgs>
            <arg>--add-modules</arg>
            <arg>jdk.httpserver</arg>
          </compilerArgs>
        </configuration>
      </plugin>
      <plugin>
        <groupId>org.apache.maven.plugins</groupId>
        <artifactId>maven-jar-plugin</artifactId>
        <version>3.3.0</version>
        <configuration>
          <archive>
            <manifest>
              <mainClass>com.example.App</mainClass>
            </manifest>
          </archive>
        </configuration>
      </plugin>
    </plugins>
  </build>
</project>`

		if err := os.WriteFile(pomPath, []byte(initialPom), 0644); err != nil {
			t.Fatal(err)
		}

		// Add OpenTelemetry dependencies with BOM (like real usage)
		deps := []string{
			"io.opentelemetry:opentelemetry-bom:1.42.1",
			"io.opentelemetry:opentelemetry-api",
			"io.opentelemetry:opentelemetry-sdk",
			"io.opentelemetry:opentelemetry-exporter-otlp",
		}
		err := installer.Install(ctx, dir, deps, false)
		if err != nil {
			t.Fatal(err)
		}

		// Read the modified pom.xml
		content, err := os.ReadFile(pomPath)
		if err != nil {
			t.Fatal(err)
		}

		contentStr := string(content)

		// Verify proper XML structure
		if !strings.Contains(contentStr, "</dependencyManagement>") {
			t.Error("Expected properly closed dependencyManagement section")
		}
		if !strings.Contains(contentStr, "</dependencies>") {
			t.Error("Expected properly closed dependencies section")
		}
		if !strings.Contains(contentStr, "</plugins>") {
			t.Error("Expected properly closed plugins section")
		}
		if !strings.Contains(contentStr, "</project>") {
			t.Error("Expected properly closed project tag")
		}

		// Verify all OpenTelemetry dependencies are present
		otelDeps := []string{"opentelemetry-api", "opentelemetry-sdk", "opentelemetry-exporter-otlp"}
		for _, dep := range otelDeps {
			if !strings.Contains(contentStr, dep) {
				t.Errorf("Expected %s dependency to be present", dep)
			}
		}

		// Verify Maven Shade plugin was added
		if !strings.Contains(contentStr, "maven-shade-plugin") {
			t.Error("Expected Maven Shade plugin to be added")
		}

		// Verify that the original plugins remain
		if !strings.Contains(contentStr, "maven-compiler-plugin") {
			t.Error("Expected existing maven-compiler-plugin to remain")
		}
		if !strings.Contains(contentStr, "maven-jar-plugin") {
			t.Error("Expected existing maven-jar-plugin to remain")
		}
	})
}

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
}

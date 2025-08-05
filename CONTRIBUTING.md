# Contributing to Lawrence

Thank you for your interest in contributing to Lawrence! This document provides guidelines for contributing to the project.

## Getting Started

1. Fork the repository
2. Clone your fork: `git clone https://github.com/your-username/cli.git`
3. Create a feature branch: `git checkout -b feature/my-new-feature`
4. Make your changes
5. Test your changes: `go test ./...`
6. Build and test the CLI: `go build -o lawrence && ./lawrence analyze examples/`
7. Commit your changes: `git commit -am 'Add new feature'`
8. Push to your fork: `git push origin feature/my-new-feature`
9. Create a Pull Request

## Types of Contributions

### 1. Adding Language Support

We welcome support for new programming languages! See [ARCHITECTURE.md](ARCHITECTURE.md) for detailed instructions.

**Popular languages to add:**
- Java (Maven, Gradle)
- JavaScript/TypeScript (npm, yarn)
- C# (.NET, NuGet)
- Rust (Cargo)
- Ruby (Bundler)
- PHP (Composer)

### 2. Adding Issue Detectors

Help improve OpenTelemetry deployments by adding new issue detectors:

**Ideas for new detectors:**
- Missing resource attributes (service.name, service.version)
- Improper sampling configuration
- Missing error handling in instrumentation
- Performance anti-patterns
- Security issues (exposed endpoints, sensitive data)
- Version compatibility issues

### 3. Improving Analysis

- Better library version detection
- More accurate import parsing
- Improved file pattern matching
- Configuration validation

### 4. Documentation and Examples

- Add more example projects
- Improve command documentation
- Add troubleshooting guides
- Create integration guides

## Code Style and Standards

### Go Code Guidelines

- Follow standard Go formatting: `go fmt`
- Run linter: `golangci-lint run`
- Add tests for new functionality
- Document exported functions and types
- Use meaningful variable and function names
- Handle errors appropriately

### Project Structure

- Language detectors: `internal/detector/languages/`
- Issue detectors: `internal/detector/issues/`
- Commands: `cmd/`
- Configuration: `internal/config/`
- Examples: `examples/`

### Interface Implementation

When implementing interfaces, ensure:
- All methods are properly implemented
- Error handling is consistent
- Context is respected for cancellation
- Resource cleanup is handled

## Testing

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/detector/...
```

### Writing Tests

- Add unit tests for new detectors
- Test both positive and negative cases
- Include edge cases and error conditions
- Use table-driven tests when appropriate

Example test structure:
```go
func TestGoDetector_Detect(t *testing.T) {
    tests := []struct {
        name     string
        files    map[string]string
        expected bool
        wantErr  bool
    }{
        {
            name: "detects go.mod",
            files: map[string]string{
                "go.mod": "module example.com/test\ngo 1.21",
            },
            expected: true,
        },
        // ... more test cases
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Test implementation
        })
    }
}
```

### Integration Testing

Test your changes with the example projects:

```bash
# Build the CLI
go build -o lawrence

# Test with examples
./lawrence analyze examples/go-sample/
./lawrence analyze examples/python-sample/

# Test different output formats
./lawrence analyze examples/go-sample/ --output json
./lawrence analyze examples/python-sample/ --detailed
```

## Example: Adding Java Support

Here's a complete example of adding Java language support:

### 1. Create Language Detector

```go
// internal/detector/languages/java.go
package languages

import (
    "context"
    "os"
    "path/filepath"
    // ... other imports
)

type JavaDetector struct{}

func NewJavaDetector() *JavaDetector {
    return &JavaDetector{}
}

func (j *JavaDetector) Name() string {
    return "java"
}

func (j *JavaDetector) Detect(ctx context.Context, rootPath string) (bool, error) {
    // Check for pom.xml (Maven)
    if _, err := os.Stat(filepath.Join(rootPath, "pom.xml")); err == nil {
        return true, nil
    }
    
    // Check for build.gradle (Gradle)
    if _, err := os.Stat(filepath.Join(rootPath, "build.gradle")); err == nil {
        return true, nil
    }
    
    // Check for .java files
    javaFiles, err := filepath.Glob(filepath.Join(rootPath, "**/*.java"))
    if err != nil {
        return false, err
    }
    
    return len(javaFiles) > 0, nil
}

func (j *JavaDetector) GetOTelLibraries(ctx context.Context, rootPath string) ([]detector.Library, error) {
    var libraries []detector.Library
    
    // Parse Maven pom.xml
    pomPath := filepath.Join(rootPath, "pom.xml")
    if _, err := os.Stat(pomPath); err == nil {
        libs, err := j.parsePomXML(pomPath)
        if err != nil {
            return nil, err
        }
        libraries = append(libraries, libs...)
    }
    
    // Parse Gradle build files
    gradlePath := filepath.Join(rootPath, "build.gradle")
    if _, err := os.Stat(gradlePath); err == nil {
        libs, err := j.parseGradleBuild(gradlePath)
        if err != nil {
            return nil, err
        }
        libraries = append(libraries, libs...)
    }
    
    return j.deduplicateLibraries(libraries), nil
}

func (j *JavaDetector) GetFilePatterns() []string {
    return []string{"**/*.java", "pom.xml", "build.gradle", "build.gradle.kts"}
}

// Implement helper methods...
```

### 2. Register the Detector

```go
// cmd/analyze.go (in runAnalyze function)
manager.RegisterLanguage(languages.NewJavaDetector())
```

### 3. Add Configuration

```go
// internal/config/config.go (in DefaultConfig)
"java": {
    Enabled:      true,
    FilePatterns: []string{"**/*.java"},
    PackageFiles: []string{"pom.xml", "build.gradle", "build.gradle.kts"},
    OTelPatterns: []string{"io.opentelemetry*"},
},
```

### 4. Add Example Project

```xml
<!-- examples/java-sample/pom.xml -->
<?xml version="1.0" encoding="UTF-8"?>
<project xmlns="http://maven.apache.org/POM/4.0.0">
    <modelVersion>4.0.0</modelVersion>
    <groupId>com.example</groupId>
    <artifactId>otel-sample</artifactId>
    <version>1.0.0</version>
    
    <dependencies>
        <dependency>
            <groupId>io.opentelemetry</groupId>
            <artifactId>opentelemetry-api</artifactId>
            <version>1.32.0</version>
        </dependency>
    </dependencies>
</project>
```

### 5. Test and Document

```bash
# Test the new detector
./lawrence analyze examples/java-sample/
./lawrence list languages
```

## Submitting Pull Requests

### Pull Request Guidelines

1. **Clear title and description**: Explain what your PR does and why
2. **Reference issues**: Link to related issues with "Fixes #123"
3. **Test coverage**: Include tests for new functionality
4. **Documentation**: Update README and examples as needed
5. **Small, focused changes**: Keep PRs focused on a single feature/fix

### Pull Request Template

```markdown
## Description
Brief description of the changes.

## Type of Change
- [ ] Bug fix
- [ ] New feature (language support, issue detector, etc.)
- [ ] Documentation update
- [ ] Refactoring

## Testing
- [ ] Added/updated tests
- [ ] Tested with example projects
- [ ] Verified all existing tests pass

## Checklist
- [ ] Code follows project style guidelines
- [ ] Self-review completed
- [ ] Documentation updated
- [ ] No breaking changes (or clearly documented)
```

## Getting Help

- 📖 Check the [ARCHITECTURE.md](ARCHITECTURE.md) for technical details
- 🐛 Open an issue for bugs or questions
- 💬 Use GitHub Discussions for general questions
- 📧 Contact maintainers for sensitive issues

## Recognition

Contributors will be:
- Listed in the README contributors section
- Mentioned in release notes for significant contributions
- Credited in documentation for major features

Thank you for contributing to Lawrence! 🙏

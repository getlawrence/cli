# Project Overview

Lawrence CLI is a sophisticated codebase analyzer and auto-instrumentation tool for OpenTelemetry (OTEL). It helps developers detect, analyze, and automatically instrument their applications with OTEL across multiple programming languages. The tool is built using Go and follows a modular, extensible architecture.

## Key Features

- **Multi-Language Support**: Analyze Go, Python, JavaScript, Java, .NET (C#), Ruby, PHP
- **Library Detection**: Automatically detect OpenTelemetry libraries and versions
- **Issue Detection**: Find common problems and get actionable recommendations
- **Code Generation**: Generate instrumentation using AI agents or built-in templates
- **Knowledge Base Management**: Discover, update, and query OpenTelemetry components
- **Extensible Architecture**: Add custom detectors and language support

## Architecture Overview

The project follows a clean, modular architecture with clear separation of concerns:

### Core Domains
- **Detection & Analysis**: Language detection, library discovery, issue identification
- **Code Generation**: Template-based and AI-assisted code generation
- **Knowledge Management**: Component discovery and registry management
- **Dependency Management**: Automated dependency installation and management

### Design Patterns
- **Interface-Based Architecture**: Extensive use of interfaces for extensibility and testing
- **Strategy Pattern**: Pluggable code generation strategies (template vs AI)
- **Factory Pattern**: Language detector and provider creation
- **Adapter Pattern**: Backward compatibility for API evolution
- **Orchestrator Pattern**: Coordinated workflow management

## Folder Structure

### `/cmd` - CLI Commands
Contains Cobra command definitions and CLI interface logic:
- `analyze.go` - Codebase analysis command
- `gen.go` - Code generation command  
- `knowledge.go` - Knowledge base management
- `registry.go` - Component registry operations
- `root.go` - Root command and global flags
- `version.go` - Version information

### `/internal` - Core Business Logic
Internal packages following domain-driven design:

#### `/internal/domain`
Core domain models and types:
- `analysis.go` - Analysis results, issues, opportunities
- `instrumentation.go` - Library and package representations
- `entrypoint.go` - Application entry point detection

#### `/internal/detector`
Multi-language detection and analysis system:
- `detector.go` - Core detection interfaces and analyzer
- `language_detector.go` - Language identification
- `instrumentation_registry.go` - Component registry integration
- `knowledge_integration.go` - Knowledge base integration
- `/languages/` - Language-specific detectors (Go, Python, Java, etc.)
- `/issues/` - Issue detection implementations

#### `/internal/codegen`
Code generation and instrumentation system:
- `/generator/` - Generation strategies and orchestration
- `/injector/` - Language-specific code injection
- `/dependency/` - Dependency management framework
- `/types/` - Generation-related type definitions

#### `/internal/agents`
AI agent detection and integration:
- `detector.go` - Detect available AI coding agents

#### `/internal/templates`
Template engine and code generation templates:
- `engine.go` - Template processing engine
- `*.tmpl` - Language-specific templates

#### `/internal/logger`
Structured logging system:
- `logger.go` - Logger interface
- `stdout.go` - Standard output logger
- `ui.go` - UI-friendly logging

#### `/internal/config`
Configuration management:
- `config.go` - Application configuration

### `/pkg` - Public APIs
Public interfaces and shared components:

#### `/pkg/knowledge`
Knowledge base management system:
- `/client/` - Knowledge base client
- `/pipeline/` - Data processing pipelines  
- `/providers/` - Component discovery providers
- `/registry/` - Component registry management
- `/storage/` - Data storage abstractions
- `/types/` - Knowledge base type definitions
- `/utils/` - Utility functions

### `/examples`
Multi-language sample projects for testing and demonstration:
- Docker Compose configurations
- Sample applications in Go, Python, JavaScript, Java, C#, Ruby, PHP

### `/e2e`
End-to-end integration tests



## Tech Stack & Dependencies

### Core Framework
- **Go 1.23+** - Primary language
- **Cobra** (`github.com/spf13/cobra`) - CLI framework
- **SQLite** (`github.com/mattn/go-sqlite3`) - Knowledge base storage

### Language Processing
- **Tree-sitter** (`github.com/smacker/go-tree-sitter`) - Code parsing
- **go-enry** (`github.com/go-enry/go-enry/v2`) - Language detection

### External Integrations
- **GitHub API** (`github.com/google/go-github/v74`) - Repository interactions
- **YAML** (`gopkg.in/yaml.v3`) - Configuration parsing

### Testing
- **Testify** (`github.com/stretchr/testify`) - Testing framework

## Coding Standards & Conventions

### Go Standards
- Follow [Effective Go](https://go.dev/doc/effective_go) guidelines
- Use `gofmt` for consistent formatting
- Implement interfaces for testability and extensibility
- Prefer composition over inheritance

### Architecture Principles
- **Interface Segregation**: Small, focused interfaces
- **Dependency Inversion**: Depend on abstractions, not concretions
- **Single Responsibility**: Each component has one clear purpose
- **Open/Closed**: Open for extension, closed for modification

### Package Organization
- Domain logic in `/internal/domain`
- Interface definitions close to implementations
- Separate concerns: detection, generation, knowledge management
- Test files alongside implementation (`*_test.go`)

### Error Handling
- Wrap errors with context using `fmt.Errorf`
- Return errors explicitly, avoid panics
- Use structured logging for error reporting

### Naming Conventions
- Use clear, descriptive names
- Interfaces often end with `-er` (e.g., `Detector`, `Generator`)
- Constants use `CamelCase` with prefixes (e.g., `CategoryMissingOtel`)
- Private functions use `camelCase`

### Testing Strategy
- Unit tests for all public interfaces
- Mock dependencies using interfaces
- Integration tests in `/e2e`
- Test coverage focus on business logic

## Extension Points

### Adding New Languages
1. Implement `Language` interface in `/internal/detector/languages/`
2. Add language-specific templates in `/internal/templates/`
3. Implement code injector in `/internal/codegen/injector/`
4. Add dependency management support in `/internal/codegen/dependency/`

### Adding New Issue Detectors
1. Implement `IssueDetector` interface
2. Register in detection system
3. Add corresponding resolution templates

### Adding New Code Generation Strategies
1. Implement `CodeGenerationStrategy` interface
2. Register in generator factory
3. Add strategy-specific configuration options
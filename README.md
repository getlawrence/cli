# Lawrence - OpenTelemetry Codebase Analyzer

Lawrence is a CLI tool for analyzing codebases to detect OpenTelemetry deployments and troubleshoot common issues across multiple programming languages.

## Features

🔍 **Multi-Language Support**: Analyze Go, Python, and more programming languages
📦 **Library Detection**: Automatically detect OpenTelemetry libraries and versions
⚠️ **Issue Detection**: Find common problems and get actionable recommendations
🔧 **Extensible**: Add custom detectors and language support
📊 **Multiple Output Formats**: Text, JSON output options
⚙️ **Configurable**: Customize analysis behavior with configuration files

## Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/getlawrence/cli/main/install.sh | bash
```

### Using Go Install

```bash
go install github.com/getlawrence/cli@latest
```

Make sure `$GOPATH/bin` is in your `$PATH`:

```bash
export PATH=$PATH:$(go env GOPATH)/bin
```

### Using Homebrew (macOS/Linux)

```bash
# Add our tap (once available)
brew tap getlawrence/tap
brew install lawrence
```

### Download Pre-built Binaries

Download the latest release from the [releases page](https://github.com/getlawrence/cli/releases).

Available for:
- Linux (x64, ARM64)
- macOS (x64, ARM64) 
- Windows (x64, ARM64)

### Using Docker

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/getlawrence/cli analyze /workspace
```

### From Source

```bash
git clone https://github.com/getlawrence/cli.git
cd cli
go build -o lawrence
```

## Quick Start

### Analyze Current Directory

```bash
lawrence analyze
```

### Analyze Specific Project

```bash
lawrence analyze /path/to/your/project
```

### Get JSON Output

```bash
lawrence analyze --output json
```

### Show Detailed Information

```bash
lawrence analyze --detailed
```

## Commands

### `analyze`

Analyze a codebase for OpenTelemetry usage and issues.

```bash
lawrence analyze [path] [flags]

Flags:
  -d, --detailed              Show detailed analysis including file-level information
  -l, --languages strings     Limit analysis to specific languages (go, python, java, etc.)
      --categories strings    Limit issues to specific categories
  -o, --output string         Output format (text, json, yaml) (default "text")
```

### `list`

List supported languages and issue categories.

```bash
lawrence list languages     # Show supported programming languages
lawrence list categories    # Show available issue categories  
lawrence list detectors     # Show all available issue detectors
```

### `config`

Manage configuration files.

```bash
lawrence config init        # Create default configuration file
lawrence config show        # Display current configuration
lawrence config path        # Show configuration file path
```

## Configuration

Lawrence supports configuration files to customize its behavior. Configuration files are searched in the following order:

1. File specified with `--config` flag
2. `.lawrence.json` in current directory
3. `.lawrence.yaml` in current directory
4. `~/.lawrence.json` in home directory

### Create Configuration File

```bash
lawrence config init                # Create in current directory
lawrence config init --global      # Create in home directory
```

### Sample Configuration

```json
{
  "analysis": {
    "exclude_paths": [".git", "node_modules", "vendor"],
    "max_depth": 10,
    "follow_symlinks": false,
    "min_severity": "info"
  },
  "output": {
    "format": "text",
    "detailed": false,
    "color": true
  },
  "languages": {
    "go": {
      "enabled": true,
      "file_patterns": ["**/*.go"],
      "package_files": ["go.mod", "go.sum"],
      "otel_patterns": ["go.opentelemetry.io/*"]
    },
    "python": {
      "enabled": true,
      "file_patterns": ["**/*.py"],
      "package_files": ["requirements.txt", "pyproject.toml"],
      "otel_patterns": ["opentelemetry*"]
    }
  }
}
```

## Supported Languages

| Language | Library Detection | Import Analysis | Package Files |
|----------|-------------------|-----------------|---------------|
| Go       | ✅                | ✅              | go.mod, go.sum |
| Python   | ✅                | ✅              | requirements.txt, pyproject.toml, setup.py |
| JavaScript | ✅              | ✅              | package.json, lockfiles |
| Java     | ✅                | ✅              | pom.xml, gradle files |
| .NET     | ✅                | ✅              | .csproj, packages.config |
| Ruby     | ✅                | ✅              | Gemfile, Gemfile.lock |

See [Contributing](#contributing) to add support for your language.

## Issue Categories

Lawrence detects issues in the following categories:

- **missing_library**: Missing OpenTelemetry libraries or dependencies
- **configuration**: Configuration issues and misconfigurations  
- **instrumentation**: Instrumentation coverage and completeness
- **performance**: Performance-related issues and optimizations
- **security**: Security concerns and vulnerabilities
- **best_practice**: Best practice violations and recommendations
- **deprecated**: Deprecated features and outdated libraries

## Examples

### Basic Analysis

```bash
$ lawrence analyze
📊 OpenTelemetry Analysis Results
=================================

📂 Project Path: /path/to/project
🗣️  Languages Detected: [go]
📦 OpenTelemetry Libraries: 3
⚠️  Issues Found: 1

📦 OpenTelemetry Libraries Found:
---------------------------------
  • go.opentelemetry.io/otel (v1.21.0) - go
  • go.opentelemetry.io/otel/trace (v1.21.0) - go
  • go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp (v0.46.0) - go

ℹ️  Information (1):
  1. Missing metrics collection
     OpenTelemetry libraries found but no metrics instrumentation detected
     💡 Add OpenTelemetry metrics to monitor application performance and health
```

### JSON Output

```bash
$ lawrence analyze --output json
{
  "analysis": {
    "root_path": "/path/to/project",
    "detected_languages": ["go"],
    "libraries": [
      {
        "name": "go.opentelemetry.io/otel",
        "version": "v1.21.0",
        "language": "go",
        "import_path": "go.opentelemetry.io/otel",
        "package_file": "/path/to/project/go.mod"
      }
    ]
  },
  "issues": [...]
}
```

## Contributing

We welcome contributions! Here's how you can help:

### Adding Language Support

1. Create a new detector in `internal/detector/languages/`
2. Implement the `Language` interface
3. Register the detector in `cmd/analyze.go`
4. Add tests and documentation

### Adding Issue Detectors

1. Create a new detector in `internal/detector/issues/`
2. Implement the `IssueDetector` interface
3. Register the detector in `cmd/analyze.go`
4. Add tests and documentation

### Example: Adding Java Support

```go
// internal/detector/languages/java.go
package languages

import (
    "context"
    "github.com/getlawrence/cli/internal/detector"
)

type JavaDetector struct{}

func (j *JavaDetector) Name() string {
    return "java"
}

func (j *JavaDetector) Detect(ctx context.Context, rootPath string) (bool, error) {
    // Check for pom.xml, build.gradle, etc.
    // Return true if Java project detected
}

// Implement other interface methods...
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📖 [Documentation](https://github.com/getlawrence/cli/wiki)
- 🐛 [Issue Tracker](https://github.com/getlawrence/cli/issues)
- 💬 [Discussions](https://github.com/getlawrence/cli/discussions)
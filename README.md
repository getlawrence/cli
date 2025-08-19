# Lawrence CLI – Detect, Analyze, and Auto-Instrument OpenTelemetry Code

Lawrence CLI helps you quickly identify which programming languages and OpenTelemetry libraries a project uses, detect common misconfigurations, and produce actionable findings in text or JSON.
It can then generate instrumentation code using built-in templates or your preferred coding agent (GitHub Copilot CLI, Gemini, Claude, OpenAI).
Run in dry-run mode to preview changes or output ready-to-use scaffolding for immediate integration.

## ⚠️ **IMPORTANT: Development Status** ⚠️

**🚧 This project is currently in active development and should NOT be used on production codebases. 🚧**

- **Breaking changes** may occur without notice
- **Data loss or corruption** of your codebase is possible
- **Generated code** may not follow best practices or security standards
- **Features are experimental** and may not work as expected

**Please only use this tool on test projects or in isolated development environments.**

We recommend:
- ✅ Testing on sample/demo projects first
- ✅ Using version control and creating backups before running
- ✅ Running in `--dry-run` mode to preview changes
- ✅ Thoroughly reviewing any generated code before applying

---

## Features

- 🔍 **Multi-Language Support**: Analyze Go, Python, JavaScript, Java, .NET, Ruby, PHP
- 📦 **Library Detection**: Automatically detect OpenTelemetry libraries and versions
- ☕ **Enhanced Java Support**: Improved Maven dependency scanning and detection (v0.1.0-beta.2+)
- ⚠️ **Issue Detection**: Find common problems and get actionable recommendations
- 🔧 **Extensible**: Add custom detectors and language support
- 📊 **Multiple Output Formats**: Text, JSON output options
- 🎯 **AI & Template Code Generation**: Generate instrumentation using AI agents or built-in templates
- 🧠 **Knowledge Base Management**: Discover, update, and query OpenTelemetry components across languages

## Installation

### Quick Install (Recommended)

```bash
curl -sSL https://raw.githubusercontent.com/getlawrence/cli/main/install.sh | bash
```

### Using Go Install

```bash
go install github.com/getlawrence/cli@latest
```

**Note**: The binary will be installed as `cli`, not `lawrence`. To use it as `lawrence`, create a symlink:

```bash
# Add Go bin to your PATH
export PATH=$PATH:$(go env GOPATH)/bin

# Create a symlink so you can use 'lawrence' command
ln -sf $(go env GOPATH)/bin/cli $(go env GOPATH)/bin/lawrence
```

Add the PATH export to your shell profile (`~/.bashrc`, `~/.zshrc`, etc.) to make it permanent.

### Using Homebrew (macOS/Linux)

> ⚠️ **Note**: Homebrew installation is temporarily unavailable due to technical issues with cross-compilation. Please use one of the other installation methods below.

### Download Pre-built Binaries

Download the latest release from the [releases page](https://github.com/getlawrence/cli/releases).

Currently available for:
- Linux (x64)

> ⚠️ **Note**: macOS, Windows, and ARM64 support are temporarily unavailable due to CGO cross-compilation requirements. We're working to restore full platform support in future releases.

### Using Docker

```bash
docker run --rm -v $(pwd):/workspace ghcr.io/getlawrence/cli analyze /workspace
```

> 📋 **Note**: Docker images are available for Linux x64 only at this time.

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
      --categories strings    Limit issues to specific categories (missing_library, configuration, etc.)
  -o, --output string         Output format (text, json, yaml) (default "text")
  -v, --verbose               Verbose output

Global Flags:
  -h, --help                  Show help information
      --version               Show version information
```

### `gen`

Analyze a codebase and generate OpenTelemetry instrumentation using AI or templates.

```bash
lawrence gen [path] --mode template --dry-run

Flags:
  -l, --language string       Target language (go, javascript, python, java, dotnet, ruby, php)
  -a, --agent string          Preferred coding agent (gemini, claude, openai, github)
      --list-agents           List available coding agents
      --list-templates        List available templates
      --list-strategies       List available generation strategies
      --mode string           Generation mode (ai, template)
  -o, --output string         Output directory (template mode)
      --dry-run               Show what would be generated without writing files
      --show-prompt           Display the AI prompt that would be used
      --save-prompt string    Save the AI prompt to a file
  -c, --config string         Path to advanced OpenTelemetry config YAML
```

#### Advanced configuration (YAML)

You can pass a config file with advanced OpenTelemetry settings (instrumentations, propagators, sampler, exporters):

```yaml
# otel.yaml
service_name: my-service
service_version: 1.2.3
environment: production
instrumentations: [http, express]
propagators: [tracecontext, baggage, b3]
sampler:
  type: traceidratio
  ratio: 0.2
exporters:
  traces:
    type: otlp
    protocol: http
    endpoint: https://otel-collector.example.com/v1/traces
    insecure: false
```

Use it with:

```bash
lawrence gen --mode template --config ./otel.yaml
```

### `knowledge`

Manage the OpenTelemetry knowledge base for discovering and querying components across languages.

```bash
lawrence knowledge [command] [flags]

Commands:
  update [language]           Update knowledge base for specific language or all languages
  query [query]               Query knowledge base for components
  info                        Show knowledge base information
  providers                   List available providers

Examples:
  lawrence knowledge update                    # Update all supported languages
  lawrence knowledge update go                # Update Go language only
  lawrence knowledge update --force           # Force update even if recent data exists
  lawrence knowledge update --workers 4      # Use specific number of parallel workers
  lawrence knowledge query --language javascript --type Instrumentation
  lawrence knowledge query --name express
  lawrence knowledge query --status stable
  lawrence knowledge info                     # Show database statistics and metadata
  lawrence knowledge providers                # List supported languages and providers
```

#### Knowledge Base Update Flags

```bash
Flags for update command:
  -o, --output string         Output file path (default: knowledge.db)
  -f, --force                Force update even if recent data exists
  -w, --workers int          Number of parallel workers (0 = auto-detect based on CPU cores)
  -r, --rate-limit int       Rate limit for API requests per second per worker (default: 100)
```

#### Knowledge Base Query Flags

```bash
Flags for query command:
  -l, --language string      Filter by language
  -t, --type string          Filter by component type
  -c, --category string      Filter by component category
  -s, --status string        Filter by component status
      --support-level string Filter by support level
  -n, --name string          Filter by component name (partial match)
      --version string       Filter by version
      --framework string     Filter by framework
```

## Supported Languages

| Language   | Library Detection | Import Analysis | Package Files | Recent Enhancements |
|------------|-------------------|-----------------|---------------|-------------------|
| Go         | ✅                | ✅              | go.mod, go.sum | |
| Python     | ✅                | ✅              | requirements.txt, pyproject.toml, setup.py | |
| JavaScript | ✅                | ✅              | package.json, lockfiles | |
| Java       | ✅                | ✅              | pom.xml, gradle files | 🆕 Enhanced Maven scanning (v0.1.0-beta.2) |
| .NET       | ✅                | ✅              | .csproj, packages.config | |
| Ruby       | ✅                | ✅              | Gemfile, Gemfile.lock | |
| PHP        | ✅                | ✅              | composer.json, composer.lock | |

See [Contributing](#contributing) to add support for your language.

## Current Limitations

As this project is in active development, please be aware of these current limitations:

- **Platform Support**: Currently only Linux x64 is supported due to CGO cross-compilation requirements
- **Homebrew**: Installation via Homebrew is temporarily disabled
- **Beta Status**: Features may change between releases; use `--dry-run` to preview changes
- **Production Use**: Not recommended for production codebases (see warning above)

We're actively working to address these limitations in future releases.

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

### Knowledge Base Management

```bash
# Update knowledge base for all languages
$ lawrence knowledge update
🔄 Updating OpenTelemetry knowledge base...
📦 Processing Go packages...
📦 Processing Python packages...
📦 Processing JavaScript packages...
✅ Knowledge base updated successfully

# Query for Express.js instrumentation
$ lawrence knowledge query --name express --language javascript
🔍 Query Results for "express" in JavaScript:
  • @opentelemetry/instrumentation-express (v0.33.0)
    Status: Stable
    Type: Instrumentation
    Support Level: Official
    Category: EXPERIMENTAL

# Show knowledge base statistics
$ lawrence knowledge info
📊 Knowledge Base Information
============================
Database: knowledge.db
Total Components: 1,247
Languages: 7
Last Updated: 2024-01-15 10:30:00 UTC
```

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📖 [Documentation](https://github.com/getlawrence/cli/wiki)
- 🐛 [Issue Tracker](https://github.com/getlawrence/cli/issues)
- 💬 [Discussions](https://github.com/getlawrence/cli/discussions)
# Lawrence - OpenTelemetry Codebase Analyzer

Lawrence is a CLI tool for analyzing codebases to detect OpenTelemetry deployments and troubleshoot common issues across multiple programming languages.

## Features

🔍 **Multi-Language Support**: Analyze Go, Python, JavaScript, Java, .NET, Ruby, PHP
📦 **Library Detection**: Automatically detect OpenTelemetry libraries and versions
⚠️ **Issue Detection**: Find common problems and get actionable recommendations
🔧 **Extensible**: Add custom detectors and language support
📊 **Multiple Output Formats**: Text, JSON output options

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
brew tap getlawrence/homebrew-tap
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
  -o, --output string         Output format (text, json) (default "text")
```

### `codegen`

Analyze a codebase and generate OpenTelemetry instrumentation using AI or templates.

```bash
lawrence codegen --mode template --method code --path /path/to/project --dry-run

Flags:
  -l, --language string       Target language (go, javascript, python, java, dotnet, ruby, php)
  -m, --method string         Installation method (code, auto, ebpf) (default "code")
  -a, --agent string          Preferred coding agent (gemini, claude, openai, github)
      --list-agents           List available coding agents
      --list-templates        List available templates
      --list-strategies       List available generation strategies
      --mode string           Generation mode (ai, template)
  -o, --output string         Output directory (template mode)
      --dry-run               Show what would be generated without writing files
```

## Supported Languages

| Language   | Library Detection | Import Analysis | Package Files |
|------------|-------------------|-----------------|---------------|
| Go         | ✅                | ✅              | go.mod, go.sum |
| Python     | ✅                | ✅              | requirements.txt, pyproject.toml, setup.py |
| JavaScript | ✅                | ✅              | package.json, lockfiles |
| Java       | ✅                | ✅              | pom.xml, gradle files |
| .NET       | ✅                | ✅              | .csproj, packages.config |
| Ruby       | ✅                | ✅              | Gemfile, Gemfile.lock |
| PHP        | ✅                | ✅              | composer.json, composer.lock |

See [Contributing](#contributing) to add support for your language.

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

## License

MIT License - see [LICENSE](LICENSE) for details.

## Support

- 📖 [Documentation](https://github.com/getlawrence/cli/wiki)
- 🐛 [Issue Tracker](https://github.com/getlawrence/cli/issues)
- 💬 [Discussions](https://github.com/getlawrence/cli/discussions)
# Code Generation Modes

Lawrence CLI now supports two modes for generating OpenTelemetry instrumentation code:

## AI Mode (Default)
Uses AI coding agents to generate and implement instrumentation code:
- **GitHub Copilot**: `--agent=github`
- **Gemini CLI**: `--agent=gemini`  
- **Claude Code**: `--agent=claude`
- **OpenAI**: `--agent=openai`

```bash
# Use AI mode with GitHub Copilot
lawrence codegen --mode=ai --agent=github

# AI mode is default if agents are available
lawrence codegen --agent=github
```

## Template Mode
Directly generates ready-to-use instrumentation code from templates:
- No external dependencies required
- Generates actual code files
- Supports dry-run mode

```bash
# Generate instrumentation code files
lawrence codegen --mode=template --method=code

# Preview what would be generated
lawrence codegen --mode=template --method=code --dry-run

# Specify output directory
lawrence codegen --mode=template --method=code --output=/path/to/output

# Generate for specific language
lawrence codegen --mode=template --method=code --language=python
```

## Supported Languages & Methods

### Python
- **Code Instrumentation**: Generates `otel_instrumentation.py` with manual instrumentation setup
- **Auto Instrumentation**: Generates automatic instrumentation with environment-based configuration

Detected frameworks: Flask, Django, Requests, SQLAlchemy, Psycopg2

### Go  
- **Code Instrumentation**: Generates `otel_instrumentation.go` with manual setup functions
- **Auto Instrumentation**: Generates `otel_auto.go` with automatic configuration

Detected frameworks: HTTP handlers, Gin, Gorilla Mux, SQL drivers

## List Available Options

```bash
# List available generation strategies
lawrence codegen --list-strategies

# List available AI agents
lawrence codegen --list-agents

# List available templates
lawrence codegen --list-templates
```

## Examples

### Python Flask App
```bash
cd my-flask-app
lawrence codegen --mode=template --method=code --language=python
```

Generates `otel_instrumentation.py` with Flask instrumentation ready to import and use.

### Go HTTP Server  
```bash
cd my-go-server
lawrence codegen --mode=template --method=code --language=go
```

Generates `otel_instrumentation.go` with HTTP instrumentation functions.

### AI-Assisted Generation
```bash
cd my-app
lawrence codegen --agent=github --method=code
```

Uses GitHub Copilot to implement comprehensive instrumentation across your codebase.

## Migration from AI-Only Mode

The previous AI-only interface is still supported for backward compatibility:

```bash
# Old syntax (still works)
lawrence codegen --agent=github

# New equivalent syntax
lawrence codegen --mode=ai --agent=github
```

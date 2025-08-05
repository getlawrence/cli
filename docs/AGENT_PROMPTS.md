# Agent Prompt Template System

This document describes the shared prompt template system used by all coding agents in the Lawrence CLI.

## Overview

The prompt template system provides a unified way to generate prompts for different coding agents (Gemini, Claude, OpenAI, GitHub Copilot) while maintaining consistency and flexibility across languages.

## Template Structure

The main agent prompt template is located at `internal/templates/templates/agent_prompt.tmpl`. This template uses Go's text/template syntax and supports the following variables:

### Template Variables

- `{{.Language}}` - The programming language (e.g., "Python", "Go", "JavaScript")
- `{{.Instructions}}` - The specific instrumentation instructions
- `{{.DetectedFrameworks}}` - Array of detected frameworks (e.g., ["Flask", "Django"])
- `{{.ServiceName}}` - Optional service name for the application
- `{{.AdditionalRequirements}}` - Array of additional requirements
- `{{.TemplateContent}}` - Optional template content to include

## Usage

### From Agent Detector

```go
// Create an agent execution request
request := agents.AgentExecutionRequest{
    Language:               "Python",
    Instructions:           "Add OpenTelemetry instrumentation for Flask",
    TargetDir:              "/path/to/project",
    DetectedFrameworks:     []string{"Flask"},
    ServiceName:            "my-service",
    AdditionalRequirements: []string{"Use automatic instrumentation"},
    TemplateContent:        templateContent,
}

// Execute with any agent
err := detector.ExecuteWithAgent(agents.GeminiCLI, request)
```

### From Template Manager

```go
// Create prompt data
promptData := templates.AgentPromptData{
    Language:               "Python",
    Instructions:           "Add OpenTelemetry instrumentation",
    DetectedFrameworks:     []string{"Flask", "SQLAlchemy"},
    ServiceName:            "web-service",
    AdditionalRequirements: []string{"Include logging correlation"},
}

// Generate prompt
prompt, err := templateManager.GenerateAgentPrompt(promptData)
```

## Benefits

1. **Consistency**: All agents receive consistently formatted prompts
2. **Language Agnostic**: The template adapts to any programming language
3. **Extensible**: Easy to add new prompt variables or requirements
4. **Maintainable**: Single template file to update for all agents
5. **Testable**: Template generation can be unit tested

## Adding New Variables

To add new template variables:

1. Update `AgentPromptData` struct in `internal/templates/manager.go`
2. Update `AgentExecutionRequest` struct in `internal/agents/detector.go`
3. Modify the template in `internal/templates/templates/agent_prompt.tmpl`
4. Update this documentation

## Example Generated Prompt

```
You are a coding assistant helping to add OpenTelemetry instrumentation to a Python project.

TASK: Implement OpenTelemetry instrumentation in the Python files in this directory.

INSTRUCTIONS TO FOLLOW:
Add comprehensive OpenTelemetry instrumentation including spans, metrics, and logs.

IMPLEMENTATION REQUIREMENTS:
1. Examine the existing Python files in this directory
2. Modify the main application file to add OpenTelemetry imports and initialization
3. Install and configure the appropriate OpenTelemetry instrumentations for detected frameworks (like Flask, SQLAlchemy)
4. Ensure all changes are properly implemented and saved to the files
5. Follow Python best practices and the provided template exactly
6. Use "web-service" as the service name

Please implement these changes now by modifying the actual files.
```

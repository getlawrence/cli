# OpenTelemetry Knowledge Base

This package provides a comprehensive knowledge base for OpenTelemetry components, automatically fetching and enriching data from the OpenTelemetry Registry and npm.

## Features

- **Automatic Discovery**: Fetches all OpenTelemetry components from the official registry
- **NPM Enrichment**: Enriches component data with detailed npm metadata
- **Structured Storage**: Stores data in a comprehensive, queryable JSON format
- **CLI Interface**: Easy-to-use command-line interface for updates and queries
- **Query Capabilities**: Powerful querying with filters for language, type, name, status, etc.

## Architecture

```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ OpenTelemetry   │    │ NPM Registry     │    │ Knowledge Base  │
│ Registry API    │───▶│ API              │───▶│ Storage & Query │
└─────────────────┘    └──────────────────┘    └─────────────────┘
         │                       │                       │
         ▼                       ▼                       ▼
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│ Component       │    │ Package          │    │ Enhanced        │
│ Discovery       │    │ Metadata         │    │ Components      │
└─────────────────┘    └──────────────────┘    └─────────────────┘
```

## Usage

### Update Knowledge Base

Update the knowledge base for a specific language:

```bash
# Update JavaScript components
lawrence knowledge update javascript

# Update with custom output file
lawrence knowledge update javascript --output ./my-knowledge-base.json

# Force update (ignores existing data)
lawrence knowledge update javascript --force
```

### Query Knowledge Base

Query the knowledge base for specific components:

```bash
# Find all JavaScript instrumentations
lawrence knowledge query --language javascript --type Instrumentation

# Search by component name
lawrence knowledge query --name express

# Find latest versions
lawrence knowledge query --status latest

# Output as JSON
lawrence knowledge query --name http --output json
```

### View Knowledge Base Info

Get information about the current knowledge base:

```bash
# Show basic info
lawrence knowledge info

# Show info in JSON format
lawrence knowledge info --output json
```

## Data Schema

### Component Structure

Each component in the knowledge base includes:

```json
{
  "name": "@opentelemetry/instrumentation-express",
  "type": "Instrumentation",
  "language": "javascript",
  "description": "OpenTelemetry Express instrumentation",
  "repository": "https://github.com/open-telemetry/opentelemetry-js-contrib",
  "versions": [
    {
      "name": "1.5.0",
      "release_date": "2024-05-15T00:00:00Z",
      "dependencies": {
        "typescript": {
          "name": "typescript",
          "version": "^4.8.0",
          "type": "peer"
        }
      },
      "min_runtime_version": ">=18.0.0",
      "status": "latest",
      "deprecated": false
    }
  ]
}
```

### Supported Languages

- JavaScript/Node.js
- Go
- Python
- Java
- C#/.NET
- PHP
- Ruby

### Component Types

- API
- SDK
- Instrumentation
- Exporter
- Propagator
- Sampler
- Processor
- Resource

## Configuration

### Rate Limiting

The pipeline includes rate limiting to respect API limits:

```go
// Default: 100 requests per second
pipeline := pipeline.NewPipeline()

// Custom rate limiting
pipeline := pipeline.NewPipelineWithClients(
    registry.NewClient(),
    npm.NewClient(),
)
```

### Output Files

Knowledge base files are automatically backed up before updates:

```
otel_packages.json          # Current knowledge base
otel_packages.json.backup.20241219-150405  # Previous version
```

## Development

### Adding New Data Sources

To add new data sources, implement the appropriate client interface:

```go
type DataSourceClient interface {
    GetComponents(language string) ([]Component, error)
    GetComponentMetadata(name string) (*Metadata, error)
}
```

### Extending the Schema

To add new fields to the component schema:

1. Update the types in `pkg/knowledge/types/component.go`
2. Modify the pipeline to populate new fields
3. Update the storage layer to handle new data
4. Add query capabilities for new fields

### Testing

Run tests for the knowledge base:

```bash
go test ./pkg/knowledge/...
```

## Examples

### Programmatic Usage

```go
package main

import (
    "github.com/getlawrence/cli/pkg/knowledge/pipeline"
    "github.com/getlawrence/cli/pkg/knowledge/storage"
)

func main() {
    // Create pipeline
    p := pipeline.NewPipeline()
    
    // Update knowledge base
    kb, err := p.UpdateKnowledgeBase("javascript")
    if err != nil {
        panic(err)
    }
    
    // Save to file
    storage := storage.NewStorage("")
    err = storage.SaveKnowledgeBase(kb, "knowledge-base.json")
    if err != nil {
        panic(err)
    }
    
    // Query components
    query := storage.Query{
        Language: "javascript",
        Type:     "Instrumentation",
    }
    
    result := storage.QueryKnowledgeBase(kb, query)
    fmt.Printf("Found %d components\n", result.Total)
}
```

## Troubleshooting

### Common Issues

1. **Rate Limiting**: If you encounter rate limit errors, reduce the rate limit flag
2. **Network Issues**: Ensure you have access to the OpenTelemetry Registry and npm
3. **File Permissions**: Ensure the output directory is writable

### Debug Mode

Enable verbose logging:

```bash
lawrence --verbose knowledge update javascript
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

This project is licensed under the same license as the main Lawrence CLI tool.

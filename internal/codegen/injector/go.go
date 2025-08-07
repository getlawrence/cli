package injector

import "github.com/getlawrence/cli/internal/codegen/types"

// initializeGoConfig sets up the Go language configuration
func InitializeGoConfig() *types.LanguageConfig {
	return &types.LanguageConfig{
		Language:       "Go",
		FileExtensions: []string{".go"},
		ImportQueries: map[string]string{
			"existing_imports": `
				(import_declaration 
					(import_spec 
						path: (interpreted_string_literal) @import_path
					)
				) @import_location
			`,
		},
		FunctionQueries: map[string]string{
			"main_function": `
				(function_declaration 
					name: (identifier) @function_name
					body: (block) @function_body
					(#eq? @function_name "main")
				)
			`,
		},
		InsertionQueries: map[string]string{
			"optimal_insertion": `
				(block
					(var_declaration) @after_variables
				)
				(block
					(call_expression) @before_function_calls
				)
				(block) @function_start
			`,
		},
		ImportTemplate: `import "%s"`,
		InitializationTemplate: `
	// Initialize OpenTelemetry
	tp, err := initializeOTEL()
	if err != nil {
		log.Fatal("Failed to initialize OpenTelemetry:", err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %%v", err)
		}
	}()
`,
		CleanupTemplate: `defer tp.Shutdown(context.Background())`,
	}
}

func GetRequiredGoImports() []string {
	return []string{
		"context",
		"log",
		"go.opentelemetry.io/otel",
		"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp",
		"go.opentelemetry.io/otel/sdk/trace",
	}

}

package injector

import "github.com/getlawrence/cli/internal/codegen/types"

func InitializePythonConfig() *types.LanguageConfig {
	return &types.LanguageConfig{
		Language:       "Python",
		FileExtensions: []string{".py", ".pyw"},
		ImportQueries: map[string]string{
			"existing_imports": `
				(import_statement 
					name: (dotted_name) @import_path
				) @import_location
				(import_from_statement
					module_name: (dotted_name) @import_path
				) @import_location
			`,
		},
		FunctionQueries: map[string]string{
			"main_function": `
				(function_definition 
					name: (identifier) @function_name
					body: (block) @function_body
					(#eq? @function_name "main")
				)
				(if_statement
					condition: (comparison_operator
						left: (identifier) @name_var
						right: (string) @main_str
					)
					body: (block) @function_body
					(#eq? @name_var "__name__")
					(#match? @main_str ".*__main__.*")
				)
			`,
		},
		InsertionQueries: map[string]string{
			"optimal_insertion": `
				(block
					(assignment) @after_variables
				)
				(block
					(expression_statement 
						(call)) @before_function_calls
				)
				(block) @function_start
			`,
		},
		ImportTemplate: `from opentelemetry import %s`,
		InitializationTemplate: `
    # Initialize OpenTelemetry
    tp = initialize_otel()
    import atexit
    atexit.register(lambda: tp.shutdown())
`,
		CleanupTemplate: `tp.shutdown()`,
	}
}

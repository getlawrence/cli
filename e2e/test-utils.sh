#!/bin/bash

# E2E Test Suite Configuration and Utilities

# Expected results for validation
declare -A EXPECTED_GO_PACKAGES=(
    ["gin-gonic"]="detected"
    ["gorilla/mux"]="detected" 
    ["redis"]="detected"
)

declare -A EXPECTED_PYTHON_PACKAGES=(
    ["flask"]="detected"
    ["requests"]="detected"
    ["psycopg2"]="detected"
    ["redis"]="detected"
    ["sqlalchemy"]="detected"
)

declare -A EXPECTED_LANGUAGES=(
    ["go-sample"]="go"
    ["python-sample"]="python"
)

# Validation functions
validate_package_detection() {
    local project_type="$1"
    local output="$2"
    
    case "$project_type" in
        "go")
            for package in "${!EXPECTED_GO_PACKAGES[@]}"; do
                if echo "$output" | jq -e ".packages[] | select(.name | contains(\"$package\"))" >/dev/null 2>&1; then
                    echo "✓ Detected Go package: $package"
                else
                    echo "✗ Missing Go package: $package"
                    return 1
                fi
            done
            ;;
        "python")
            for package in "${!EXPECTED_PYTHON_PACKAGES[@]}"; do
                if echo "$output" | jq -e ".packages[] | select(.name | contains(\"$package\"))" >/dev/null 2>&1; then
                    echo "✓ Detected Python package: $package"
                else
                    echo "✗ Missing Python package: $package"
                    return 1
                fi
            done
            ;;
    esac
    
    return 0
}

validate_language_detection() {
    local project_dir="$1"
    local expected_language="$2"
    local output="$3"
    
    if echo "$output" | jq -e ".languages.root == \"$expected_language\"" >/dev/null 2>&1; then
        echo "✓ Detected correct language: $expected_language"
        return 0
    else
        echo "✗ Failed to detect language: $expected_language"
        echo "Actual languages: $(echo "$output" | jq '.languages')"
        return 1
    fi
}

validate_instrumentation_opportunities() {
    local project_type="$1"
    local output="$2"
    
    case "$project_type" in
        "go")
            # Should detect HTTP instrumentation opportunities
            if echo "$output" | jq -e '.issues[] | select(.category == "instrumentation")' >/dev/null 2>&1; then
                echo "✓ Detected Go instrumentation opportunities"
                return 0
            else
                echo "✗ No Go instrumentation opportunities detected"
                return 1
            fi
            ;;
        "python")
            # Should detect Flask instrumentation opportunities
            if echo "$output" | jq -e '.issues[] | select(.category == "instrumentation")' >/dev/null 2>&1; then
                echo "✓ Detected Python instrumentation opportunities"
                return 0
            else
                echo "✗ No Python instrumentation opportunities detected"
                return 1
            fi
            ;;
    esac
}

# Test data generators
generate_test_go_project() {
    local dir="$1"
    mkdir -p "$dir"
    cd "$dir"
    
    cat > go.mod << 'EOF'
module test-project

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/go-redis/redis/v8 v8.11.5
    go.opentelemetry.io/otel v1.16.0
)
EOF
    
    cat > main.go << 'EOF'
package main

import (
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func main() {
    tracer := otel.Tracer("test-app")
    _, span := tracer.Start(context.Background(), "main")
    defer span.End()
    
    r := gin.Default()
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "instrumented"})
    })
    r.Run()
}
EOF
}

generate_test_python_project() {
    local dir="$1"
    mkdir -p "$dir"
    cd "$dir"
    
    cat > requirements.txt << 'EOF'
flask==2.3.3
requests==2.31.0
opentelemetry-api==1.20.0
opentelemetry-instrumentation-flask==0.41b0
EOF
    
    cat > app.py << 'EOF'
from flask import Flask
from opentelemetry import trace
from opentelemetry.instrumentation.flask import FlaskInstrumentor

app = Flask(__name__)
FlaskInstrumentor().instrument_app(app)

@app.route('/')
def hello():
    tracer = trace.get_tracer(__name__)
    with tracer.start_as_current_span("hello"):
        return {"message": "instrumented"}

if __name__ == '__main__':
    app.run()
EOF
}

# Performance and stress testing
run_performance_tests() {
    local project_dir="$1"
    
    echo "Running performance tests on $project_dir"
    
    # Time the analysis command
    local start_time=$(date +%s%N)
    lawrence analyze --format json > /dev/null 2>&1
    local end_time=$(date +%s%N)
    
    local duration=$(( (end_time - start_time) / 1000000 )) # Convert to milliseconds
    
    echo "Analysis completed in ${duration}ms"
    
    # Check if it's reasonably fast (less than 5 seconds)
    if [[ $duration -lt 5000 ]]; then
        echo "✓ Performance test passed"
        return 0
    else
        echo "✗ Performance test failed - took ${duration}ms"
        return 1
    fi
}

# Export functions for use in main test script
export -f validate_package_detection
export -f validate_language_detection  
export -f validate_instrumentation_opportunities
export -f generate_test_go_project
export -f generate_test_python_project
export -f run_performance_tests

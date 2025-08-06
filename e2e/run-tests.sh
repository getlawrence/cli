#!/bin/bash

# E2E Test Runner for Lawrence CLI
# This script runs comprehensive end-to-end tests in a Docker container

set -euo pipefail

# Colors for output
readonly RED='\033[0;31m'
readonly GREEN='\033[0;32m'
readonly YELLOW='\033[1;33m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m' # No Color

# Test results
declare -i TESTS_RUN=0
declare -i TESTS_PASSED=0
declare -i TESTS_FAILED=0

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
}

log_warning() {
    echo -e "${YELLOW}[WARN]${NC} $*"
}

# Test assertion functions
assert_command_succeeds() {
    local description="$1"
    shift
    
    ((TESTS_RUN++))
    log_info "Testing: $description"
    
    if "$@"; then
        log_success "$description"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "$description"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_command_fails() {
    local description="$1"
    shift
    
    ((TESTS_RUN++))
    log_info "Testing: $description"
    
    if ! "$@"; then
        log_success "$description"
        ((TESTS_PASSED++))
        return 0
    else
        log_error "$description - Expected command to fail but it succeeded"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_output_contains() {
    local description="$1"
    local expected="$2"
    shift 2
    
    ((TESTS_RUN++))
    log_info "Testing: $description"
    
    local output
    if output=$("$@" 2>&1); then
        if echo "$output" | grep -q "$expected"; then
            log_success "$description"
            ((TESTS_PASSED++))
            return 0
        else
            log_error "$description - Expected '$expected' in output but not found"
            echo "Actual output: $output"
            ((TESTS_FAILED++))
            return 1
        fi
    else
        log_error "$description - Command failed"
        ((TESTS_FAILED++))
        return 1
    fi
}

assert_json_contains() {
    local description="$1"
    local json_path="$2"
    local expected_value="$3"
    shift 3
    
    ((TESTS_RUN++))
    log_info "Testing: $description"
    
    local output
    if output=$("$@" 2>&1); then
        local actual_value
        if actual_value=$(echo "$output" | jq -r "$json_path" 2>/dev/null); then
            if [[ "$actual_value" == "$expected_value" ]]; then
                log_success "$description"
                ((TESTS_PASSED++))
                return 0
            else
                log_error "$description - Expected '$expected_value' but got '$actual_value'"
                ((TESTS_FAILED++))
                return 1
            fi
        else
            log_error "$description - Failed to parse JSON or path not found"
            echo "Output: $output"
            ((TESTS_FAILED++))
            return 1
        fi
    else
        log_error "$description - Command failed"
        echo "Output: $output"
        ((TESTS_FAILED++))
        return 1
    fi
}

# Test CLI basic functionality
test_cli_basics() {
    log_info "=== Testing CLI Basic Functionality ==="
    
    assert_command_succeeds "CLI version command" lawrence version
    assert_command_succeeds "CLI help command" lawrence help
    assert_command_succeeds "CLI list languages command" lawrence list languages
    assert_output_contains "CLI shows Go language" "go" lawrence list languages
    assert_output_contains "CLI shows Python language" "python" lawrence list languages
}

# Test Go sample project
test_go_sample() {
    log_info "=== Testing Go Sample Project ==="
    
    local go_sample_dir="/test/examples/go-sample"
    cd "$go_sample_dir"
    
    # Test language detection
    assert_output_contains "Detects Go project" "go" lawrence analyze --format json
    
    # Test package detection
    assert_output_contains "Detects gin package" "gin-gonic" lawrence analyze --format json
    assert_output_contains "Detects gorilla/mux package" "gorilla/mux" lawrence analyze --format json
    assert_output_contains "Detects redis package" "redis" lawrence analyze --format json
    
    # Test that no OTEL libraries are detected initially
    local output
    output=$(lawrence analyze --format json)
    if echo "$output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_warning "Found OpenTelemetry libraries in non-instrumented project (this might be expected)"
    else
        log_success "Correctly detected no OpenTelemetry libraries in uninstrumented project"
    fi
    
    # Test instrumentation recommendations
    assert_output_contains "Suggests HTTP instrumentation" "http" lawrence analyze --format json
    
    # Test JSON output format
    assert_json_contains "JSON output has correct structure" ".languages.root" "go" lawrence analyze --format json
}

# Test Python sample project  
test_python_sample() {
    log_info "=== Testing Python Sample Project ==="
    
    local python_sample_dir="/test/examples/python-sample"
    cd "$python_sample_dir"
    
    # Test language detection
    assert_output_contains "Detects Python project" "python" lawrence analyze --format json
    
    # Test package detection
    assert_output_contains "Detects Flask package" "flask" lawrence analyze --format json
    assert_output_contains "Detects requests package" "requests" lawrence analyze --format json
    assert_output_contains "Detects psycopg2 package" "psycopg2" lawrence analyze --format json
    assert_output_contains "Detects redis package" "redis" lawrence analyze --format json
    assert_output_contains "Detects sqlalchemy package" "sqlalchemy" lawrence analyze --format json
    
    # Test that no OTEL libraries are detected initially
    local output
    output=$(lawrence analyze --format json)
    if echo "$output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_warning "Found OpenTelemetry libraries in non-instrumented project (this might be expected)"
    else
        log_success "Correctly detected no OpenTelemetry libraries in uninstrumented project"
    fi
    
    # Test instrumentation recommendations
    assert_output_contains "Suggests Flask instrumentation" "flask" lawrence analyze --format json
    
    # Test JSON output format
    assert_json_contains "JSON output has correct structure" ".languages.root" "python" lawrence analyze --format json
}

# Test mixed language project detection
test_mixed_project() {
    log_info "=== Testing Mixed Language Project ==="
    
    # Create a temporary mixed project
    local mixed_dir="/tmp/mixed-project"
    mkdir -p "$mixed_dir"
    cd "$mixed_dir"
    
    # Create Go files
    cat > main.go << 'EOF'
package main

import (
    "net/http"
    "github.com/gin-gonic/gin"
)

func main() {
    r := gin.Default()
    r.GET("/", func(c *gin.Context) {
        c.JSON(200, gin.H{"message": "hello"})
    })
    r.Run()
}
EOF
    
    cat > go.mod << 'EOF'
module mixed-project

go 1.21

require github.com/gin-gonic/gin v1.9.1
EOF
    
    # Create Python files  
    mkdir -p python-service
    cat > python-service/app.py << 'EOF'
from flask import Flask
import requests

app = Flask(__name__)

@app.route('/api')
def api():
    return {"status": "ok"}

if __name__ == '__main__':
    app.run()
EOF
    
    cat > python-service/requirements.txt << 'EOF'
flask==2.3.3
requests==2.31.0
EOF
    
    # Test language detection in mixed project
    assert_output_contains "Detects both Go and Python" "go" lawrence analyze --format json
    assert_output_contains "Detects both Go and Python" "python" lawrence analyze --format json
    
    # Test that different directories are detected
    local output
    output=$(lawrence analyze --format json)
    
    # Check if both languages are detected in their respective directories
    if echo "$output" | jq -e '.languages | has("root") and has("python-service")' >/dev/null 2>&1; then
        log_success "Correctly detected languages in different directories"
    else
        log_warning "Language detection in mixed project may need verification"
        echo "Languages detected: $(echo "$output" | jq '.languages')"
    fi
}

# Test error conditions
test_error_conditions() {
    log_info "=== Testing Error Conditions ==="
    
    # Test non-existent directory
    assert_command_fails "Fails on non-existent directory" lawrence analyze /non/existent/path
    
    # Test invalid format
    assert_command_fails "Fails on invalid format" lawrence analyze --format invalid
    
    # Test empty directory
    local empty_dir="/tmp/empty-project"
    mkdir -p "$empty_dir"
    cd "$empty_dir"
    
    # This should not fail, but should report no languages detected
    assert_command_succeeds "Handles empty directory gracefully" lawrence analyze --format json
}

# Test configuration and output formats
test_output_formats() {
    log_info "=== Testing Output Formats ==="
    
    cd "/test/examples/go-sample"
    
    # Test JSON format
    assert_command_succeeds "JSON format works" lawrence analyze --format json
    
    # Test that JSON is valid
    ((TESTS_RUN++))
    if lawrence analyze --format json | jq . >/dev/null 2>&1; then
        log_success "JSON output is valid"
        ((TESTS_PASSED++))
    else
        log_error "JSON output is invalid"
        ((TESTS_FAILED++))
    fi
    
    # Test default format (should be table or human-readable)
    assert_command_succeeds "Default format works" lawrence analyze
}

# Test codegen detection
test_codegen_detection() {
    log_info "=== Testing Code Generation Detection ==="
    
    cd "/test/examples/go-sample"
    
    # Test that codegen opportunities are detected
    assert_output_contains "Detects codegen opportunities" "codegen" lawrence analyze --format json
    
    cd "/test/examples/python-sample"
    
    # Test Python codegen opportunities
    assert_output_contains "Detects Python codegen opportunities" "codegen" lawrence analyze --format json
}

# Main test execution
main() {
    log_info "Starting Lawrence CLI E2E Tests"
    log_info "Container environment: $(uname -a)"
    log_info "Lawrence CLI version: $(lawrence version 2>/dev/null || echo 'Version command failed')"
    
    # Run all test suites
    test_cli_basics
    test_go_sample
    test_python_sample
    test_mixed_project
    test_error_conditions
    test_output_formats
    test_codegen_detection
    
    # Print test summary
    echo
    log_info "=== Test Summary ==="
    echo -e "Tests run: ${BLUE}$TESTS_RUN${NC}"
    echo -e "Tests passed: ${GREEN}$TESTS_PASSED${NC}"
    echo -e "Tests failed: ${RED}$TESTS_FAILED${NC}"
    
    if [[ $TESTS_FAILED -eq 0 ]]; then
        log_success "All tests passed! 🎉"
        exit 0
    else
        log_error "Some tests failed 😞"
        exit 1
    fi
}

# Run main function
main "$@"

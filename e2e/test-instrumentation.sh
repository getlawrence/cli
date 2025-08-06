#!/bin/bash

# Specific tests for instrumented vs non-instrumented projects
# This script tests the CLI's ability to distinguish between projects
# that already have OTEL instrumentation and those that don't

set -euo pipefail

source /test/e2e/test-utils.sh

# Colors for output
readonly GREEN='\033[0;32m'
readonly RED='\033[0;31m'
readonly BLUE='\033[0;34m'
readonly NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $*"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $*"
}

log_error() {
    echo -e "${RED}[FAIL]${NC} $*"
}

test_uninstrumented_vs_instrumented() {
    log_info "=== Testing Instrumented vs Non-Instrumented Detection ==="
    
    # Test 1: Non-instrumented Go project (original example)
    log_info "Testing non-instrumented Go project"
    cd "/test/examples/go-sample"
    
    local uninstrumented_output
    uninstrumented_output=$(lawrence analyze --format json)
    
    # Should NOT contain OpenTelemetry libraries
    if echo "$uninstrumented_output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_error "Non-instrumented project incorrectly shows OTEL libraries"
    else
        log_success "Non-instrumented project correctly shows no OTEL libraries"
    fi
    
    # Should contain instrumentation suggestions
    if echo "$uninstrumented_output" | jq -e '.issues[] | select(.category == "instrumentation")' >/dev/null 2>&1; then
        log_success "Non-instrumented project correctly suggests instrumentation"
    else
        log_error "Non-instrumented project missing instrumentation suggestions"
    fi
    
    # Test 2: Create an instrumented Go project
    log_info "Testing instrumented Go project"
    local instrumented_dir="/tmp/instrumented-go"
    generate_test_go_project "$instrumented_dir"
    cd "$instrumented_dir"
    
    local instrumented_output
    instrumented_output=$(lawrence analyze --format json)
    
    # Should contain OpenTelemetry libraries
    if echo "$instrumented_output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_success "Instrumented project correctly shows OTEL libraries"
    else
        log_error "Instrumented project missing OTEL libraries detection"
    fi
    
    # Test 3: Non-instrumented Python project
    log_info "Testing non-instrumented Python project"
    cd "/test/examples/python-sample"
    
    local python_uninstrumented_output
    python_uninstrumented_output=$(lawrence analyze --format json)
    
    # Should NOT contain OpenTelemetry libraries
    if echo "$python_uninstrumented_output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_error "Non-instrumented Python project incorrectly shows OTEL libraries"
    else
        log_success "Non-instrumented Python project correctly shows no OTEL libraries"
    fi
    
    # Test 4: Create an instrumented Python project
    log_info "Testing instrumented Python project"
    local instrumented_python_dir="/tmp/instrumented-python"
    generate_test_python_project "$instrumented_python_dir"
    cd "$instrumented_python_dir"
    
    local python_instrumented_output
    python_instrumented_output=$(lawrence analyze --format json)
    
    # Should contain OpenTelemetry libraries
    if echo "$python_instrumented_output" | jq -e '.libraries[] | select(.name | contains("opentelemetry"))' >/dev/null 2>&1; then
        log_success "Instrumented Python project correctly shows OTEL libraries"
    else
        log_error "Instrumented Python project missing OTEL libraries detection"
    fi
}

test_framework_specific_detection() {
    log_info "=== Testing Framework-Specific Detection ==="
    
    # Test different web frameworks
    local frameworks_dir="/tmp/frameworks-test"
    mkdir -p "$frameworks_dir"
    
    # Test Express.js detection (if we had Node.js support)
    # This is a placeholder for future expansion
    
    # Test Django detection
    mkdir -p "$frameworks_dir/django-app"
    cd "$frameworks_dir/django-app"
    
    cat > requirements.txt << 'EOF'
django==4.2.0
psycopg2==2.9.7
EOF
    
    cat > manage.py << 'EOF'
import django
from django.conf import settings
from django.http import HttpResponse

def hello(request):
    return HttpResponse("Hello Django")
EOF
    
    local django_output
    django_output=$(lawrence analyze --format json)
    
    if echo "$django_output" | jq -e '.packages[] | select(.name == "django")' >/dev/null 2>&1; then
        log_success "Django framework correctly detected"
    else
        log_error "Failed to detect Django framework"
    fi
    
    # Test FastAPI detection
    mkdir -p "$frameworks_dir/fastapi-app"
    cd "$frameworks_dir/fastapi-app"
    
    cat > requirements.txt << 'EOF'
fastapi==0.104.0
uvicorn==0.24.0
EOF
    
    cat > main.py << 'EOF'
from fastapi import FastAPI

app = FastAPI()

@app.get("/")
def read_root():
    return {"Hello": "FastAPI"}
EOF
    
    local fastapi_output
    fastapi_output=$(lawrence analyze --format json)
    
    if echo "$fastapi_output" | jq -e '.packages[] | select(.name == "fastapi")' >/dev/null 2>&1; then
        log_success "FastAPI framework correctly detected"
    else
        log_error "Failed to detect FastAPI framework"
    fi
}

test_dependency_analysis() {
    log_info "=== Testing Dependency Analysis Accuracy ==="
    
    # Test complex dependency scenarios
    local complex_dir="/tmp/complex-deps"
    mkdir -p "$complex_dir"
    cd "$complex_dir"
    
    # Create a Go project with many dependencies
    cat > go.mod << 'EOF'
module complex-app

go 1.21

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/gorilla/mux v1.8.0
    github.com/go-redis/redis/v8 v8.11.5
    gorm.io/gorm v1.25.0
    gorm.io/driver/postgres v1.5.0
    github.com/prometheus/client_golang v1.17.0
    go.opentelemetry.io/otel v1.16.0
    go.opentelemetry.io/otel/exporters/jaeger v1.16.0
    go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin v0.42.0
)
EOF
    
    cat > main.go << 'EOF'
package main

import (
    "github.com/gin-gonic/gin"
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
    "gorm.io/gorm"
    "github.com/prometheus/client_golang/prometheus"
)

func main() {
    r := gin.Default()
    r.Use(otelgin.Middleware("complex-app"))
    
    // Complex application with multiple integrations
    r.GET("/metrics", gin.WrapH(prometheus.Handler()))
    r.Run()
}
EOF
    
    local complex_output
    complex_output=$(lawrence analyze --format json)
    
    # Verify comprehensive detection
    local expected_packages=("gin-gonic" "gorm" "prometheus" "opentelemetry")
    
    for package in "${expected_packages[@]}"; do
        if echo "$complex_output" | jq -e ".packages[] | select(.name | contains(\"$package\"))" >/dev/null 2>&1; then
            log_success "Detected complex dependency: $package"
        else
            log_error "Failed to detect complex dependency: $package"
        fi
    done
    
    # Check if it distinguishes between OTEL and regular packages
    local otel_libs
    otel_libs=$(echo "$complex_output" | jq '.libraries[] | select(.name | contains("opentelemetry")) | .name')
    
    if [[ -n "$otel_libs" ]]; then
        log_success "Correctly identified OTEL libraries in complex project"
        echo "OTEL libraries found: $otel_libs"
    else
        log_error "Failed to identify OTEL libraries in complex project"
    fi
}

main() {
    log_info "Running Lawrence CLI Instrumentation Detection Tests"
    
    test_uninstrumented_vs_instrumented
    test_framework_specific_detection
    test_dependency_analysis
    
    log_info "Instrumentation detection tests completed"
}

main "$@"

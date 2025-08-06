#!/bin/bash

# E2E Test Structure Validation
# This script validates the E2E test setup without requiring Docker

set -euo pipefail

readonly GREEN='\033[0;32m'
readonly RED='\033[0;31m'
readonly YELLOW='\033[1;33m'
readonly NC='\033[0m'

log_info() {
    echo -e "ℹ️  $*"
}

log_success() {
    echo -e "${GREEN}✅ $*${NC}"
}

log_error() {
    echo -e "${RED}❌ $*${NC}"
}

log_warning() {
    echo -e "${YELLOW}⚠️  $*${NC}"
}

check_file_exists() {
    local file="$1"
    local description="$2"
    
    if [[ -f "$file" ]]; then
        log_success "$description exists: $file"
        return 0
    else
        log_error "$description missing: $file"
        return 1
    fi
}

check_directory_exists() {
    local dir="$1"
    local description="$2"
    
    if [[ -d "$dir" ]]; then
        log_success "$description exists: $dir"
        return 0
    else
        log_error "$description missing: $dir"
        return 1
    fi
}

check_executable() {
    local file="$1"
    local description="$2"
    
    if [[ -x "$file" ]]; then
        log_success "$description is executable: $file"
        return 0
    else
        log_error "$description is not executable: $file"
        return 1
    fi
}

validate_json_examples() {
    log_info "Validating example project structure..."
    
    # Check Go sample
    if [[ -d "examples/go-sample" ]]; then
        if [[ -f "examples/go-sample/go.mod" ]] && [[ -f "examples/go-sample/main.go" ]]; then
            log_success "Go sample project structure is valid"
        else
            log_error "Go sample project missing required files"
        fi
    else
        log_error "Go sample project directory missing"
    fi
    
    # Check Python sample
    if [[ -d "examples/python-sample" ]]; then
        if [[ -f "examples/python-sample/requirements.txt" ]] && [[ -f "examples/python-sample/app.py" ]]; then
            log_success "Python sample project structure is valid"
        else
            log_error "Python sample project missing required files"
        fi
    else
        log_error "Python sample project directory missing"
    fi
}

validate_cli_build() {
    log_info "Validating CLI can be built..."
    
    if command -v go >/dev/null 2>&1; then
        if go build -o lawrence-test . >/dev/null 2>&1; then
            log_success "CLI builds successfully"
            rm -f lawrence-test
        else
            log_error "CLI build failed"
        fi
    else
        log_warning "Go not installed, skipping build test"
    fi
}

validate_script_syntax() {
    log_info "Validating shell script syntax..."
    
    local scripts=(
        "e2e/run-tests.sh"
        "e2e/test-utils.sh"
        "e2e/test-instrumentation.sh"
    )
    
    for script in "${scripts[@]}"; do
        if [[ -f "$script" ]]; then
            if bash -n "$script" 2>/dev/null; then
                log_success "Script syntax valid: $script"
            else
                log_error "Script syntax error: $script"
            fi
        fi
    done
}

main() {
    log_info "Lawrence CLI E2E Test Structure Validation"
    log_info "=========================================="
    
    local errors=0
    
    # Check core E2E files
    check_file_exists "Dockerfile.e2e" "E2E Dockerfile" || ((errors++))
    check_file_exists "docker-compose.e2e.yml" "Docker Compose file" || ((errors++))
    check_file_exists ".github/workflows/e2e.yml" "GitHub Actions workflow" || ((errors++))
    
    # Check E2E directory structure
    check_directory_exists "e2e" "E2E directory" || ((errors++))
    check_file_exists "e2e/run-tests.sh" "Main test runner" || ((errors++))
    check_file_exists "e2e/test-utils.sh" "Test utilities" || ((errors++))
    check_file_exists "e2e/test-instrumentation.sh" "Instrumentation tests" || ((errors++))
    check_file_exists "e2e/README.md" "E2E documentation" || ((errors++))
    
    # Check executability
    check_executable "e2e/run-tests.sh" "Main test runner" || ((errors++))
    check_executable "e2e/test-instrumentation.sh" "Instrumentation tests" || ((errors++))
    
    # Check example projects
    check_directory_exists "examples" "Examples directory" || ((errors++))
    check_directory_exists "examples/go-sample" "Go sample project" || ((errors++))
    check_directory_exists "examples/python-sample" "Python sample project" || ((errors++))
    
    # Validate content
    validate_json_examples
    validate_cli_build
    validate_script_syntax
    
    # Check Makefile targets
    if grep -q "e2e-test:" Makefile 2>/dev/null; then
        log_success "Makefile E2E targets present"
    else
        log_error "Makefile E2E targets missing"
        ((errors++))
    fi
    
    echo
    if [[ $errors -eq 0 ]]; then
        log_success "All E2E test structure validation checks passed! 🎉"
        log_info "You can now run E2E tests with: make e2e-test"
        exit 0
    else
        log_error "E2E test structure validation failed with $errors errors"
        exit 1
    fi
}

main "$@"

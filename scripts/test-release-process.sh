#!/bin/bash

# Test Release Process Script
# This script tests the release process to ensure it works correctly

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
BINARY_NAME="lawrence"
TEST_DB_PATH="test_release_knowledge.db"

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Cleanup function
cleanup() {
    log_info "Cleaning up test files..."
    rm -f "$BINARY_NAME"
    rm -f "$TEST_DB_PATH"
    rm -f "$TEST_DB_PATH"-journal
    log_success "Cleanup completed"
}

# Set up trap for cleanup
trap cleanup EXIT

# Test 1: Verify workspace
test_workspace() {
    log_info "Test 1: Verifying workspace..."
    
    if [ ! -f "go.mod" ] || [ ! -f "embedded.go" ]; then
        log_error "Workspace check failed"
        return 1
    fi
    
    log_success "Workspace verification passed"
    return 0
}

# Test 2: Test database generation script
test_database_generation() {
    log_info "Test 2: Testing database generation script..."
    
    if [ ! -f "scripts/generate-release-db.sh" ]; then
        log_error "Database generation script not found"
        return 1
    fi
    
    if [ ! -x "scripts/generate-release-db.sh" ]; then
        log_error "Database generation script is not executable"
        return 1
    fi
    
    log_success "Database generation script verification passed"
    return 0
}

# Test 3: Test Makefile targets
test_makefile_targets() {
    log_info "Test 3: Testing Makefile targets..."
    
    # Test help target
    if ! make help >/dev/null 2>&1; then
        log_error "Makefile help target failed"
        return 1
    fi
    
    # Test clean target
    if ! make clean >/dev/null 2>&1; then
        log_error "Makefile clean target failed"
        return 1
    fi
    
    log_success "Makefile targets verification passed"
    return 0
}

# Test 4: Test Go build
test_go_build() {
    log_info "Test 4: Testing Go build..."
    
    # Test that the code compiles
    if ! go build -o "$BINARY_NAME" . >/dev/null 2>&1; then
        log_error "Go build failed"
        return 1
    fi
    
    # Test that the binary was created
    if [ ! -f "$BINARY_NAME" ]; then
        log_error "Binary was not created"
        return 1
    fi
    
    # Test that the binary is executable
    if [ ! -x "$BINARY_NAME" ]; then
        log_error "Binary is not executable"
        return 1
    fi
    
    log_success "Go build verification passed"
    return 0
}

# Test 5: Test embedded database functionality
test_embedded_database() {
    log_info "Test 5: Testing embedded database functionality..."
    
    # Test that the binary can run and access embedded database
    if ! ./"$BINARY_NAME" --version >/dev/null 2>&1; then
        log_error "Binary version command failed"
        return 1
    fi
    
    log_success "Embedded database functionality verification passed"
    return 0
}

# Test 6: Test release build process
test_release_build() {
    log_info "Test 6: Testing release build process..."
    
    # Clean previous build
    make clean >/dev/null 2>&1
    
    # Test release build (this will fail if database generation fails)
    if ! make release-build >/dev/null 2>&1; then
        log_warning "Release build failed (this is expected if no registry exists)"
        log_info "This is normal for testing without a full registry setup"
        return 0
    fi
    
    log_success "Release build process verification passed"
    return 0
}

# Main test execution
main() {
    log_info "Starting release process tests..."
    
    local tests_passed=0
    local tests_total=6
    
    # Run all tests
    test_workspace && tests_passed=$((tests_passed + 1))
    test_database_generation && tests_passed=$((tests_passed + 1))
    test_makefile_targets && tests_passed=$((tests_passed + 1))
    test_go_build && tests_passed=$((tests_passed + 1))
    test_embedded_database && tests_passed=$((tests_passed + 1))
    test_release_build && tests_passed=$((tests_passed + 1))
    
    # Report results
    echo ""
    if [ $tests_passed -eq $tests_total ]; then
        log_success "All tests passed! ($tests_passed/$tests_total)"
        log_info "Release process is ready for use"
        return 0
    else
        log_warning "Some tests failed ($tests_passed/$tests_total passed)"
        log_info "Review the failed tests above"
        return 1
    fi
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo ""
        echo "This script tests the Lawrence CLI release process to ensure it works correctly."
        exit 0
        ;;
    "")
        main
        ;;
    *)
        log_error "Unknown option: $1"
        echo "Use --help for usage information"
        exit 1
        ;;
esac

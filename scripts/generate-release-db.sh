#!/bin/bash

# Generate Release Knowledge Database Script
# This script generates the knowledge database for Lawrence CLI releases
# It ensures the database is up-to-date and properly embedded in the binary

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REGISTRY_DIR=".registry"
DB_OUTPUT_PATH="data/knowledge.db"
BINARY_NAME="lawrence"

# GitHub token (can be set via environment variable or command line)
GITHUB_TOKEN="${GITHUB_TOKEN:-}"

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

# Check if we're in the right directory
check_workspace() {
    if [ ! -f "go.mod" ] || [ ! -f "embedded.go" ]; then
        log_error "This script must be run from the Lawrence CLI workspace root"
        exit 1
    fi
    log_info "Workspace check passed"
}

# Ensure Go is available
check_go() {
    if ! command -v go >/dev/null 2>&1; then
        log_error "Go is not installed or not in PATH"
        exit 1
    fi
    
    GO_VERSION=$(go version | awk '{print $3}')
    log_info "Using Go version: $GO_VERSION"
}

# Sync the registry
sync_registry() {
    log_info "Syncing OpenTelemetry registry..."
    
    if [ ! -d "$REGISTRY_DIR" ]; then
        log_info "Registry directory not found, creating..."
        mkdir -p "$REGISTRY_DIR"
    fi
    
    if ! go run . registry sync; then
        log_error "Failed to sync registry"
        exit 1
    fi
    
    log_success "Registry synced successfully"
}

# Generate the knowledge database
generate_database() {
    log_info "Generating knowledge database..."
    
    # Ensure the data directory exists
    mkdir -p "$(dirname "$DB_OUTPUT_PATH")"
    
    # Build the command with optional GitHub token
    local cmd="go run . knowledge update --output \"$DB_OUTPUT_PATH\" --force"
    
    if [ -n "$GITHUB_TOKEN" ]; then
        log_info "Using GitHub token for enhanced API access"
        cmd="$cmd --token \"$GITHUB_TOKEN\""
    else
        log_warning "No GitHub token provided - API access may be limited"
    fi
    
    # Generate the database
    if ! eval "$cmd"; then
        log_error "Failed to generate knowledge database"
        exit 1
    fi
    
    # Verify the database was created
    if [ ! -f "$DB_OUTPUT_PATH" ]; then
        log_error "Knowledge database was not created at $DB_OUTPUT_PATH"
        exit 1
    fi
    
    # Get database size
    DB_SIZE=$(du -h "$DB_OUTPUT_PATH" | cut -f1)
    log_success "Knowledge database generated successfully at $DB_OUTPUT_PATH (Size: $DB_SIZE)"
}

# Verify the database can be read
verify_database() {
    log_info "Verifying knowledge database..."
    
    # Try to read the database using the CLI
    if ! go run . knowledge status >/dev/null 2>&1; then
        log_error "Failed to verify knowledge database - status command failed"
        exit 1
    fi
    
    log_success "Knowledge database verification passed"
}

# Build the binary to ensure embedding works
build_binary() {
    log_info "Building binary to verify database embedding..."
    
    if ! go build -o "$BINARY_NAME" .; then
        log_error "Failed to build binary"
        exit 1
    fi
    
    # Check if the binary was created
    if [ ! -f "$BINARY_NAME" ]; then
        log_error "Binary was not created"
        exit 1
    fi
    
    # Test that the embedded database works
    if ! ./"$BINARY_NAME" knowledge status >/dev/null 2>&1; then
        log_error "Binary with embedded database failed status check"
        exit 1
    fi
    
    BINARY_SIZE=$(du -h "$BINARY_NAME" | cut -f1)
    log_success "Binary built successfully with embedded database (Size: $BINARY_SIZE)"
    
    # Clean up the binary
    rm -f "$BINARY_NAME"
}

# Main execution
main() {
    log_info "Starting knowledge database generation for release..."
    
    check_workspace
    check_go
    sync_registry
    generate_database
    verify_database
    build_binary
    
    log_success "Release knowledge database generation completed successfully!"
    log_info "The database at $DB_OUTPUT_PATH is ready for embedding in releases"
}

# Handle script arguments
case "${1:-}" in
    --help|-h)
        echo "Usage: $0 [OPTIONS]"
        echo ""
        echo "Options:"
        echo "  --help, -h    Show this help message"
        echo "  --verify      Only verify existing database (don't regenerate)"
        echo "  --token TOKEN Set GitHub token for enhanced API access"
        echo ""
        echo "Environment Variables:"
        echo "  GITHUB_TOKEN  GitHub personal access token for API authentication"
        echo ""
        echo "This script generates the knowledge database for Lawrence CLI releases."
        echo ""
        echo "Examples:"
        echo "  $0                                    # Generate with no token"
        echo "  GITHUB_TOKEN=ghp_xxx $0              # Generate with token from env"
        echo "  $0 --token ghp_xxx                   # Generate with token from arg"
        exit 0
        ;;
    --verify)
        log_info "Verifying existing knowledge database..."
        check_workspace
        check_go
        verify_database
        log_success "Database verification completed successfully!"
        exit 0
        ;;
    --token)
        if [ -z "${2:-}" ]; then
            log_error "Token argument requires a value"
            exit 1
        fi
        GITHUB_TOKEN="$2"
        log_info "Using GitHub token from command line argument"
        main
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

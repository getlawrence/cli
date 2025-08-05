#!/bin/bash
set -e

# Lawrence CLI Installation Script
# Usage: curl -sSL https://raw.githubusercontent.com/getlawrence/cli/main/install.sh | bash

REPO="getlawrence/cli"
BINARY_NAME="lawrence"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

info() {
    echo -e "${GREEN}Info: $1${NC}"
}

warn() {
    echo -e "${YELLOW}Warning: $1${NC}"
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $OS in
        linux*) OS="linux" ;;
        darwin*) OS="darwin" ;;
        msys*|mingw*|cygwin*) OS="windows" ;;
        *) error "Unsupported operating system: $OS" ;;
    esac

    case $ARCH in
        x86_64|amd64) ARCH="amd64" ;;
        arm64|aarch64) ARCH="arm64" ;;
        *) error "Unsupported architecture: $ARCH" ;;
    esac

    info "Detected platform: $OS/$ARCH"
}

# Get the latest release version
get_latest_version() {
    info "Fetching latest release information..."
    LATEST_VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
    
    if [ -z "$LATEST_VERSION" ]; then
        error "Could not fetch the latest version"
    fi
    
    info "Latest version: $LATEST_VERSION"
}

# Download and install the binary
install_binary() {
    BINARY_NAME_WITH_EXT="$BINARY_NAME"
    if [ "$OS" = "windows" ]; then
        BINARY_NAME_WITH_EXT="$BINARY_NAME.exe"
    fi
    
    DOWNLOAD_URL="https://github.com/$REPO/releases/download/$LATEST_VERSION/$BINARY_NAME-$OS-$ARCH"
    if [ "$OS" = "windows" ]; then
        DOWNLOAD_URL="$DOWNLOAD_URL.exe"
    fi
    
    info "Downloading $BINARY_NAME from $DOWNLOAD_URL"
    
    # Create temporary directory
    TMP_DIR=$(mktemp -d)
    TMP_FILE="$TMP_DIR/$BINARY_NAME_WITH_EXT"
    
    # Download the binary
    if command -v curl >/dev/null 2>&1; then
        curl -L "$DOWNLOAD_URL" -o "$TMP_FILE" || error "Failed to download $BINARY_NAME"
    elif command -v wget >/dev/null 2>&1; then
        wget "$DOWNLOAD_URL" -O "$TMP_FILE" || error "Failed to download $BINARY_NAME"
    else
        error "Neither curl nor wget is available"
    fi
    
    # Make it executable
    chmod +x "$TMP_FILE"
    
    # Determine installation directory
    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ -w "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    else
        INSTALL_DIR="$HOME/bin"
        mkdir -p "$INSTALL_DIR"
    fi
    
    # Install the binary
    info "Installing $BINARY_NAME to $INSTALL_DIR"
    mv "$TMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    
    # Clean up
    rm -rf "$TMP_DIR"
    
    info "$BINARY_NAME installed successfully to $INSTALL_DIR"
    
    # Check if the installation directory is in PATH
    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "$INSTALL_DIR is not in your PATH"
        warn "Add the following line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        warn "export PATH=\"$INSTALL_DIR:\$PATH\""
    fi
}

# Verify installation
verify_installation() {
    if command -v "$BINARY_NAME" >/dev/null 2>&1; then
        info "✓ $BINARY_NAME is installed and available in PATH"
        info "Run '$BINARY_NAME --help' to get started"
    else
        warn "$BINARY_NAME is installed but not available in PATH"
        warn "You may need to restart your shell or update your PATH"
    fi
}

# Main installation flow
main() {
    info "Installing Lawrence CLI..."
    
    detect_platform
    get_latest_version
    install_binary
    verify_installation
    
    info "Installation complete!"
}

# Run the installation
main "$@"

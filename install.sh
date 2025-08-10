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

# Download and install the binary from archives
install_binary() {
    local archive_ext
    local asset_name

    if [ "$OS" = "windows" ]; then
        archive_ext="zip"
    else
        archive_ext="tar.gz"
    fi

    asset_name="${BINARY_NAME}_${LATEST_VERSION}_${OS}_${ARCH}.${archive_ext}"
    download_url="https://github.com/${REPO}/releases/download/${LATEST_VERSION}/${asset_name}"

    info "Downloading ${asset_name} from ${download_url}"

    TMP_DIR=$(mktemp -d)
    ARCHIVE_PATH="${TMP_DIR}/${asset_name}"

    if command -v curl >/dev/null 2>&1; then
        curl -fL "${download_url}" -o "${ARCHIVE_PATH}" || error "Failed to download ${asset_name}"
    elif command -v wget >/dev/null 2>&1; then
        wget -O "${ARCHIVE_PATH}" "${download_url}" || error "Failed to download ${asset_name}"
    else
        error "Neither curl nor wget is available"
    fi

    info "Extracting archive"
    if [ "$archive_ext" = "zip" ]; then
        if ! command -v unzip >/dev/null 2>&1; then
            error "unzip is required to extract zip archives"
        fi
        unzip -q "${ARCHIVE_PATH}" -d "${TMP_DIR}"
        BIN_PATH="${TMP_DIR}/${BINARY_NAME}.exe"
    else
        tar -xzf "${ARCHIVE_PATH}" -C "${TMP_DIR}"
        BIN_PATH="${TMP_DIR}/${BINARY_NAME}"
    fi

    [ -f "$BIN_PATH" ] || error "Binary not found in archive"
    chmod +x "$BIN_PATH"

    if [ -w "/usr/local/bin" ]; then
        INSTALL_DIR="/usr/local/bin"
    elif [ -w "$HOME/.local/bin" ]; then
        INSTALL_DIR="$HOME/.local/bin"
        mkdir -p "$INSTALL_DIR"
    else
        INSTALL_DIR="$HOME/bin"
        mkdir -p "$INSTALL_DIR"
    fi

    info "Installing ${BINARY_NAME} to ${INSTALL_DIR}"
    mv "$BIN_PATH" "$INSTALL_DIR/${BINARY_NAME}"

    rm -rf "$TMP_DIR"

    info "${BINARY_NAME} installed successfully to ${INSTALL_DIR}"

    if [[ ":$PATH:" != *":$INSTALL_DIR:"* ]]; then
        warn "$INSTALL_DIR is not in your PATH"
        warn "Add the following line to your shell profile (~/.bashrc, ~/.zshrc, etc.):"
        warn "export PATH=\"$INSTALL_DIR:$PATH\""
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

main() {
    info "Installing Lawrence CLI..."
    detect_platform
    get_latest_version
    install_binary
    verify_installation
    info "Installation complete!"
}

main "$@"

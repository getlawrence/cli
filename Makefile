# Lawrence CLI Makefile

# Build variables
BINARY_NAME=lawrence
VERSION?=dev
GIT_COMMIT?=$(shell git rev-parse HEAD)
BUILD_DATE?=$(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS=-ldflags "-s -w -X github.com/getlawrence/cli/cmd.Version=${VERSION} -X github.com/getlawrence/cli/cmd.GitCommit=${GIT_COMMIT} -X github.com/getlawrence/cli/cmd.BuildDate=${BUILD_DATE}"

# Go parameters
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod

.PHONY: all build clean test deps fmt vet lint install uninstall cross-compile help

all: deps fmt vet test build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@egrep '^(.+)\s*:.*##\s*(.+)' $(MAKEFILE_LIST) | column -t -c 2 -s ':#'
	@echo ''
	@echo 'Cross-compilation targets:'
	@echo '  build-native         - Build for native platforms (macOS AMD64/ARM64)'
	@echo '  cross-compile        - Build for all platforms (requires C compilers)'
	@echo '  goreleaser-local     - Test GoReleaser locally (dry-run)'
	@echo '  goreleaser-build     - Build all targets using GoReleaser locally'
	@echo '  test-windows-arm64   - Test Windows ARM64 build in Docker'
	@echo '  test-all-platforms   - Test all platforms including Windows ARM64'

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the binary
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .

clean: ## Clean build artifacts
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_NAME)-*

test: ## Run tests
	$(GOTEST) -v ./...

test-coverage: ## Run tests with coverage
	$(GOTEST) -v -cover -race ./...

test-coverage-html: ## Run tests with coverage and generate HTML report
	$(GOTEST) -coverprofile=coverage.out ./...
	$(GOCMD) tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report generated: coverage.html"

test-detector: ## Run detector-specific tests
	$(GOTEST) -v ./internal/detector/...

test-watch: ## Run tests in watch mode (requires entr)
	find . -name "*.go" | entr -c make test

fmt: ## Format code
	$(GOCMD) fmt ./...

vet: ## Run go vet
	$(GOCMD) vet ./...

lint: ## Run golangci-lint (requires golangci-lint to be installed)
	golangci-lint run

# E2E Tests
validate-e2e: ## Validate E2E test structure without Docker
	./validate-e2e.sh

e2e-test: ## Run E2E tests in Docker
	docker-compose -f docker-compose.e2e.yml up --build --abort-on-container-exit lawrence-e2e

e2e-test-instrumentation: ## Run instrumentation-specific E2E tests
	docker-compose -f docker-compose.e2e.yml up --build --abort-on-container-exit lawrence-instrumentation-tests

e2e-test-multi: ## Run E2E tests on multiple distributions
	docker-compose -f docker-compose.e2e.yml --profile multi-distro up --build --abort-on-container-exit

e2e-clean: ## Clean up E2E test artifacts
	docker-compose -f docker-compose.e2e.yml down --volumes --remove-orphans
	docker system prune -f

e2e-shell: ## Start an interactive shell in the E2E test environment
	docker-compose -f docker-compose.e2e.yml run --rm lawrence-e2e /bin/bash

install: build ## Install the binary to GOPATH/bin
	cp $(BINARY_NAME) $(GOPATH)/bin/$(BINARY_NAME)

uninstall: ## Remove the binary from GOPATH/bin
	rm -f $(GOPATH)/bin/$(BINARY_NAME)

# Knowledge base commands
generate-kb: build ## Generate knowledge base from local registry
	@echo "Generating knowledge base..."
	@if [ ! -d ".registry" ]; then \
		echo "Local registry not found. Running 'lawrence registry sync' first..."; \
		./$(BINARY_NAME) registry sync; \
	fi
	./$(BINARY_NAME) knowledge update --output pkg/knowledge/storage/knowledge.db --force
	@echo "Knowledge base generated at pkg/knowledge/storage/knowledge.db"

update-embedded-kb: generate-kb ## Update the embedded knowledge database
	@echo "Knowledge base ready for embedding in binary"
	@echo "Run 'make build' to create binary with embedded database"

# Cross compilation
cross-compile: ## Build for multiple platforms
	@echo "Building for multiple platforms..."
	@echo "Note: This requires cross-compilation toolchains. For easier cross-compilation, use 'make goreleaser-local'"
	@if command -v x86_64-linux-musl-gcc >/dev/null 2>&1; then \
		CC=x86_64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .; \
		echo "✓ Built $(BINARY_NAME)-linux-amd64"; \
	else \
		echo "⚠ x86_64-linux-musl-gcc not found, skipping linux-amd64"; \
	fi
	@if command -v aarch64-linux-musl-gcc >/dev/null 2>&1; then \
		CC=aarch64-linux-musl-gcc CGO_ENABLED=1 GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .; \
		echo "✓ Built $(BINARY_NAME)-linux-arm64"; \
	else \
		echo "⚠ aarch64-linux-musl-gcc not found, skipping linux-arm64"; \
	fi
	@if command -v x86_64-w64-mingw32-gcc >/dev/null 2>&1; then \
		CC=x86_64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .; \
		echo "✓ Built $(BINARY_NAME)-windows-amd64.exe"; \
	else \
		echo "⚠ x86_64-w64-mingw32-gcc not found, skipping windows-amd64"; \
	fi
	@if command -v aarch64-w64-mingw32-gcc >/dev/null 2>&1; then \
		CC=aarch64-w64-mingw32-gcc CGO_ENABLED=1 GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-arm64.exe .; \
		echo "✓ Built $(BINARY_NAME)-windows-arm64.exe"; \
	else \
		echo "⚠ aarch64-w64-mingw32-gcc not found, skipping windows-arm64"; \
	fi
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .; \
	echo "✓ Built $(BINARY_NAME)-darwin-amd64"
	@CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .; \
	echo "✓ Built $(BINARY_NAME)-darwin-arm64"
	@echo "Cross-compilation complete!"

# GoReleaser local testing
goreleaser-local: ## Test GoReleaser locally (dry-run)
	@echo "Testing GoReleaser locally..."
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "Installing GoReleaser..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	fi
	goreleaser release --snapshot --clean

goreleaser-build: ## Build all targets using GoReleaser locally
	@echo "Building all targets using GoReleaser..."
	@if ! command -v goreleaser >/dev/null 2>&1; then \
		echo "Installing GoReleaser..."; \
		go install github.com/goreleaser/goreleaser@latest; \
	fi
	goreleaser build --snapshot --clean

# Build native platforms only (works without cross-compilation toolchains)
build-native: ## Build for native platforms only (macOS AMD64/ARM64)
	@echo "Building for native platforms..."
	@CGO_ENABLED=1 GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	@CGO_ENABLED=1 GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .
	@echo "✅ Native builds complete: $(BINARY_NAME)-darwin-amd64, $(BINARY_NAME)-darwin-arm64"

# Test Windows ARM64 build in Docker
test-windows-arm64: ## Test Windows ARM64 build in Docker environment
	@echo "Testing Windows ARM64 build in Docker..."
	@echo "This will attempt to build Windows ARM64 in an emulated environment"
	docker compose -f docker-compose.windows-arm64.yml up ubuntu-windows-arm64 --build

# Test all platforms including Windows ARM64
test-all-platforms: ## Test all platforms including Windows ARM64
	@echo "Testing all platforms including Windows ARM64..."
	@echo "1. Testing native builds..."
	@make build-native
	@echo ""
	@echo "2. Testing GoReleaser (5 platforms)..."
	@make goreleaser-local
	@echo ""
	@echo "3. Testing Windows ARM64 in Docker..."
	@make test-windows-arm64

# Cross compilation without CGO (easier for local testing)
cross-compile-no-cgo: ## Build for multiple platforms without CGO (easier local testing)
	@echo "⚠️  Warning: This project requires CGO and cannot be built without it."
	@echo "Use 'make goreleaser-local' instead for local testing."
	@echo "Or install cross-compilation toolchains with './install-cross-compilers.sh'"
	exit 1

# Docker commands
docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	docker run --rm -v $(PWD):/workspace $(BINARY_NAME):latest analyze /workspace

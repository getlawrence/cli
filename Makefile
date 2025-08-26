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

# Get Go environment variables
GOBIN=$(shell $(GOCMD) env GOBIN)
GOPATH=$(shell $(GOCMD) env GOPATH)
ifeq ($(GOBIN),)
GOBIN=$(GOPATH)/bin
endif

.PHONY: all build clean test deps fmt vet lint setup unsetup help

all: deps fmt vet test build

help: ## Show this help message
	@echo 'Usage: make [target]'
	@echo ''
	@echo 'Targets:'
	@egrep '^(.+)\s*:.*##\s*(.+)' $(MAKEFILE_LIST) | column -t -c 2 -s ':#'

deps: ## Download dependencies
	$(GOMOD) download
	$(GOMOD) tidy

build: ## Build the binary
	@if [ ! -f "data/knowledge.db" ]; then \
		echo "Knowledge database not found. Generating from existing data..."; \
		cp pkg/knowledge/pipeline/knowledge.db data/ || echo "Warning: Could not copy knowledge.db to data/"; \
	fi
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

# E2E Tests using examples docker-compose
e2e-test: ## Run E2E tests using examples docker-compose
	@echo "Running E2E tests using examples docker-compose..."
	@if [ ! -f "examples/docker-compose.yml" ]; then \
		echo "Error: examples/docker-compose.yml not found"; \
		exit 1; \
	fi
	cd examples && docker-compose up --build --abort-on-container-exit

e2e-clean: ## Clean up E2E test artifacts
	@if [ -f "examples/docker-compose.yml" ]; then \
		cd examples && docker-compose down --volumes --remove-orphans; \
	fi
	docker system prune -f

# Using 'setup' instead of 'install' to avoid conflicts with GNU Make 3.81's built-in rules
install: build ## Install the binary to GOBIN directory
	@echo "Installing $(BINARY_NAME) to $(GOBIN)/$(BINARY_NAME)"
	@echo "GOBIN: $(GOBIN)"
	@echo "GOPATH: $(GOPATH)"
	cp $(BINARY_NAME) $(GOBIN)/$(BINARY_NAME)
	@echo "Installation complete! You can now run '$(BINARY_NAME)' from anywhere"

uninstall: ## Remove the binary from GOBIN directory
	@echo "Removing $(BINARY_NAME) from $(GOBIN)/$(BINARY_NAME)"
	rm -f $(GOBIN)/$(BINARY_NAME)
	@echo "Uninstallation complete!"

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

# Cross compilation using GoReleaser (recommended approach)
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

# Docker commands
docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	docker run --rm -v $(PWD):/workspace $(BINARY_NAME):latest analyze /workspace

# Release build commands
release-build: clean ## Build release binary with fresh knowledge database
	@echo "Building release binary with embedded knowledge database..."
	@if [ -n "$(GITHUB_TOKEN)" ]; then \
		echo "Using GitHub token for enhanced API access"; \
		GITHUB_TOKEN="$(GITHUB_TOKEN)" ./scripts/generate-release-db.sh; \
	else \
		echo "No GitHub token provided - API access may be limited"; \
		./scripts/generate-release-db.sh; \
	fi
	$(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME) .
	@echo "Release binary built: $(BINARY_NAME)"

# CI/CD pipeline commands
ci-release-db: ## Generate knowledge database for CI/CD releases (no build)
	@echo "Generating knowledge database for CI/CD release..."
	@if [ -n "$(GITHUB_TOKEN)" ]; then \
		echo "Using GitHub token for enhanced API access"; \
		GITHUB_TOKEN="$(GITHUB_TOKEN)" ./scripts/generate-release-db.sh; \
	else \
		echo "No GitHub token provided - API access may be limited"; \
		./scripts/generate-release-db.sh; \
	fi

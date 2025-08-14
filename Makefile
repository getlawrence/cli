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

# Cross compilation
cross-compile: ## Build for multiple platforms
	GOOS=linux GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-amd64 .
	GOOS=linux GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-linux-arm64 .
	GOOS=windows GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-amd64.exe .
	GOOS=windows GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-windows-arm64.exe .
	GOOS=darwin GOARCH=amd64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-amd64 .
	GOOS=darwin GOARCH=arm64 $(GOBUILD) $(LDFLAGS) -o $(BINARY_NAME)-darwin-arm64 .

# Docker commands
docker-build: ## Build Docker image
	docker build -t $(BINARY_NAME):latest .

docker-run: ## Run Docker container
	docker run --rm -v $(PWD):/workspace $(BINARY_NAME):latest analyze /workspace

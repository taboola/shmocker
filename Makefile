# Build variables
BINARY_NAME := shmocker
BINARY_PATH := ./bin/$(BINARY_NAME)
MAIN_PATH := ./cmd/shmocker
GO_VERSION := 1.21

# Build flags for static linking
CGO_ENABLED := 0
LDFLAGS := -ldflags '-extldflags "-static" -s -w'
BUILD_TAGS := netgo,osusergo

# Git info
GIT_COMMIT := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
GIT_TAG := $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0")
BUILD_TIME := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# Enhanced LDFLAGS with version info
LDFLAGS_WITH_VERSION := -ldflags '-extldflags "-static" -s -w -X main.version=$(GIT_TAG) -X main.commit=$(GIT_COMMIT) -X main.buildTime=$(BUILD_TIME)'

.PHONY: help build test lint clean install deps fmt vet security audit release

help: ## Show this help message
	@echo 'Usage: make <target>'
	@echo ''
	@echo 'Targets:'
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  %-15s %s\n", $$1, $$2}'

build: ## Build the binary with static linking
	@echo "Building $(BINARY_NAME)..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) GOOS=linux GOARCH=amd64 go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o $(BINARY_PATH) \
		$(MAIN_PATH)
	@echo "Built $(BINARY_PATH)"

build-local: ## Build binary for local OS
	@echo "Building $(BINARY_NAME) for local OS..."
	@mkdir -p bin
	CGO_ENABLED=$(CGO_ENABLED) go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o ./bin/$(BINARY_NAME)-local \
		$(MAIN_PATH)
	@echo "Built ./bin/$(BINARY_NAME)-local"

test: ## Run tests
	@echo "Running tests..."
	go test -v -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

test-short: ## Run short tests
	@echo "Running short tests..."
	go test -short -v ./...

lint: ## Run linters
	@echo "Running linters..."
	@which golangci-lint > /dev/null || (echo "Installing golangci-lint..." && go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest)
	golangci-lint run --timeout=5m

fmt: ## Format code
	@echo "Formatting code..."
	go fmt ./...
	goimports -w .

vet: ## Run go vet
	@echo "Running go vet..."
	go vet ./...

security: ## Run security checks
	@echo "Running security checks..."
	@which gosec > /dev/null || (echo "Installing gosec..." && go install github.com/securecodewarrior/gosec/v2/cmd/gosec@latest)
	gosec ./...

audit: ## Audit dependencies for vulnerabilities
	@echo "Auditing dependencies..."
	go list -json -deps ./... | nancy sleuth

deps: ## Download and verify dependencies
	@echo "Downloading dependencies..."
	go mod download
	go mod verify

clean: ## Clean build artifacts
	@echo "Cleaning..."
	rm -rf bin/
	rm -f coverage.out coverage.html
	go clean -cache
	go clean -testcache

install: build ## Install the binary to GOPATH/bin
	@echo "Installing $(BINARY_NAME)..."
	go install -tags "$(BUILD_TAGS)" $(LDFLAGS_WITH_VERSION) $(MAIN_PATH)

release: ## Build release binaries for multiple platforms
	@echo "Building release binaries..."
	@mkdir -p bin/release
	# Linux AMD64
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o bin/release/$(BINARY_NAME)-linux-amd64 \
		$(MAIN_PATH)
	# Linux ARM64
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o bin/release/$(BINARY_NAME)-linux-arm64 \
		$(MAIN_PATH)
	# Darwin AMD64
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o bin/release/$(BINARY_NAME)-darwin-amd64 \
		$(MAIN_PATH)
	# Darwin ARM64
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build \
		-tags "$(BUILD_TAGS)" \
		$(LDFLAGS_WITH_VERSION) \
		-o bin/release/$(BINARY_NAME)-darwin-arm64 \
		$(MAIN_PATH)
	@echo "Release binaries built in bin/release/"

.DEFAULT_GOAL := help
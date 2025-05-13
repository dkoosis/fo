# Enhanced Makefile for fo utility with robust development support

# Force bash shell for consistent behavior
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

# Specify phony targets
.PHONY: all build test lint lint-yaml check-line-length clean deps fmt golangci-lint check-gomod check \
        install-tools check-vulns test-verbose vet tree help

# --- Configuration ---
# Colors for output formatting
RESET   := \033[0m
BOLD    := \033[1m
GREEN   := \033[0;32m
YELLOW  := \033[0;33m
RED     := \033[0;31m
BLUE    := \033[0;34m
NC      := $(RESET) # No Color Alias

# Icons for visually distinct output
ICON_START := $(BLUE)▶$(NC)
ICON_OK    := $(GREEN)✓$(NC)
ICON_WARN  := $(YELLOW)⚠$(NC)
ICON_FAIL  := $(RED)✗$(NC)
ICON_INFO  := $(BLUE)ℹ$(NC)

# Variables
SERVICE_NAME := fo
BINARY_NAME := $(SERVICE_NAME)
MODULE_PATH := github.com/davidkoosis/fo
CMD_PATH := ./cmd
SCRIPT_DIR := ./scripts

# Build-time variables for version injection
LOCAL_VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for injecting build information
LDFLAGS := -ldflags "-s -w \
            -X $(MODULE_PATH)/internal/version.Version=$(LOCAL_VERSION) \
            -X $(MODULE_PATH)/internal/version.CommitHash=$(LOCAL_COMMIT_HASH) \
            -X $(MODULE_PATH)/internal/version.BuildDate=$(BUILD_DATE)"

# Tool Versions
GOLANGCILINT_VERSION := latest
GOTESTSUM_VERSION := latest

# Line length check configuration
WARN_LINES := 350
FAIL_LINES := 1500
GO_FILES := $(shell find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*")

# YAML files to lint
YAML_FILES := $(shell find . -type f \( -name "*.yaml" -o -name "*.yml" \) -not -path "./vendor/*" -not -path "./.git/*")

# --- Core Targets ---
# Default target: Run all checks and build
all: check deps fmt lint lint-yaml check-line-length test build
	@printf "$(GREEN)$(BOLD)✨ All tasks completed successfully! ✨$(NC)\n"

# Build the application
build:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Building $(BINARY_NAME)...$(NC)\n"
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) $(CMD_PATH)
	@printf "  $(ICON_OK) $(GREEN)Build successful: $(PWD)/bin/$(BINARY_NAME)$(NC)\n"

# Run tests
test:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running tests...$(NC)\n"
	@go test -race -coverprofile=coverage.out ./...
	@printf "  $(ICON_OK) $(GREEN)Tests passed$(NC)\n"

# Run tests with verbose output
test-verbose:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running verbose tests...$(NC)\n"
	@go test -v -race ./...
	@printf "  $(ICON_OK) $(GREEN)Tests completed$(NC)\n"

# Format code
fmt: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Formatting code...$(NC)\n"
	@golangci-lint fmt ./...
	@go mod tidy -v
	@printf "  $(ICON_OK) $(GREEN)Code formatted$(NC)\n"

# Run linter
lint: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running linter...$(NC)\n"
	@golangci-lint run ./...
	@printf "  $(ICON_OK) $(GREEN)Linting passed$(NC)\n"

# Lint YAML files
lint-yaml:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Linting YAML files...$(NC)\n"
	@if ! command -v yamllint >/dev/null 2>&1; then \
		printf "  $(ICON_WARN) $(YELLOW)yamllint not found. Installing...$(NC)\n"; \
		pip install --user yamllint || pip3 install --user yamllint || \
		(printf "  $(ICON_FAIL) $(RED)Failed to install yamllint$(NC)\n" && exit 1); \
	fi
	@if [ -n "$(YAML_FILES)" ]; then \
		yamllint $(YAML_FILES) && \
		printf "  $(ICON_OK) $(GREEN)YAML linting passed$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)YAML linting failed$(NC)\n" && exit 1); \
	else \
		printf "  $(ICON_INFO) $(YELLOW)No YAML files found$(NC)\n"; \
	fi

ensure-fo:
	@if [ ! -f bin/fo ] || [ bin/fo -ot cmd/main.go ]; then \
		mkdir -p bin && \
		echo "Building fo..." && \
		go build -o bin/fo ./cmd; \
	fi

# Start with one simple target as a test
fo-vet: ensure-fo
	@bin/fo -l "Vetting code" -- go vet ./...


# Check file line length
check-line-length:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking file line length...$(NC)\n"
	@if [ -x "$(SCRIPT_DIR)/check_file_length.sh" ]; then \
		$(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES); \
	else \
		printf "  $(ICON_WARN) $(YELLOW)Script $(SCRIPT_DIR)/check_file_length.sh not found or not executable$(NC)\n"; \
		mkdir -p $(SCRIPT_DIR); \
		printf "  $(ICON_INFO) Installing check script...$(NC)\n"; \
		curl -s https://raw.githubusercontent.com/dkoosis/go-script-examples/main/check_file_length.sh > $(SCRIPT_DIR)/check_file_length.sh 2>/dev/null || \
		(printf "  $(ICON_FAIL) $(RED)Failed to download check_file_length.sh$(NC)\n" && exit 1); \
		chmod +x $(SCRIPT_DIR)/check_file_length.sh; \
		$(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES) || exit 0; \
	fi
	@printf "  $(ICON_OK) $(GREEN)Line length check completed$(NC)\n"

# Verify go.mod setup
check-gomod:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking go.mod module path...$(NC)\n"
	@if [ ! -f "go.mod" ]; then \
		printf "  $(ICON_FAIL) $(RED)go.mod file is missing. Run: go mod init $(MODULE_PATH)$(NC)\n"; \
		exit 1; \
	fi
	@if ! grep -q "^module $(MODULE_PATH)$$" go.mod; then \
		printf "  $(ICON_FAIL) $(RED)go.mod has incorrect module path.$(NC)\n"; \
		printf "    $(ICON_INFO) $(YELLOW)Expected: module $(MODULE_PATH)$(NC)\n"; \
		printf "    $(ICON_INFO) $(YELLOW)Found:    $$(grep "^module" go.mod)$(NC)\n"; \
		exit 1; \
	else \
		printf "  $(ICON_OK) $(GREEN)go.mod has correct module path.$(NC)\n"; \
	fi

# Clean build artifacts
clean:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Cleaning up...$(NC)\n"
	@rm -rf bin/ coverage.out
	@go clean -cache -testcache
	@printf "  $(ICON_OK) $(GREEN)Cleaned$(NC)\n"

# Sync dependencies
deps:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Syncing dependencies...$(NC)\n"
	@go mod tidy -v
	@go mod download
	@printf "  $(ICON_OK) $(GREEN)Dependencies synchronized$(NC)\n"

# Run go vet
vet: ensure-fo
	@bin/fo -l "Vetting code" -- go vet ./...

deps: ensure-fo
	@bin/fo -l "Syncing dependencies" -- sh -c "go mod tidy -v && go mod download"

# Scan for vulnerabilities
check-vulns: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking for vulnerabilities...$(NC)\n"
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		printf "  $(ICON_INFO) $(YELLOW)Installing govulncheck...$(NC)\n"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@govulncheck ./...
	@printf "  $(ICON_OK) $(GREEN)No known vulnerabilities found$(NC)\n"

# Generate project directory tree
tree:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Generating project tree...$(NC)\n"
	@mkdir -p ./docs
	@if ! command -v tree > /dev/null; then \
		printf "  $(ICON_WARN) $(YELLOW)'tree' command not found. Skipping tree generation.$(NC)\n"; \
		printf "  $(ICON_INFO) $(YELLOW)To install on macOS: brew install tree$(NC)\n"; \
		printf "  $(ICON_INFO) $(YELLOW)To install on Debian/Ubuntu: sudo apt-get install tree$(NC)\n"; \
	else \
		tree -F -I "vendor|.git|.idea*|*.DS_Store|$(BINARY_NAME)|coverage.out|bin" --dirsfirst > ./docs/project_directory_tree.txt && \
		printf "  $(ICON_OK) $(GREEN)Project tree generated at ./docs/project_directory_tree.txt$(NC)\n"; \
	fi

# Install required development tools
install-tools:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Installing dev tools...$(NC)\n"
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		printf "  $(ICON_INFO) $(YELLOW)Installing golangci-lint...$(NC)\n"; \
		go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION) && \
		printf "  $(ICON_OK) $(GREEN)golangci-lint installed successfully$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Failed to install golangci-lint$(NC)\n" && exit 1); \
	else \
		printf "  $(ICON_OK) $(GREEN)golangci-lint already installed$(NC)\n"; \
	fi
	@if ! command -v gotestsum >/dev/null 2>&1; then \
		printf "  $(ICON_INFO) $(YELLOW)Installing gotestsum...$(NC)\n"; \
		go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION) && \
		printf "  $(ICON_OK) $(GREEN)gotestsum installed successfully$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Failed to install gotestsum$(NC)\n" && exit 1); \
	else \
		printf "  $(ICON_OK) $(GREEN)gotestsum already installed$(NC)\n"; \
	fi

# Comprehensive environment check
check: install-tools check-gomod
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking environment...$(NC)\n"
	@printf "  $(ICON_INFO) Go: $(shell go version)\n"
	@printf "  $(ICON_INFO) GoLangCI-Lint: $(shell golangci-lint --version 2>/dev/null || echo "Not installed")\n"
	@if command -v gotestsum >/dev/null 2>&1; then \
		printf "  $(ICON_INFO) Gotestsum: $(shell gotestsum --version 2>/dev/null || echo "Version unknown")\n"; \
	else \
		printf "  $(ICON_INFO) Gotestsum: Not installed\n"; \
	fi
	@if command -v tree >/dev/null 2>&1; then \
		printf "  $(ICON_OK) Tree: Installed\n"; \
	else \
		printf "  $(ICON_WARN) Tree: Not installed\n"; \
	fi
	@if command -v yamllint >/dev/null 2>&1; then \
		printf "  $(ICON_OK) Yamllint: $(shell yamllint --version 2>/dev/null || echo "Version unknown")\n"; \
	else \
		printf "  $(ICON_WARN) Yamllint: Not installed\n"; \
	fi
	@printf "  $(ICON_OK) $(GREEN)Environment check completed$(NC)\n"

# Display help information
help:
	@printf "$(BLUE)$(BOLD)$(SERVICE_NAME) Makefile targets:$(NC)\n"
	@printf "  %-25s %s\n" "all" "Run all checks and build (default)"
	@printf "  %-25s %s\n" "build" "Build the application binary"
	@printf "  %-25s %s\n" "test" "Run tests with coverage"
	@printf "  %-25s %s\n" "test-verbose" "Run tests with verbose output"
	@printf "  %-25s %s\n" "fmt" "Format code using golangci-lint fmt"
	@printf "  %-25s %s\n" "lint" "Run linter with golangci-lint"
	@printf "  %-25s %s\n" "lint-yaml" "Lint YAML files with yamllint"
	@printf "  %-25s %s\n" "check-line-length" "Check Go file line count (W:$(WARN_LINES), F:$(FAIL_LINES))"
	@printf "  %-25s %s\n" "clean" "Clean build artifacts and caches"
	@printf "  %-25s %s\n" "deps" "Tidy and download Go module dependencies"
	@printf "  %-25s %s\n" "vet" "Run go vet with output display"
	@printf "  %-25s %s\n" "check-vulns" "Scan for known vulnerabilities"
	@printf "  %-25s %s\n" "tree" "Generate project directory tree view"
	@printf "  %-25s %s\n" "install-tools" "Install/update required Go tools"
	@printf "  %-25s %s\n" "check" "Check environment setup"
	@printf "  %-25s %s\n" "check-gomod" "Verify go.mod configuration"
	@printf "  %-25s %s\n" "help" "Display this help message"
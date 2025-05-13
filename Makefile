# Enhanced Makefile for fo utility with robust development support

# Force bash shell for consistent behavior
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

# Specify phony targets
.PHONY: all build test lint lint-yaml check-line-length clean deps fmt golangci-lint check-gomod check \
        install-tools check-vulns test-verbose vet tree help ensure-fo

# --- Configuration ---
# Variables
SERVICE_NAME := fo
BINARY_NAME  := $(SERVICE_NAME)
MODULE_PATH  := github.com/davidkoosis/fo
CMD_PATH     := ./cmd
SCRIPT_DIR   := ./scripts
FO_CMD       := bin/$(BINARY_NAME) # Path to the fo utility built by this Makefile

# Build-time variables for version injection
LOCAL_VERSION     := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE        := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for injecting build information
LDFLAGS := -ldflags "-s -w \
            -X $(MODULE_PATH)/internal/version.Version=$(LOCAL_VERSION) \
            -X $(MODULE_PATH)/internal/version.CommitHash=$(LOCAL_COMMIT_HASH) \
            -X $(MODULE_PATH)/internal/version.BuildDate=$(BUILD_DATE)"

# Tool Versions
GOLANGCILINT_VERSION := latest
GOTESTSUM_VERSION    := latest

# Line length check configuration
WARN_LINES := 350
FAIL_LINES := 1500
GO_FILES   := $(shell find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*")

# YAML files to lint
YAML_FILES := $(shell find . -type f \( -name "*.yaml" -o -name "*.yml" \) -not -path "./vendor/*" -not -path "./.git/*")

# Color and Icon definitions (can be removed if fo handles all visual output or uses its own config)
# These might still be useful for the 'help' target or any direct echos not wrapped by fo.
RESET   := \033[0m
BOLD    := \033[1m
GREEN   := \033[0;32m
YELLOW  := \033[0;33m
RED     := \033[0;31m
BLUE    := \033[0;34m
NC      := $(RESET) # No Color Alias
ICON_INFO  := $(BLUE)ℹ$(NC) # For informational messages not part of fo's output

# --- Core Targets ---

# Target to ensure fo itself is built
ensure-fo:
	@if [ ! -f "$(FO_CMD)" ] || [ "$(FO_CMD)" -ot "$(CMD_PATH)/main.go" ]; then \
		printf "$(ICON_INFO) $(YELLOW)Building $(BINARY_NAME) utility...$(NC)\n"; \
		mkdir -p bin && \
		go build -o $(FO_CMD) $(CMD_PATH)/main.go; \
	fi

# Default target: Run all checks and build
all: ensure-fo check deps fmt lint lint-yaml check-line-length test build
	@printf "$(GREEN)$(BOLD)✨ All development cycle tasks completed successfully! ✨$(NC)\n"

# Build the application (fo itself)
# This target builds 'fo', so it cannot use 'fo' to wrap its own build command.
build:
	@printf "$(BLUE)$(BOLD)▶ Building $(BINARY_NAME) application...$(NC)\n"
	@mkdir -p bin
	@go build $(LDFLAGS) -o bin/$(BINARY_NAME) $(CMD_PATH)/main.go
	@printf "  $(GREEN)✓ Build successful: $(PWD)/bin/$(BINARY_NAME)$(NC)\n"

# Run tests
test: ensure-fo
	@$(FO_CMD) -l "Running tests" -- go test -race -coverprofile=coverage.out ./...

# Run tests with verbose output using gotestsum (example, can use `go test -v` too)
test-verbose: ensure-fo install-tools
	@$(FO_CMD) -l "Running verbose tests (gotestsum)" -s -- gotestsum --format standard-verbose -- -race -coverprofile=coverage.out ./...
# Alternative if not using gotestsum:
#	@$(FO_CMD) -l "Running verbose tests" -s -- go test -v -race ./...

# Format code
fmt: ensure-fo install-tools
	@$(FO_CMD) -l "Formatting Go code" -- golangci-lint fmt --verbose ./...
	@$(FO_CMD) -l "Tidying go.mod" -- go mod tidy -v

# Run linter
lint: ensure-fo install-tools
	@$(FO_CMD) -l "Running Go linter" -s -- golangci-lint run ./... # -s for potentially verbose output

# Lint YAML files
lint-yaml: ensure-fo
	@if ! command -v yamllint >/dev/null 2>&1; then \
		printf "$(ICON_INFO) $(YELLOW)yamllint not found. Attempting to install...$(NC)\n"; \
		$(FO_CMD) -l "Installing yamllint" -- sh -c "pip install --user yamllint || pip3 install --user yamllint"; \
	fi
	@if [ -n "$(YAML_FILES)" ]; then \
		$(FO_CMD) -l "Linting YAML files" -- yamllint $(YAML_FILES); \
	else \
		printf "$(ICON_INFO) $(YELLOW)No YAML files found to lint.$(NC)\n"; \
	fi

# Check file line length
check-line-length: ensure-fo
	@if [ -x "$(SCRIPT_DIR)/check_file_length.sh" ]; then \
		$(FO_CMD) -l "Checking Go file line lengths" -- $(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES); \
	else \
		printf "$(ICON_INFO) $(YELLOW)Script $(SCRIPT_DIR)/check_file_length.sh not found or not executable. Attempting to download...$(NC)\n"; \
		mkdir -p $(SCRIPT_DIR); \
		$(FO_CMD) -l "Downloading check_file_length.sh" -- curl -sSLo $(SCRIPT_DIR)/check_file_length.sh https://raw.githubusercontent.com/dkoosis/go-script-examples/main/check_file_length.sh && \
		chmod +x $(SCRIPT_DIR)/check_file_length.sh && \
		$(FO_CMD) -l "Checking Go file line lengths (after download)" -- $(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES); \
	fi

# Verify go.mod setup
check-gomod: ensure-fo
	@$(FO_CMD) -l "Checking go.mod module path" -- $(SCRIPT_DIR)/check_go_mod_path.sh $(MODULE_PATH)

# Clean build artifacts
clean: ensure-fo
	@$(FO_CMD) -l "Cleaning artifacts" -- sh -c "rm -rf bin/ coverage.out && go clean -cache -testcache"

# Sync dependencies
deps: ensure-fo
	@$(FO_CMD) -l "Tidying go.mod" -- go mod tidy -v
	@$(FO_CMD) -l "Downloading Go modules" -- go mod download

# Run go vet
vet: ensure-fo
	@$(FO_CMD) -l "Vetting Go code" -- go vet ./...

# Scan for vulnerabilities
check-vulns: ensure-fo install-tools
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		printf "$(ICON_INFO) $(YELLOW)govulncheck not found. Installing...$(NC)\n"; \
		$(FO_CMD) -l "Installing govulncheck" -- go install golang.org/x/vuln/cmd/govulncheck@latest; \
	fi
	@$(FO_CMD) -l "Checking for vulnerabilities" -s -- govulncheck ./...

# Generate project directory tree
tree: ensure-fo
	@if ! command -v tree > /dev/null; then \
		printf "$(ICON_INFO) $(YELLOW)'tree' command not found. Skipping tree generation.$(NC)\n"; \
		printf "  $(ICON_INFO) To install on macOS: brew install tree\n"; \
		printf "  $(ICON_INFO) To install on Debian/Ubuntu: sudo apt-get install tree\n"; \
	else \
		$(FO_CMD) -l "Generating project tree" --tree -F -I 'vendor|.git|.idea*|*.DS_Store|$(BINARY_NAME)|coverage.out|bin' --dirsfirst -o ./docs/project_directory_tree.txt .; \
	fi

# Install required development tools
install-tools: ensure-fo
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		$(FO_CMD) -l "Installing golangci-lint" -- go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION); \
	else \
		printf "$(ICON_INFO) $(GREEN)golangci-lint already installed.$(NC)\n"; \
	fi
	@if ! command -v gotestsum >/dev/null 2>&1; then \
		$(FO_CMD) -l "Installing gotestsum" -- go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); \
	else \
		printf "$(ICON_INFO) $(GREEN)gotestsum already installed.$(NC)\n"; \
	fi

# Comprehensive environment check
check: ensure-fo install-tools check-gomod
	@printf "$(BLUE)$(BOLD)▶ Checking environment...$(NC)\n"
	@printf "  $(ICON_INFO) $(BINARY_NAME) utility: $(shell $(FO_CMD) --version 2>/dev/null || echo "Not built or version arg not supported")\n"
	@printf "  $(ICON_INFO) Go: $(shell go version)\n"
	@printf "  $(ICON_INFO) GoLangCI-Lint: $(shell golangci-lint --version 2>/dev/null || echo "Not installed")\n"
	@printf "  $(ICON_INFO) Gotestsum: $(shell gotestsum --version 2>/dev/null || echo "Not installed")\n"
	@printf "  $(ICON_INFO) Tree: $(shell tree --version 2>/dev/null || echo "Not installed")\n"
	@printf "  $(ICON_INFO) Yamllint: $(shell yamllint --version 2>/dev/null || echo "Not installed")\n"
	@printf "  $(GREEN)✓ Environment check completed$(NC)\n"

# Display help information
help:
	@printf "$(BLUE)$(BOLD)$(SERVICE_NAME) Makefile targets (now using $(FO_CMD) for most tasks):$(NC)\n"
	@printf "  %-25s %s\n" "all" "Run all checks and build (default)"
	@printf "  %-25s %s\n" "ensure-fo" "Build the '$(BINARY_NAME)' utility if needed"
	@printf "  %-25s %s\n" "build" "Build the '$(BINARY_NAME)' application binary (cannot use '$(FO_CMD)')"
	@printf "  %-25s %s\n" "test" "Run tests with coverage"
	@printf "  %-25s %s\n" "test-verbose" "Run tests with verbose output (e.g., using gotestsum)"
	@printf "  %-25s %s\n" "fmt" "Format code using golangci-lint fmt and go mod tidy"
	@printf "  %-25s %s\n" "lint" "Run linter with golangci-lint"
	@printf "  %-25s %s\n" "lint-yaml" "Lint YAML files with yamllint (installs if needed)"
	@printf "  %-25s %s\n" "check-line-length" "Check Go file line count (W:$(WARN_LINES), F:$(FAIL_LINES))"
	@printf "  %-25s %s\n" "clean" "Clean build artifacts and caches"
	@printf "  %-25s %s\n" "deps" "Tidy and download Go module dependencies"
	@printf "  %-25s %s\n" "vet" "Run go vet"
	@printf "  %-25s %s\n" "check-vulns" "Scan for known vulnerabilities (installs govulncheck if needed)"
	@printf "  %-25s %s\n" "tree" "Generate project directory tree view (if 'tree' command is available)"
	@printf "  %-25s %s\n" "install-tools" "Install/update required Go tools (golangci-lint, gotestsum)"
	@printf "  %-25s %s\n" "check" "Perform comprehensive environment and configuration checks"
	@printf "  %-25s %s\n" "check-gomod" "Verify go.mod configuration using script"
	@printf "  %-25s %s\n" "help" "Display this help message"
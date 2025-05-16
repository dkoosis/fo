# Enhanced Makefile for fo utility with robust development support and 'fo' integration
# This version assumes 'fo' has a 'print --type <type> "message"' subcommand.

# Force bash shell for consistent behavior
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

# Specify phony targets
.PHONY: all build test lint lint-yaml check-line-length clean deps fmt golangci-lint check-gomod check \
        install-tools check-vulns test-verbose vet tree help ensure-fo \
        download-check-file-length-script

# --- Variables ---
SERVICE_NAME := fo
BINARY_NAME := $(SERVICE_NAME)
MODULE_PATH := github.com/davidkoosis/fo
CMD_PATH := ./cmd
SCRIPT_DIR := ./scripts
FO := bin/fo

# Fo Flags for use within this Makefile for wrapping commands.
# --ci ensures ASCII output and spinner for wrapped commands.
FO_FLAGS := --ci

# Fo Flags for 'fo print' subcommand.
# We might want colors for these, so we don't use --ci by default.
# If a specific theme is desired for print that differs from command wrapping, set it here.
# For example, to use unicode_vibrant for print messages: FO_PRINT_FLAGS := --theme unicode_vibrant
# For now, let's assume 'fo print' will use the default theme or one specified in .fo.yaml
FO_PRINT_FLAGS :=

# Build-time variables for version injection
LOCAL_VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for injecting build information
LDFLAGS := -ldflags "-s -w \
            -X $(MODULE_PATH)/cmd/internal/version.Version=$(LOCAL_VERSION) \
            -X $(MODULE_PATH)/cmd/internal/version.CommitHash=$(LOCAL_COMMIT_HASH) \
            -X $(MODULE_PATH)/cmd/internal/version.BuildDate=$(BUILD_DATE)"

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
	@$(FO) $(FO_PRINT_FLAGS) print --type success --icon sparkles "All development cycle tasks completed successfully!"
	# The --icon sparkles is hypothetical; 'fo print' would need a way to specify icons
	# or use a default success icon from the theme.

# Ensure fo is built and up-to-date
ensure-fo:
	@if [ ! -f "$(FO)" ] || [ -n "$(shell find cmd -name '*.go' -newer $(FO))" ]; then \
		$(FO) $(FO_PRINT_FLAGS) print --type info "Building $(SERVICE_NAME) utility..."; \
		go build $(LDFLAGS) -o $(FO) $(CMD_PATH); \
	fi

# Build the application (fo itself)
build: ensure-fo
	@$(FO) $(FO_PRINT_FLAGS) print --type info "$(SERVICE_NAME) utility is up to date: $(PWD)/$(FO)"

# Run tests
test: ensure-fo
	$(FO) $(FO_FLAGS) -l "Run tests" -s -- gotestsum --format short -- ./...

# Run tests with verbose output
test-verbose: ensure-fo
	$(FO) $(FO_FLAGS) -l "Run verbose tests" -s -- go test -v -race ./...

# Format code (golangci-lint fmt and go mod tidy)
fmt: install-tools ensure-fo
	$(FO) $(FO_FLAGS) -l "Format Go code" -- golangci-lint fmt ./...
	$(FO) $(FO_FLAGS) -l "Tidy Go modules" -- go mod tidy -v

# Run linter
lint: install-tools ensure-fo
	$(FO) $(FO_FLAGS) -l "Run Go linter" -- golangci-lint run ./...

# Lint YAML files
lint-yaml: install-tools ensure-fo
	@if ! command -v yamllint >/dev/null 2>&1; then \
		$(FO) $(FO_PRINT_FLAGS) print --type warning "Yamllint not found. Skipping YAML lint. Please install it (e.g., pip install yamllint)."; \
	else \
		if [ -n "$(YAML_FILES)" ]; then \
			$(FO) $(FO_FLAGS) -l "Lint YAML files" -- yamllint $(YAML_FILES); \
		else \
			$(FO) $(FO_FLAGS) -l "Lint YAML files (skipped)" -- echo "No YAML files found to lint."; \
		fi \
	fi

# Download check_file_length.sh if it doesn't exist
download-check-file-length-script:
	@mkdir -p $(SCRIPT_DIR)
	@if [ ! -f "$(SCRIPT_DIR)/check_file_length.sh" ]; then \
		$(FO) $(FO_PRINT_FLAGS) print --type info "Downloading check_file_length.sh..."; \
		curl -sfL https://raw.githubusercontent.com/dkoosis/go-script-examples/main/check_file_length.sh -o $(SCRIPT_DIR)/check_file_length.sh && chmod +x $(SCRIPT_DIR)/check_file_length.sh \
			|| ($(FO) $(FO_PRINT_FLAGS) print --type error "Failed to download check_file_length.sh" && exit 1); \
	fi

# Check file line length
check-line-length: ensure-fo download-check-file-length-script
	$(FO) $(FO_FLAGS) -l "Check Go file line lengths" -- $(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES)

# Verify go.mod setup using the provided script
check-gomod: ensure-fo
	@if [ ! -x "$(SCRIPT_DIR)/check_go_mod_path.sh" ]; then \
		$(FO) $(FO_PRINT_FLAGS) print --type warning "$(SCRIPT_DIR)/check_go_mod_path.sh not found or not executable. Skipping go.mod check."; \
	else \
		$(FO) $(FO_FLAGS) -l "Check go.mod module path" -- $(SCRIPT_DIR)/check_go_mod_path.sh $(MODULE_PATH); \
	fi

# Clean build artifacts
clean: ensure-fo
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Cleaning up..."
	@rm -rf $(FO) coverage.out
	@go clean -cache -testcache
	@$(FO) $(FO_PRINT_FLAGS) print --type success "Cleanup complete."

# Sync dependencies
deps: ensure-fo
	$(FO) $(FO_FLAGS) -l "Tidy Go modules" -- go mod tidy -v
	$(FO) $(FO_FLAGS) -l "Download Go modules" -- go mod download

# Run go vet
vet: ensure-fo
	$(FO) $(FO_FLAGS) -l "Vet Go code" -- go vet ./...

# Scan for vulnerabilities
check-vulns: install-tools ensure-fo
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		$(FO) $(FO_PRINT_FLAGS) print --type warning "govulncheck not found. Skipping vulnerability check. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest"; \
	else \
		$(FO) $(FO_FLAGS) -l "Check for vulnerabilities" -- govulncheck ./...; \
	fi

# Generate project directory tree
tree: ensure-fo
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Generating project tree..."
	@mkdir -p ./docs
	@if ! command -v tree > /dev/null; then \
		$(FO) $(FO_PRINT_FLAGS) print --type warning "'tree' command not found. Skipping tree generation."; \
		$(FO) $(FO_PRINT_FLAGS) print --type info "  To install on macOS: brew install tree"; \
		$(FO) $(FO_PRINT_FLAGS) print --type info "  To install on Debian/Ubuntu: sudo apt-get install tree"; \
	else \
		tree -F -I "vendor|.git|.idea*|*.DS_Store|$(BINARY_NAME)|coverage.out|bin" --dirsfirst > ./docs/project_directory_tree.txt && \
		$(FO) $(FO_PRINT_FLAGS) print --type success "Project tree generated at ./docs/project_directory_tree.txt"; \
	fi

# Install required development tools
# This target is more about setup, so its sub-steps print their own info.
install-tools: ensure-fo
	@$(FO) $(FO_PRINT_FLAGS) print --type header "Ensuring development tools..."
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Checking golangci-lint..."
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		$(FO) $(FO_PRINT_FLAGS) print --type info "  Installing golangci-lint@$(GOLANGCILINT_VERSION)..."; \
		GOBIN=$(PWD)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION) && \
		$(FO) $(FO_PRINT_FLAGS) print --type success "  golangci-lint installed successfully to $(PWD)/bin" || \
		($(FO) $(FO_PRINT_FLAGS) print --type error "  Failed to install golangci-lint" && exit 1); \
	else \
		$(FO) $(FO_PRINT_FLAGS) print --type success "  golangci-lint already installed: $(shell which golangci-lint)"; \
	fi
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Checking gotestsum..."
	@if ! command -v gotestsum >/dev/null 2>&1; then \
		$(FO) $(FO_PRINT_FLAGS) print --type info "  Installing gotestsum@$(GOTESTSUM_VERSION)..."; \
		GOBIN=$(PWD)/bin go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION) && \
		$(FO) $(FO_PRINT_FLAGS) print --type success "  gotestsum installed successfully to $(PWD)/bin" || \
		($(FO) $(FO_PRINT_FLAGS) print --type error "  Failed to install gotestsum" && exit 1); \
	else \
		$(FO) $(FO_PRINT_FLAGS) print --type success "  gotestsum already installed: $(shell which gotestsum)"; \
	fi
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Checking yamllint (via pip)..."
	@if ! command -v yamllint >/dev/null 2>&1; then \
		$(FO) $(FO_PRINT_FLAGS) print --type info "  Attempting to install yamllint via pip/pip3..."; \
		python3 -m pip install --user yamllint || python -m pip install --user yamllint || \
		($(FO) $(FO_PRINT_FLAGS) print --type warning "  Could not install yamllint. Please install it manually (e.g., 'pip install yamllint')."); \
		if command -v yamllint >/dev/null 2>&1; then \
			$(FO) $(FO_PRINT_FLAGS) print --type success "  yamllint installed successfully."; \
		fi \
	else \
		$(FO) $(FO_PRINT_FLAGS) print --type success "  yamllint already installed: $(shell which yamllint)"; \
	fi


# Comprehensive environment check
# This target is informational, so direct printf is clearer.
check: install-tools # check-gomod is already wrapped, so it will use fo.
	@$(FO) $(FO_PRINT_FLAGS) print --type header "Checking environment..."
	@$(FO) $(FO_PRINT_FLAGS) print --type info "$(SERVICE_NAME) utility: $(PWD)/$(FO) $(shell $(FO) --version 2>/dev/null)"
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Go: $(shell go version)"
	@$(FO) $(FO_PRINT_FLAGS) print --type info "GoLangCI-Lint: $(shell golangci-lint --version 2>/dev/null || echo "Not detected")"
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Gotestsum: $(shell gotestsum --version 2>/dev/null || echo "Not detected")"
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Tree: $(shell tree --version 2>/dev/null || echo "Not detected")"
	@$(FO) $(FO_PRINT_FLAGS) print --type info "Yamllint: $(shell yamllint --version 2>/dev/null || echo "Not detected")"
	@$(FO) $(FO_PRINT_FLAGS) print --type success "Environment check completed"

# Display help information
help:
	@printf "\033[1m\033[0;34m$(SERVICE_NAME) Makefile targets:\033[0m\n" # Keep help output simple and direct
	@printf "  %-28s %s\n" "all" "Run all checks and build (default)"
	@printf "  %-28s %s\n" "ensure-fo" "Builds the 'fo' utility if missing or outdated"
	@printf "  %-28s %s\n" "build" "Ensures 'fo' utility is up to date"
	@printf "  %-28s %s\n" "test" "Run tests with gotestsum (uses fo)"
	@printf "  %-28s %s\n" "test-verbose" "Run tests with verbose Go output (uses fo)"
	@printf "  %-28s %s\n" "fmt" "Format code and tidy modules (uses fo)"
	@printf "  %-28s %s\n" "lint" "Run Go linter (uses fo)"
	@printf "  %-28s %s\n" "lint-yaml" "Lint YAML files (uses fo)"
	@printf "  %-28s %s\n" "check-line-length" "Check Go file line count (uses fo)"
	@printf "  %-28s %s\n" "clean" "Clean build artifacts and caches (uses fo print)"
	@printf "  %-28s %s\n" "deps" "Tidy and download Go module dependencies (uses fo)"
	@printf "  %-28s %s\n" "vet" "Run go vet (uses fo)"
	@printf "  %-28s %s\n" "check-vulns" "Scan for known vulnerabilities (uses fo if govulncheck installed)"
	@printf "  %-28s %s\n" "tree" "Generate project directory tree view (uses fo print)"
	@printf "  %-28s %s\n" "install-tools" "Install/update required Go tools (uses fo print)"
	@printf "  %-28s %s\n" "check" "Check environment setup (uses fo print)"
	@printf "  %-28s %s\n" "check-gomod" "Verify go.mod configuration (uses fo)"
	@printf "  %-28s %s\n" "help" "Display this help message"

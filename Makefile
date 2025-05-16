# Makefile for fo utility
# IMPORTANT: Lines that are commands under a target (recipes) MUST start with a TAB character, not spaces.

# Force bash shell for consistent behavior
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

# Specify phony targets
.PHONY: all build test lint lint-yaml check-line-length clean deps fmt golangci-lint check-gomod check \
        install-tools check-vulns test-verbose vet tree help ensure-fo \
        download-check-file-length-script

# --- Variables ---
SERVICE_NAME := fo
BINARY_NAME  := $(SERVICE_NAME)
MODULE_PATH  := github.com/davidkoosis/fo
CMD_PATH     := ./cmd
SCRIPT_DIR   := ./scripts
FO_PATH      := ./bin # Store path to fo binary directory
FO           := $(FO_PATH)/$(BINARY_NAME) # Use this for invoking fo

# --- Fo Configuration ---
# Default Fo Flags for command wrapping.
# --ci will be added conditionally based on the CI environment variable.
BASE_FO_FLAGS :=
ifeq ($(CI),true)
    FO_FLAGS := $(BASE_FO_FLAGS) --ci
    FO_PRINT_FLAGS := --ci
else
    FO_FLAGS := $(BASE_FO_FLAGS)
    FO_PRINT_FLAGS :=
endif

# Build-time variables for version injection
LOCAL_VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for injecting build information
LDFLAGS := -ldflags "-s -w \
            -X '$(MODULE_PATH)/cmd/internal/version.Version=$(LOCAL_VERSION)' \
            -X '$(MODULE_PATH)/cmd/internal/version.CommitHash=$(LOCAL_COMMIT_HASH)' \
            -X '$(MODULE_PATH)/cmd/internal/version.BuildDate=$(BUILD_DATE)'"

# Tool Versions
GOLANGCILINT_VERSION := latest
GOTESTSUM_VERSION    := latest

# Line length check configuration
WARN_LINES := 350
FAIL_LINES := 1500
GO_FILES := $(shell find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./bin/*" -not -path "$(FO_PATH)/*")

# YAML files to lint
YAML_FILES := $(shell find . -type f \( -name "*.yaml" -o -name "*.yml" \) -not -path "./vendor/*" -not -path "./.git/*")

# --- Helper Macros for fo ---
# FO_RUN: Wraps a command with fo.
# Arg1: Label for fo
# Arg2: Command to execute
# Arg3: Optional additional fo flags (e.g., --show-output always, -s for stream)
define FO_RUN
    $(FO) $(FO_FLAGS) -l "$(1)" $(3) -- $(2)
endef

# FO_PRINT: Uses 'fo print' for direct messages.
# Arg1: Type (info, success, warning, error, header, raw)
# Arg2: Icon (can be empty to use type default)
# Arg3: Indentation level (0 for no indent)
# Arg4: Message string
define FO_PRINT
    $(FO) $(FO_PRINT_FLAGS) print --type "$(1)" --icon "$(2)" --indent $(3) -- "$(4)"
endef

# --- Core Targets ---
all: ensure-fo check deps fmt lint lint-yaml check-line-length test build
	@$(call FO_PRINT,success,✨,0,All development cycle tasks completed successfully!)

# Ensure fo is built and up-to-date
ensure-fo:
	@mkdir -p $(FO_PATH) # Ensure bin directory exists
	@if [ ! -f "$(FO)" ] || [ -n "$(shell find cmd -name '*.go' -newer $(FO))" ]; then \
	    echo "ℹ️ Building $(SERVICE_NAME) utility (ensure-fo)..."; \
	    go build $(LDFLAGS) -o $(FO) $(CMD_PATH); \
	fi

build: ensure-fo
	@$(call FO_PRINT,success,✅,0,$(SERVICE_NAME) utility is up to date: $(PWD)/$(FO))

test: ensure-fo
	$(call FO_RUN,Run tests,gotestsum --format short -- ./...,-s)

test-verbose: ensure-fo
	$(call FO_RUN,Run verbose tests,go test -v -race ./...,-s)

fmt: install-tools ensure-fo
	$(call FO_RUN,Format Go code,golangci-lint fmt ./...)
	$(call FO_RUN,Tidy Go modules (post-fmt),go mod tidy -v)

lint: install-tools ensure-fo
	$(call FO_RUN,Run Go linter,golangci-lint run ./...)

lint-yaml: install-tools ensure-fo
	@if ! command -v yamllint >/dev/null 2>&1; then \
	    $(call FO_PRINT,warning,⚠️,0,Yamllint not found. Skipping YAML lint. Please install it (e.g., pip install yamllint).); \
	else \
	    if [ -n "$(YAML_FILES)" ]; then \
	        $(call FO_RUN,Lint YAML files,yamllint $(YAML_FILES)); \
	    else \
	        $(call FO_RUN,Lint YAML files (skipped),echo "No YAML files found to lint."); \
	    fi \
	fi

download-check-file-length-script:
	@mkdir -p $(SCRIPT_DIR)
	@if [ ! -f "$(SCRIPT_DIR)/check_file_length.sh" ]; then \
	    $(call FO_PRINT,info,ℹ️,0,Downloading check_file_length.sh...); \
	    curl -sfL https://raw.githubusercontent.com/dkoosis/go-script-examples/main/check_file_length.sh -o $(SCRIPT_DIR)/check_file_length.sh && chmod +x $(SCRIPT_DIR)/check_file_length.sh \
	        || ($(call FO_PRINT,error,❌,0,Failed to download check_file_length.sh) && exit 1); \
	fi

check-line-length: ensure-fo download-check-file-length-script
	$(call FO_RUN,Check Go file line lengths,$(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES))

check-gomod: ensure-fo
	@if [ ! -x "$(SCRIPT_DIR)/check_go_mod_path.sh" ]; then \
	    $(call FO_PRINT,warning,⚠️,0,$(SCRIPT_DIR)/check_go_mod_path.sh not found or not executable. Skipping go.mod check.); \
	else \
	    $(call FO_RUN,Check go.mod module path,$(SCRIPT_DIR)/check_go_mod_path.sh $(MODULE_PATH)); \
	fi

clean: ensure-fo
	@$(call FO_PRINT,info,🧹,0,Cleaning up...)
	@rm -rf $(FO_PATH) coverage.out # Remove ./bin directory
	@go clean -cache -testcache
	@$(call FO_PRINT,success,✅,0,Cleanup complete.)

deps: ensure-fo
	$(call FO_RUN,Tidy Go modules,go mod tidy -v)
	$(call FO_RUN,Download Go modules,go mod download)

vet: ensure-fo
	$(call FO_RUN,Vet Go code,go vet ./...)

check-vulns: install-tools ensure-fo
	@if ! command -v govulncheck >/dev/null 2>&1; then \
	    $(call FO_PRINT,warning,⚠️,0,govulncheck not found. Skipping vulnerability check. Install with: go install golang.org/x/vuln/cmd/govulncheck@latest); \
	else \
	    $(call FO_RUN,Check for vulnerabilities,govulncheck ./...); \
	fi

# Generate project directory tree
tree: ensure-fo
	@$(call FO_PRINT,info,🌲,0,Generating project tree...)
	@mkdir -p ./docs
	@if ! command -v tree > /dev/null; then \
	    $(call FO_PRINT,warning,⚠️,0,'tree' command not found. Skipping tree generation.); \
	    $(call FO_PRINT,info,ℹ️,1,To install on macOS: brew install tree); \
	    $(call FO_PRINT,info,ℹ️,1,To install on Debian/Ubuntu: sudo apt-get install tree); \
	else \
	    if tree -F -I "vendor|.git|.idea*|*.DS_Store|coverage.out|$(FO_PATH)" --dirsfirst -o ./docs/project_folder_tree.txt .; then \
	        $(call FO_PRINT,success,✅,0,Project tree generated at ./docs/project_folder_tree.txt); \
	    else \
	        $(call FO_PRINT,error,❌,0,Failed to generate project tree. 'tree' command may have failed.); \
	        $(call FO_PRINT,info,ℹ️,1,Check for errors above or try running the tree command manually with the same options.); \
	        rm -f ./docs/project_folder_tree.txt; \
	    fi \
	fi

# Install required development tools - Revised for desired output
# This target manages its own output carefully.
install-tools: ensure-fo
	@$(call FO_PRINT,header,▶️,0,Check for development tools)
	@echo -ne "$($(call FO_PRINT,info,ℹ️,1,Check for golangci-lint...)\033[0K)" # Print initial check line, \033[0K clears rest of line
	@path_to_tool=$$(which golangci-lint 2>/dev/null); \
	if [ -n "$$path_to_tool" ]; then \
	    echo -ne "\r$($(call FO_PRINT,success,✅,1,golangci-lint already installed: $$path_to_tool)\033[0K)\n"; \
	else \
	    echo -ne "\r$($(call FO_PRINT,info,ℹ️,1,Installing golangci-lint@$(GOLANGCILINT_VERSION)...)\033[0K)"; \
	    if GOBIN=$(PWD)/$(FO_PATH) go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION); then \
	        echo -ne "\r$($(call FO_PRINT,success,✅,1,golangci-lint installed to $(PWD)/$(FO_PATH))\033[0K)\n"; \
	    else \
	        echo -ne "\r$($(call FO_PRINT,error,❌,1,Failed to install golangci-lint)\033[0K)\n" && exit 1; \
	    fi \
	fi
	@echo -ne "$($(call FO_PRINT,info,ℹ️,1,Check for gotestsum...)\033[0K)"
	@path_to_tool=$$(which gotestsum 2>/dev/null); \
	if [ -n "$$path_to_tool" ]; then \
	    echo -ne "\r$($(call FO_PRINT,success,✅,1,gotestsum already installed: $$path_to_tool)\033[0K)\n"; \
	else \
	    echo -ne "\r$($(call FO_PRINT,info,ℹ️,1,Installing gotestsum@$(GOTESTSUM_VERSION)...)\033[0K)"; \
	    if GOBIN=$(PWD)/$(FO_PATH) go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION); then \
	        echo -ne "\r$($(call FO_PRINT,success,✅,1,gotestsum installed to $(PWD)/$(FO_PATH))\033[0K)\n"; \
	    else \
	        echo -ne "\r$($(call FO_PRINT,error,❌,1,Failed to install gotestsum)\033[0K)\n" && exit 1; \
	    fi \
	fi
	@echo -ne "$($(call FO_PRINT,info,ℹ️,1,Check for yamllint (via pip)...)\033[0K)"
	@path_to_tool=$$(which yamllint 2>/dev/null); \
	if [ -n "$$path_to_tool" ]; then \
	    echo -ne "\r$($(call FO_PRINT,success,✅,1,yamllint already installed: $$path_to_tool)\033[0K)\n"; \
	else \
	    echo -ne "\r$($(call FO_PRINT,info,ℹ️,1,Attempting to install yamllint via pip/pip3...)\033[0K)"; \
	    if python3 -m pip install --user yamllint >/dev/null 2>&1 || python -m pip install --user yamllint >/dev/null 2>&1; then \
	        if command -v yamllint >/dev/null 2>&1; then \
	             echo -ne "\r$($(call FO_PRINT,success,✅,1,yamllint installed successfully: $$(which yamllint))\033[0K)\n"; \
	        else \
	             echo -ne "\r$($(call FO_PRINT,warning,⚠️,1,yamllint installed but not found in PATH. Please check your pip user bin path.)\033[0K)\n"; \
	        fi \
	    else \
	        echo -ne "\r$($(call FO_PRINT,warning,⚠️,1,Could not install yamllint. Please install it manually.)\033[0K)\n"; \
	    fi \
	fi


# Comprehensive environment check
check: install-tools check-gomod # Removed ensure-fo as install-tools depends on it
	@$(call FO_PRINT,header,▶️,0,Check environment)
	@$(call FO_PRINT,info,ℹ️,1,$(SERVICE_NAME) utility: $(PWD)/$(FO) $(shell $(FO) --version 2>/dev/null | head -n 1))
	@$(call FO_PRINT,info,ℹ️,1,Go: $(shell go version))
	@$(call FO_PRINT,info,ℹ️,1,GoLangCI-Lint: $(shell golangci-lint --version 2>/dev/null || echo "Not detected"))
	@$(call FO_PRINT,info,ℹ️,1,Gotestsum: $(shell gotestsum --version 2>/dev/null || echo "Not detected"))
	@$(FO) $(FO_FLAGS) -l "Check Tree version" --show-output always -- bash -c 'v=$$(tree --version 2>/dev/null || echo "Tree: Not detected"); echo "$$v"'
	@$(FO) $(FO_FLAGS) -l "Check Yamllint version" --show-output always -- bash -c 'v=$$(yamllint --version 2>/dev/null || echo "Yamllint: Not detected"); echo "$$v"'
	@$(call FO_PRINT,success,✅,0,Environment check completed)


help:
	@printf "\033[1m\033[0;34m%-28s %s\033[0m\n" "$(SERVICE_NAME) Makefile" "Development Targets"
	@printf "\033[0;34m----------------------------------------------------------------------\033[0m\n"
	@printf "  %-28s %s\n" "all" "Run all checks and build (default)"
	@printf "  %-28s %s\n" "ensure-fo" "Builds the 'fo' utility if missing or outdated"
	@printf "  %-28s %s\n" "build" "Ensures 'fo' utility is up to date"
	@printf "  %-28s %s\n" "test" "Run tests with gotestsum (uses fo)"
	@printf "  %-28s %s\n" "test-verbose" "Run tests with verbose Go output (uses fo)"
	@printf "  %-28s %s\n" "fmt" "Format code and tidy modules (uses fo)"
	@printf "  %-28s %s\n" "lint" "Run Go linter (uses fo)"
	@printf "  %-28s %s\n" "lint-yaml" "Lint YAML files (uses fo if yamllint found)"
	@printf "  %-28s %s\n" "check-line-length" "Check Go file line count (uses fo)"
	@printf "  %-28s %s\n" "clean" "Clean build artifacts and caches"
	@printf "  %-28s %s\n" "deps" "Tidy and download Go module dependencies (uses fo)"
	@printf "  %-28s %s\n" "vet" "Run go vet (uses fo)"
	@printf "  %-28s %s\n" "check-vulns" "Scan for known vulnerabilities (uses fo if govulncheck installed)"
	@printf "  %-28s %s\n" "tree" "Generate project directory tree view"
	@printf "  %-28s %s\n" "install-tools" "Install/update required Go tools and checks status"
	@printf "  %-28s %s\n" "check" "Comprehensive environment check"
	@printf "  %-28s %s\n" "check-gomod" "Verify go.mod configuration (uses fo)"
	@printf "  %-28s %s\n" "help" "Display this help message"


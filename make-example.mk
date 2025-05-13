# Makefile for hello-tool-base
# Incorporates features for robust local development and CI/CD integration.

# Force bash shell for consistent behavior
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

# Specify phony targets (targets not associated with files)
.PHONY: all tree build clean deps fmt lint golangci-lint lint-yaml test test-debug \
        install-tools check-gomod check-line-length check deploy help check-vulns health-check

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

# Formatting Strings for Alignment
LABEL_FMT := "   %-25s" # Indent 3, Pad label to 25 chars, left-aligned

# --- Variables ---
# Application specific variables
SERVICE_NAME := hello-tool-base
BINARY_NAME  := $(SERVICE_NAME) # Binary name will be 'hello-tool-base'

# Go module path - IMPORTANT: Ensure this matches your go.mod file
MODULE_PATH  := github.com/dkoosis/hello-tool-base

# Path to the main package, assuming main.go is in cmd/$(SERVICE_NAME)/
CMD_PATH     := ./cmd/$(SERVICE_NAME)
SCRIPT_DIR   := ./scripts

# Build-time variables for version injection
# These are determined locally from git for the `deploy` target and local `build`
# For builds within Docker triggered by this Makefile, VERSION_ARG and COMMIT_ARG can be passed.
LOCAL_VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# Effective version and commit to use in LDFLAGS.
# Prioritize args passed (e.g., from Docker build ARGs if this Makefile runs inside Docker),
# then fallback to locally determined git values.
VERSION      := $(or $(VERSION_ARG),$(LOCAL_VERSION))
COMMIT_HASH  := $(or $(COMMIT_ARG),$(LOCAL_COMMIT_HASH))
BUILD_DATE   := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

# LDFLAGS for injecting build information into the internal/buildinfo package
LDFLAGS      := -ldflags "-s -w \
                -X $(MODULE_PATH)/internal/buildinfo.Version=$(VERSION) \
                -X $(MODULE_PATH)/internal/buildinfo.CommitHash=$(COMMIT_HASH) \
                -X $(MODULE_PATH)/internal/buildinfo.BuildDate=$(BUILD_DATE)"

# Google Cloud Platform variables
PROJECT_ID   := $(shell gcloud config get-value project 2>/dev/null)
# Define GCP Region
GCP_REGION   := us-central1
# Your Artifact Registry repo name
ARTIFACT_REGISTRY_REPO := my-go-apps
# Default path
CLOUDBUILD_CONFIG_PATH := ./build/cloudbuild/cloudbuild.yaml

# Substitutions for `gcloud builds submit`
# We pass the LOCAL_VERSION and LOCAL_COMMIT_HASH from the Makefile execution context
# These will be used by cloudbuild.yaml as _MAKEFILE_VERSION and _MAKEFILE_COMMIT
GCLOUD_BUILD_SUBSTITUTIONS := _SERVICE_NAME=$(SERVICE_NAME),_REGION=$(GCP_REGION),_ARTIFACT_REGISTRY_REPO=$(ARTIFACT_REGISTRY_REPO),_MODULE_PATH=$(MODULE_PATH),_MAKEFILE_VERSION=$(LOCAL_VERSION),_MAKEFILE_COMMIT=$(LOCAL_COMMIT_HASH)

# Tool Versions (for install-tools target)
GOLANGCILINT_VERSION := latest # Or a specific version e.g., v1.58.0
GOTESTSUM_VERSION    := latest # Or a specific version e.g., v1.11.0

# Line length check configuration
WARN_LINES   := 350  # Warn if lines exceed this
FAIL_LINES   := 1500 # Fail if lines exceed this
# GO_FILES for line length check - adjust if needed (e.g., to exclude more specific test files)
GO_FILES     := $(shell find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*")

# YAML files to lint
YAML_FILES   := $(shell find . -type f \( -name "*.yaml" -o -name "*.yml" \) -not -path "./vendor/*" -not -path "./.git/*")

# --- Core Targets ---
# Default target: Runs general checks, formatting, tests, and then builds the application.
all: tree check-gomod deps fmt golangci-lint lint-yaml check-line-length test build
	@printf "$(GREEN)$(BOLD)✨ All general checks passed and build completed successfully! ✨$(NC)\n"

# Generates a project directory tree view.
tree:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Generating project tree...$(NC)\n"
	@mkdir -p ./docs
	@if ! command -v tree > /dev/null; then \
		printf "   $(ICON_WARN) $(YELLOW)'tree' command not found. Skipping tree generation.$(NC)\n"; \
		printf "   $(ICON_INFO) $(YELLOW)To install on macOS: brew install tree$(NC)\n"; \
		printf "   $(ICON_INFO) $(YELLOW)To install on Debian/Ubuntu: sudo apt-get install tree$(NC)\n"; \
	else \
		tree -F -I 'vendor|.git|.idea*|*.DS_Store|$(BINARY_NAME)|coverage.out' --dirsfirst > ./docs/project_directory_tree.txt && \
		printf "   $(ICON_OK) $(GREEN)Project tree generated at ./docs/project_directory_tree.txt$(NC)\n" || \
		printf "   $(ICON_FAIL) $(RED)Failed to generate project tree.$(NC)\n"; \
	fi
	@printf "\n"

# Builds the application binary for Linux AMD64.
build:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Building $(BINARY_NAME) for linux/amd64...$(NC)\n"
	@printf "  $(ICON_INFO) Using Version: $(VERSION), Commit: $(COMMIT_HASH), BuildDate: $(BUILD_DATE)\n"
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o $(BINARY_NAME) $(CMD_PATH) && \
		printf "  $(ICON_OK) $(GREEN)Build successful: $(PWD)/$(BINARY_NAME)$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Build failed$(NC)\n" && exit 1)
	@printf "\n"

# Cleans build artifacts and Go caches.
clean:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Cleaning build artifacts...$(NC)\n"
	@rm -f $(BINARY_NAME) coverage.out
	@go clean -cache -testcache
	@printf "  $(ICON_OK) $(GREEN)Cleaned$(NC)\n"
	@printf "\n"

# --- Dependency Management ---
# Tidies and downloads Go module dependencies.
deps:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Synchronizing dependencies...$(NC)\n"
	@printf "  $(ICON_INFO) Running go mod tidy...\n"
	@go mod tidy -v && printf "  $(ICON_OK) $(GREEN)Dependencies tidied successfully$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Failed to tidy dependencies$(NC)\n" && exit 1)
	@printf "  $(ICON_INFO) Running go mod download...\n"
	@go mod download && printf "  $(ICON_OK) $(GREEN)Dependencies downloaded successfully$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Failed to download dependencies$(NC)\n" && exit 1)
	@printf "\n"

# Checks if the go.mod file exists and has the correct module path.
check-gomod:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking go.mod module path...$(NC)\n"
	@if [ ! -f "go.mod" ]; then \
		printf "  $(ICON_FAIL) $(RED)go.mod file is missing. Run: go mod init $(MODULE_PATH)$(NC)\n"; \
		exit 1; \
	fi
	@# Use the script if available, otherwise basic grep for a fallback check.
	@if [ -x "$(SCRIPT_DIR)/check_go_mod_path.sh" ]; then \
		"$(SCRIPT_DIR)/check_go_mod_path.sh" $(MODULE_PATH) || exit 1; \
	elif ! grep -q "^module $(MODULE_PATH)$$" go.mod; then \
		printf "  $(ICON_FAIL) $(RED)go.mod has incorrect module path (basic check).$(NC)\n"; \
		printf "    $(ICON_INFO) $(YELLOW)Expected: module $(MODULE_PATH)$(NC)\n"; \
		printf "    $(ICON_INFO) $(YELLOW)Found:    $$(grep '^module' go.mod)$(NC)\n"; \
		exit 1; \
	else \
		printf "  $(ICON_OK) $(GREEN)go.mod has correct module path (basic check).$(NC)\n"; \
	fi
	@printf "\n"

# --- Quality & Testing ---
# Formats Go code using golangci-lint's formatters.
fmt: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Formatting code using golangci-lint fmt...$(NC)\n"
	@golangci-lint fmt ./... && \
		printf "  $(ICON_OK) $(GREEN)Code formatted$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)$(BOLD)Formatting failed (see errors above)$(NC)\n" && exit 1)
	@# Ensure go.mod and go.sum are tidy after formatting, as some formatters might change imports.
	@go mod tidy -v
	@printf "\n"

# Runs basic 'go vet' linter for quick checks.
lint:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running basic linter (go vet)...$(NC)\n"
	@go vet ./... && \
		printf "  $(ICON_OK) $(GREEN)go vet passed$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)go vet found issues$(NC)\n" && exit 1)
	@printf "\n"

# Runs comprehensive linters using golangci-lint.
golangci-lint: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running golangci-lint...$(NC)\n"
	@golangci-lint run ./... && \
		printf "  $(ICON_OK) $(GREEN)golangci-lint passed$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)$(BOLD)golangci-lint failed (see errors above)$(NC)\n" && exit 1)
	@printf "\n"

# Runs YAML linter using yamllint.
lint-yaml:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Linting YAML files...$(NC)\n"
	@# Check if yamllint is installed
	@if ! command -v yamllint >/dev/null 2>&1; then \
		printf "  $(ICON_WARN) $(YELLOW)yamllint not found.$(NC)\n"; \
		if command -v pip >/dev/null 2>&1; then \
			printf "  $(ICON_INFO) $(YELLOW)Attempting to install yamllint using pip...$(NC)\n"; \
			pip install --user yamllint && \
			printf "  $(ICON_OK) $(GREEN)yamllint installed successfully via pip.$(NC)\n" || \
			(printf "  $(ICON_FAIL) $(RED)Failed to install yamllint via pip. Please install it manually.$(NC)\n" && exit 1); \
		elif command -v pip3 >/dev/null 2>&1; then \
			printf "  $(ICON_INFO) $(YELLOW)Attempting to install yamllint using pip3...$(NC)\n"; \
			pip3 install --user yamllint && \
			printf "  $(ICON_OK) $(GREEN)yamllint installed successfully via pip3.$(NC)\n" || \
			(printf "  $(ICON_FAIL) $(RED)Failed to install yamllint via pip3. Please install it manually.$(NC)\n" && exit 1); \
		else \
			printf "  $(ICON_FAIL) $(RED)pip/pip3 not found. Please install yamllint manually (e.g., 'pip install yamllint' or using your system package manager).$(NC)\n"; \
			exit 1; \
		fi; \
		echo "# Ensure yamllint installed via pip is in PATH (common issue)"; \
		if ! command -v yamllint >/dev/null 2>&1; then \
			printf "  $(ICON_WARN) $(YELLOW)yamllint installed but might not be in your PATH. Please check your environment.$(NC)\n"; \
			printf "  $(ICON_INFO) $(YELLOW)Common paths include: ~/.local/bin - ensure this is in your PATH environment variable.$(NC)\n"; \
			exit 1; \
		fi; \
	else \
		printf "  $(ICON_OK) $(GREEN)yamllint found: $$(command -v yamllint)$(NC)\n"; \
	fi
	@# Check if there are any YAML files to lint
	@if [ -z "$(YAML_FILES)" ]; then \
		printf "  $(ICON_INFO) $(YELLOW)No YAML files found to lint.$(NC)\n"; \
	else \
		printf "  $(ICON_INFO) Checking files: $(YAML_FILES)\n"; \
		yamllint $(YAML_FILES); \
		EXIT_CODE=$$?; \
		if [ $$EXIT_CODE -eq 0 ]; then \
			printf "  $(ICON_OK) $(GREEN)YAML linting passed.$(NC)\n"; \
		else \
			printf "  $(ICON_FAIL) $(RED)$(BOLD)YAML linting failed (exit code: $$EXIT_CODE). See errors above.$(NC)\n"; \
			exit $$EXIT_CODE; \
		fi; \
	fi
	@printf "\n"

# Runs tests using gotestsum for better formatted output and race detection.
test: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Running tests with gotestsum...$(NC)\n"
	@gotestsum --format testdox -- -race -coverprofile=coverage.out -covermode=atomic ./... && \
		printf "  $(ICON_OK) $(GREEN)Tests passed$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)$(BOLD)Tests failed$(NC)\n" && exit 1)
	@printf "\n"

# Runs tests with verbose output for debugging purposes.
test-debug: install-tools
	@printf "$(ICON_START) $(BOLD)$(YELLOW)Running tests (verbose debug mode)...$(NC)\n"
	@LOG_LEVEL=debug go test -v -race -count=1 -coverprofile=coverage.out ./... && \
		printf "  $(ICON_OK) $(GREEN)Tests finished (check output for failures)$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)$(BOLD)Tests failed$(NC)\n" && exit 1)
	@printf "\n"

# Checks Go file line lengths using an external script.
check-line-length:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking file lengths (warn > $(WARN_LINES), fail > $(FAIL_LINES))...$(NC)\n"
	@if [ ! -x "$(SCRIPT_DIR)/check_file_length.sh" ]; then \
		printf "   $(ICON_FAIL) $(RED)Error: Script '$(SCRIPT_DIR)/check_file_length.sh' not found or not executable.$(NC)\n"; \
		exit 1; \
	fi
	@$(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES) || \
		(printf "   $(ICON_WARN) $(YELLOW)Line length check reported issues (see script output above)$(NC)\n" && exit 0)
	@printf "\n"

# --- Security Checks ---
# Scans for known vulnerabilities in dependencies. Fails build if any are found.
check-vulns: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Scanning for vulnerabilities with govulncheck...$(NC)\n"
	@if ! command -v govulncheck >/dev/null 2>&1; then \
		printf "  $(ICON_INFO) $(YELLOW)govulncheck not found. Installing...$(NC)\n"; \
		go install golang.org/x/vuln/cmd/govulncheck@latest && \
		printf "  $(ICON_OK) $(GREEN)govulncheck installed successfully.$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)Failed to install govulncheck.$(NC)\n" && exit 1); \
	else \
		printf "  $(ICON_OK) $(GREEN)govulncheck is already installed.$(NC)\n"; \
	fi
	@govulncheck ./... && \
		printf "  $(ICON_OK) $(GREEN)No known vulnerabilities found.$(NC)\n" || \
		(printf "  $(ICON_FAIL) $(RED)$(BOLD)govulncheck found potential vulnerabilities. Deployment will be halted.$(NC)\n" && exit 1) # Fail build if vulnerabilities are found
	@printf "\n"

# --- Tooling & Setup ---
# Installs or updates required Go development tools.
install-tools:
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking/installing required Go tools...$(NC)\n"
	@printf $(LABEL_FMT) "golangci-lint:"
	@if ! command -v golangci-lint >/dev/null 2>&1 || ([ "$(GOLANGCILINT_VERSION)" != "latest" ] && ! golangci-lint --version | grep -qF "$(GOLANGCILINT_VERSION)"); then \
		printf "$(ICON_INFO) $(YELLOW)Installing/Updating golangci-lint@$(GOLANGCILINT_VERSION)...$(NC)\n" ;\
		GOBIN=$(shell go env GOPATH)/bin go install github.com/golangci/golangci-lint/cmd/golangci-lint@$(GOLANGCILINT_VERSION) && \
		printf "    $(ICON_OK) $(GREEN)golangci-lint installed/updated: $$(golangci-lint --version)$(NC)\n" || \
		(printf "    $(ICON_FAIL) $(RED)golangci-lint installation/update failed$(NC)\n" && exit 1); \
	else \
		printf "$(ICON_OK) $(GREEN)golangci-lint already installed: $$(golangci-lint --version)$(NC)\n"; \
	fi
	@printf $(LABEL_FMT) "gotestsum:"
	@if ! command -v gotestsum >/dev/null 2>&1 || ([ "$(GOTESTSUM_VERSION)" != "latest" ] && ! gotestsum --version | grep -qF "$(GOTESTSUM_VERSION)"); then \
		printf "$(ICON_INFO) $(YELLOW)Installing/Updating gotestsum@$(GOTESTSUM_VERSION)...$(NC)\n" ;\
		GOBIN=$(shell go env GOPATH)/bin go install gotest.tools/gotestsum@$(GOTESTSUM_VERSION) && \
		printf "    $(ICON_OK) $(GREEN)gotestsum installed/updated: $$(gotestsum --version)$(NC)\n" || \
		(printf "    $(ICON_FAIL) $(RED)gotestsum installation/update failed$(NC)\n" && exit 1); \
	else \
		printf "$(ICON_OK) $(GREEN)gotestsum already installed: $$(gotestsum --version)$(NC)\n"; \
	fi
	@printf "  $(ICON_OK) $(GREEN)Go tools check/installation complete$(NC)\n"
	@printf "\n"

# Checks for required local tools and environment setup.
check: install-tools
	@printf "$(ICON_START) $(BOLD)$(BLUE)Checking for required tools and environment...$(NC)\n"
	@printf $(LABEL_FMT) "Go:"
	@if command -v go >/dev/null 2>&1; then \
		printf "$(ICON_OK) $(GREEN)Installed$(NC) ($(shell go version))\n"; \
	else \
		printf "$(ICON_FAIL) $(RED)Not installed$(NC)\n"; exit 1; \
	fi
	@printf $(LABEL_FMT) "tree:"
	@if command -v tree >/dev/null 2>&1; then \
		printf "$(ICON_OK) $(GREEN)Installed$(NC)\n"; \
	else \
		printf "$(ICON_WARN) $(YELLOW)Not installed$(NC) (needed for 'tree' target)\n"; \
	fi
	@printf $(LABEL_FMT) "yamllint:"
	@if command -v yamllint >/dev/null 2>&1; then \
		printf "$(ICON_OK) $(GREEN)Installed$(NC) ($$(yamllint --version))\n"; \
	else \
		printf "$(ICON_WARN) $(YELLOW)Not installed$(NC) (needed for 'lint-yaml', will attempt install)\n"; \
	fi
	@if [ -x "$(SCRIPT_DIR)/check_go_bin_path.sh" ]; then \
		printf $(LABEL_FMT) "Go Bin Path Script:"; \
		"$(SCRIPT_DIR)/check_go_bin_path.sh" && \
		printf "$(ICON_OK) $(GREEN)check_go_bin_path.sh passed$(NC)\n" || \
		printf "$(ICON_WARN) $(YELLOW)check_go_bin_path.sh reported issues or failed$(NC)\n"; \
	else \
		printf $(LABEL_FMT) "Go Bin Path Script:"; \
		printf "$(ICON_WARN) $(YELLOW)Not found or not executable: $(SCRIPT_DIR)/check_go_bin_path.sh$(NC)\n"; \
	fi
	@printf "  $(ICON_OK) $(GREEN)Tool and environment check complete$(NC)\n"
	@printf "\n"

# --- Deployment ---
# Deploys the application to Google Cloud via Cloud Build.
deploy: all check-vulns lint-yaml
	@printf "$(ICON_START) $(BOLD)$(BLUE)Deploying $(SERVICE_NAME) to Google Cloud...$(NC)\n"
	@if [ -z "$(PROJECT_ID)" ]; then \
		printf "  $(ICON_FAIL) $(RED)Error: Google Cloud Project ID not found. Set via 'gcloud config set project YOUR_PROJECT_ID' or ensure gcloud is configured.$(NC)\n"; \
		exit 1; \
	fi
	@printf "  $(ICON_INFO) Project: $(PROJECT_ID)\n"
	@CONFIG_PATH=""; \
	if [ -f "build/cloudbuild/cloudbuild.yaml" ]; then \
		CONFIG_PATH="./build/cloudbuild/cloudbuild.yaml"; \
	elif [ -f "./cloudbuild.yaml" ]; then \
		CONFIG_PATH="./cloudbuild.yaml"; \
	else \
		printf "  $(ICON_FAIL) $(RED)Error: Cloud Build config file not found at './build/cloudbuild/cloudbuild.yaml' or './cloudbuild.yaml'.$(NC)\n"; \
		exit 1; \
	fi; \
	printf "  $(ICON_INFO) Using Cloud Build config: '%s'\n" "$$CONFIG_PATH"; \
	CLEAN_SUBS="_SERVICE_NAME=$(SERVICE_NAME),_REGION=$(GCP_REGION),_ARTIFACT_REGISTRY_REPO=$(ARTIFACT_REGISTRY_REPO),_MODULE_PATH=$(MODULE_PATH),_MAKEFILE_VERSION=$(LOCAL_VERSION),_MAKEFILE_COMMIT=$(LOCAL_COMMIT_HASH)"; \
	printf "  $(ICON_INFO) DEBUG: Substitutions string before gcloud:\n>>>$$CLEAN_SUBS<<<\n"; \
	printf "  $(ICON_INFO) Using Substitutions: $$CLEAN_SUBS\n"; \
	printf "  $(ICON_INFO) $(YELLOW)Submitting to Google Cloud Build and awaiting completion...$(NC)\n"; \
	printf "  $(ICON_INFO) $(YELLOW)Begin gcloud output:----------------------------------------------$(NC)\n"; \
	if gcloud builds submit . --config="$$CONFIG_PATH" --project=$(PROJECT_ID) --substitutions="$$CLEAN_SUBS"; then \
		printf "  $(ICON_INFO) $(YELLOW)End gcloud output:------------------------------------------------$(NC)\n"; \
		printf "  $(ICON_OK) $(GREEN)Cloud Build completed successfully.$(NC)\n"; \
		printf "  $(ICON_INFO) Monitor detailed build logs at the URL provided in the gcloud output or here: https://console.cloud.google.com/cloud-build/builds?project=$(PROJECT_ID)\n"; \
		printf "  $(ICON_START) $(BLUE)Fetching deployed service URL...$(NC)\n"; \
		SERVICE_URL=$$(gcloud run services describe $(SERVICE_NAME) --platform=managed --region=$(GCP_REGION) --project=$(PROJECT_ID) --format="value(status.url)" 2>/dev/null); \
		if [ -n "$$SERVICE_URL" ]; then \
			printf "  $(ICON_OK) $(GREEN)Service URL: $$SERVICE_URL$(NC)\n"; \
			make health-check HEALTH_CHECK_URL="$$SERVICE_URL/health" EXPECTED_VERSION="$(LOCAL_VERSION)" EXPECTED_COMMIT="$(LOCAL_COMMIT_HASH)"; \
		else \
			printf "  $(ICON_WARN) $(YELLOW)Could not retrieve service URL. Skipping health check. Please check the Cloud Run console.$(NC)\n"; \
		fi; \
	else \
		printf "  $(ICON_INFO) $(YELLOW)End gcloud output:------------------------------------------------$(NC)\n"; \
		printf "  $(ICON_FAIL) $(RED)Cloud Build submission or execution failed. Review gcloud output above and build logs for details.$(NC)\n"; \
		exit 1; \
	fi
	@printf "\n"
	
health-check:
ifndef HEALTH_CHECK_URL
	$(error HEALTH_CHECK_URL is not set. Usage: make health-check HEALTH_CHECK_URL=... EXPECTED_VERSION=... EXPECTED_COMMIT=...)
endif
ifndef EXPECTED_VERSION
	$(error EXPECTED_VERSION is not set)
endif
ifndef EXPECTED_COMMIT
	$(error EXPECTED_COMMIT is not set)
endif
	@printf "$(ICON_START) $(BOLD)$(BLUE)Performing health check (Expected Version: $(EXPECTED_VERSION), Commit: $(EXPECTED_COMMIT)) on $(HEALTH_CHECK_URL)...$(NC)\n"
	@SUCCESS=false; \
	for i in $$(seq 1 5); do \
		printf "  $(ICON_INFO) Health check attempt #$$i...\n"; \
		RESPONSE=$$(curl -s -w "\n%{http_code}" "$(HEALTH_CHECK_URL)"); \
		HTTP_CODE=$$(echo "$$RESPONSE" | tail -n1); \
		BODY=$$(echo "$$RESPONSE" | sed '$$d'); \
		if [ "$$HTTP_CODE" -eq 200 ]; then \
			RESPONSE_VERSION=$$(echo "$$BODY" | jq -r .version); \
			RESPONSE_COMMIT=$$(echo "$$BODY" | jq -r .commit); \
			if [ "$$RESPONSE_VERSION" = "$(EXPECTED_VERSION)" ] && [ "$$RESPONSE_COMMIT" = "$(EXPECTED_COMMIT)" ]; then \
				printf "  $(ICON_OK) $(GREEN)Health check PASSED. Version and Commit match expected values.$(NC)\n"; \
				printf "    Response: %s\n" "$$BODY"; \
				SUCCESS=true; break; \
			else \
				printf "  $(ICON_WARN) $(YELLOW)Health check attempt #$$i: Service responded OK, but content mismatch.$(NC)\n"; \
				printf "    Expected Version: $(EXPECTED_VERSION), Got: $$RESPONSE_VERSION\n"; \
				printf "    Expected Commit: $(EXPECTED_COMMIT), Got: $$RESPONSE_COMMIT\n"; \
				printf "    Full Response: %s\n" "$$BODY"; \
			fi \
		else \
			printf "  $(ICON_WARN) $(YELLOW)Health check attempt #$$i: Failed with HTTP status $$HTTP_CODE.$(NC)\n"; \
			printf "    Response Body: %s\n" "$$BODY"; \
		fi; \
		if [ $$i -lt 5 ]; then printf "  $(ICON_INFO) Retrying in 10 seconds... $(NC)\n"; sleep 10; fi; \
	done; \
	if [ "$$SUCCESS" = "false" ]; then \
		printf "  $(ICON_FAIL) $(RED)$(BOLD)Post-deployment health check FAILED after 5 attempts.$(NC)\n"; \
		exit 1; \
	fi
	@printf "\n"

# --- Help ---
# Displays this help message.
help:
	@printf "$(BLUE)$(BOLD)$(SERVICE_NAME) Make Targets:$(NC)\n"
	@printf "  %-25s %s\n" "all" "Run general checks, format, tests, and build (default, no vuln scan)"
	@printf "  %-25s %s\n" "build" "Build the application binary ($(BINARY_NAME)) for Linux"
	@printf "  %-25s %s\n" "clean" "Clean build artifacts and caches"
	@printf "  %-25s %s\n" "deps" "Tidy and download Go module dependencies"
	@printf "  %-25s %s\n" "install-tools" "Install/update required Go tools (golangci-lint, gotestsum)"
	@printf "  %-25s %s\n" "check" "Check if required tools (Go, tree, yamllint, etc.) are installed"
	@printf "  %-25s %s\n" "check-gomod" "Check if go.mod module path is correct"
	@printf "\n$(YELLOW)Code Quality & Testing:$(NC)\n"
	@printf "  %-25s %s\n" "fmt" "Format code using golangci-lint fmt"
	@printf "  %-25s %s\n" "lint" "Run basic 'go vet' checks (quick pre-check)"
	@printf "  %-25s %s\n" "golangci-lint" "Run comprehensive linters with golangci-lint"
	@printf "  %-25s %s\n" "lint-yaml" "Lint YAML files (*.yaml, *.yml) using yamllint" # Added lint-yaml description
	@printf "  %-25s %s\n" "test" "Run tests using gotestsum"
	@printf "  %-25s %s\n" "test-debug" "Run tests with verbose output (go test -v)"
	@printf "  %-25s %s\n" "check-line-length" "Check Go file line count (W:$(WARN_LINES), F:$(FAIL_LINES))"
	@printf "\n$(YELLOW)Security:$(NC)\n"
	@printf "  %-25s %s\n" "check-vulns" "Scan for known vulnerabilities in dependencies (fails build if found)"
	@printf "\n$(YELLOW)Other:$(NC)\n"
	@printf "  %-25s %s\n" "tree" "Generate project directory tree view in ./docs/"
	@printf "  %-25s %s\n" "deploy" "Run all checks (incl. vuln scan, yaml lint), build, then deploy, show URL & health check" # Updated deploy description
	@printf "  %-25s %s\n" "help" "Display this help message"

##
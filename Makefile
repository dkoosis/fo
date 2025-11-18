# Makefile for fo utility
SHELL := /bin/bash
.SHELLFLAGS := -e -o pipefail -c

.PHONY: all build test clean check lint fmt tree help ensure-fo

# --- Variables ---
SERVICE_NAME := fo
BINARY_NAME  := $(SERVICE_NAME)
MODULE_PATH  := github.com/davidkoosis/fo
CMD_PATH     := ./cmd
SCRIPT_DIR   := ./scripts
FO_PATH      := ./bin
FO           := $(FO_PATH)/$(BINARY_NAME)

# CI settings
FO_FLAGS :=
FO_PRINT_FLAGS :=
ifeq ($(CI),true)
    FO_FLAGS += --ci
    FO_PRINT_FLAGS += --ci
endif

# Build variables
LOCAL_VERSION := $(shell git describe --tags --always --dirty --match=v* 2>/dev/null || echo "dev")
LOCAL_COMMIT_HASH := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
LDFLAGS := -ldflags "-s -w \
            -X '$(MODULE_PATH)/cmd/internal/version.Version=$(LOCAL_VERSION)' \
            -X '$(MODULE_PATH)/cmd/internal/version.CommitHash=$(LOCAL_COMMIT_HASH)' \
            -X '$(MODULE_PATH)/cmd/internal/version.BuildDate=$(BUILD_DATE)'"

# File lists
WARN_LINES := 350
FAIL_LINES := 1500
GO_FILES := $(shell find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" -not -path "./bin/*")
YAML_FILES := $(shell find . -type f \( -name "*.yaml" -o -name "*.yml" \) -not -path "./vendor/*" -not -path "./.git/*")

# --- Helper Macros ---
# NO leading @ here
define FO_RUN
    $(FO) $(FO_FLAGS) -l "$(1)" $(3) -- $(2)
endef

# NO leading @ here
define FO_PRINT
    $(FO) $(FO_PRINT_FLAGS) print --type "$(1)" --icon "$(2)" --indent $(3) -- "$(4)"
endef

# --- Core Targets ---
all: ensure-fo check fmt lint test build tree
	@$(call FO_PRINT,success,✨,0,All development cycle tasks completed successfully!)

ensure-fo:
	@mkdir -p $(FO_PATH)
	@if [ ! -f "$(FO)" ] || [ -n "$(shell find cmd -name '*.go' -newer $(FO))" ]; then \
	echo "ℹ️ Building $(SERVICE_NAME) utility..."; \
	go build $(LDFLAGS) -o $(FO) $(CMD_PATH); \
	fi

build: ensure-fo
	@$(call FO_PRINT,h1,▶️,0,Build fo application)
	@$(call FO_PRINT,success,✓,1,Build successful: $(realpath $(FO)))

fmt: ensure-fo
	@$(call FO_PRINT,h1,▶️,0,Format and organize code)
	@$(call FO_RUN,Tidy modules,go mod tidy -v)
	@$(call FO_RUN,Download modules,go mod download)
	@$(call FO_RUN,Format Go code,golangci-lint fmt ./...)

lint: ensure-fo
	@$(call FO_PRINT,h1,▶️,0,Lint code)
	@$(call FO_RUN,Run Go linter,golangci-lint run ./...)
	@if command -v yamllint >/dev/null 2>&1 && [ -n "$(YAML_FILES)" ]; then \
	$(call FO_RUN,Lint YAML files,yamllint $(YAML_FILES)); \
	fi
	@$(call FO_RUN,Check file line lengths,$(SCRIPT_DIR)/check_file_length.sh $(WARN_LINES) $(FAIL_LINES) $(GO_FILES))
	@if [ -x "$(SCRIPT_DIR)/check_go_mod_path.sh" ]; then \
	$(call FO_RUN,Check go.mod path,$(SCRIPT_DIR)/check_go_mod_path.sh $(MODULE_PATH)); \
	fi

test: ensure-fo
	@$(call FO_PRINT,h1,▶️,0,Run tests)
	@$(call FO_RUN,Run tests,gotestsum --format short -- ./...,-s)

check: ensure-fo
	@$(call FO_PRINT,h1,▶️,0,Check for required tools)
	@$(call FO_PRINT,success,✅,1,Go: $(shell go version 2>/dev/null || echo "Not installed"))
	@$(call FO_PRINT,success,✅,1,GoLangCI-Lint: $(shell golangci-lint --version 2>/dev/null | head -n1 || echo "Not installed"))
	@$(call FO_PRINT,success,✅,1,GoTestSum: $(shell gotestsum --version 2>/dev/null || echo "Not installed"))
	@$(call FO_PRINT,success,✅,1,Tree: $(shell tree --version 2>/dev/null | head -n1 || echo "Not installed"))
	@$(call FO_PRINT,success,✅,1,YamlLint: $(shell yamllint --version 2>/dev/null || echo "Not installed"))

tree: ensure-fo
	@$(call FO_PRINT,info,🌲,0,Generate project tree)
	@mkdir -p ./docs
	@if command -v tree > /dev/null; then \
	tree -F -I "vendor|.git|.idea*|*.DS_Store|coverage.out|$(FO_PATH)" --dirsfirst -o ./docs/project_folder_tree.txt . && \
	$(call FO_PRINT,success,✅,0,Project tree generated at ./docs/project_folder_tree.txt); \
    else \
        $(call FO_PRINT,warning,⚠️,0,Tree command not found. Install with: brew install tree); \
    fi

clean: ensure-fo
	@$(call FO_PRINT,info,🧹,0,Clean build artifacts)
	@rm -rf $(FO_PATH) coverage.out
	@go clean -cache -testcache
	@$(call FO_PRINT,success,✅,0,Cleanup complete)

help:
	@printf "\033[1m\033[0;34m%-20s %s\033[0m\n" "$(SERVICE_NAME) Makefile" "Development Targets"
	@printf "\033[0;34m-----------------------------------------\033[0m\n"
	@printf "  %-20s %s\n" "all" "Run complete build pipeline (default)"
	@printf "  %-20s %s\n" "build" "Build fo utility"
	@printf "  %-20s %s\n" "fmt" "Format code and manage dependencies"
	@printf "  %-20s %s\n" "lint" "Run linters (Go, YAML, line length)"
	@printf "  %-20s %s\n" "test" "Run tests with gotestsum"
	@printf "  %-20s %s\n" "check" "Check dev tools and environment"
	@printf "  %-20s %s\n" "tree" "Generate project directory tree"
	@printf "  %-20s %s\n" "clean" "Clean build artifacts"
	@printf "  %-20s %s\n" "help" "Show this help message"

# ============================================================================
# Documentation Generation Targets (added by install-docs-hooks.sh)
# ============================================================================
# Documentation generation targets
# Add these to your Makefile for generating docs/tree.txt and docs/issues.json

.PHONY: docs-tree docs-issues docs-all install-hooks

# Generate project documentation tree
docs-tree:
	@mkdir -p docs
	@if ! which tree > /dev/null 2>&1; then \
		echo "⚠️  tree command not found. Install with: brew install tree (macOS)"; \
		echo "Generating fallback tree.txt with find..."; \
		find . -type d -name '.git' -prune -o \
		       -type d -name 'node_modules' -prune -o \
		       -type d -name 'vendor' -prune -o \
		       -type d -name 'bin' -prune -o \
		       -type d -name 'build' -prune -o \
		       -type f -name '.DS_Store' -prune -o \
		       -type f -print | sort > docs/tree.txt; \
	else \
		tree --gitignore -F --dirsfirst -I '.git|.DS_Store|*.log|*.db|coverage.*|bin|tmp|test-output|test-results|build|node_modules|vendor' > docs/tree.txt 2>/dev/null || \
		tree -F --dirsfirst -I '.git|.DS_Store|*.log|*.db|coverage.*|bin|tmp|test-output|test-results|build|node_modules|vendor' > docs/tree.txt; \
	fi
	@wc -l docs/tree.txt | awk '{print "✓ Generated docs/tree.txt with " $$1 " entries"}'

# Fetch and export GitHub issues to JSON
docs-issues:
	@mkdir -p docs
	@if ! which gh > /dev/null 2>&1; then \
		echo "❌ GitHub CLI 'gh' not found. Install with: brew install gh"; \
		exit 1; \
	fi
	@if ! gh auth status > /dev/null 2>&1; then \
		echo "❌ GitHub CLI not authenticated. Run: gh auth login"; \
		exit 1; \
	fi
	@OWNER_REPO=$$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null); \
	if [ -z "$$OWNER_REPO" ]; then \
		echo "❌ Could not determine repository. Make sure you're in a git repo with a remote."; \
		exit 1; \
	fi; \
	echo "📥 Fetching issues for $$OWNER_REPO..."; \
	gh issue list -R "$$OWNER_REPO" --state all --limit 9999 --json \
		number,title,body,state,labels,assignees,milestone,createdAt,updatedAt,closedAt,author,comments \
		> docs/issues.json && \
	ISSUE_COUNT=$$(cat docs/issues.json | grep -o '"number"' | wc -l | tr -d ' ') && \
	echo "✓ Exported $$ISSUE_COUNT issues to docs/issues.json"

# Generate both tree and issues documentation
docs-all: docs-tree docs-issues
	@echo "✅ Documentation generation complete!"

# Install git hooks for automatic documentation generation
install-hooks:
	@echo "Installing git hooks..."
	@if [ -f scripts/pre-commit-template ]; then \
		cp scripts/pre-commit-template .git/hooks/pre-commit; \
		chmod +x .git/hooks/pre-commit; \
		echo "✓ Installed pre-commit hook"; \
	else \
		echo "⚠️  scripts/pre-commit-template not found"; \
		echo "Download from: https://github.com/dkoosis/pha-wix-ccs/scripts/pre-commit-template"; \
	fi

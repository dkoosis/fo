.PHONY: all qa build test lint fmt fmt-fix vet clean lint-sarif \
       cross cross-amd64 cross-arm64 help

all: build test lint ## Full QA pass

qa: all ## Alias for all

build: ## Install binary to $GOBIN
	go install ./cmd/fo

test: ## Run tests with race detector and coverage
	go test -race -cover ./...

lint: vet ## Run all linters (vet + golangci-lint)
	golangci-lint run --output.text.path=stdout ./...

vet: ## Run go vet
	go vet ./...

fmt: ## Check formatting (exits non-zero if unformatted)
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

fmt-fix: ## Fix formatting in-place
	gofmt -w .

lint-sarif: vet ## Run linters with SARIF output (pipe through fo)
	golangci-lint run --output.sarif.path=stdout ./...

clean: ## Remove build artifacts
	rm -f fo
	rm -rf build/

## ---------------------------------------------------------------------
## Sandbox prebuilts
## ---------------------------------------------------------------------

# Versions — keep in sync with what .codex/setup.sh expects
GOLANGCI_LINT_VER ?= v2.11.3
GOFUMPT_VER       ?= v0.9.2
GOIMPORTS_VER     ?= v0.39.0
BAT_VER           ?= v0.25.0
SNIPE_SRC         ?= $(HOME)/Projects/snipe
GOMOD_VER         := $(shell awk '/^go /{print $$2}' go.mod)
VERSION           ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

cross: cross-amd64 ## Cross-compile sandbox tools (default: amd64)

cross-amd64: ## Cross-compile linux/amd64 sandbox tools
	@echo "=== cross: linux/amd64 ==="
	@$(MAKE) --no-print-directory _cross-build CROSS_ARCH=amd64

cross-arm64: ## Cross-compile linux/arm64 sandbox tools
	@echo "=== cross: linux/arm64 ==="
	@$(MAKE) --no-print-directory _cross-build CROSS_ARCH=arm64

_cross-build:
	@# Pre-flight: local Go must be >= go.mod target
	@LOCAL_GO=$$(go version | sed 's/.*go\([0-9]*\.[0-9]*\).*/\1/'); \
	MOD_MIN=$$(echo $(GOMOD_VER) | cut -d. -f1)$$(printf '%03d' $$(echo $(GOMOD_VER) | cut -d. -f2)); \
	LOC_MIN=$$(echo $$LOCAL_GO | cut -d. -f1)$$(printf '%03d' $$(echo $$LOCAL_GO | cut -d. -f2)); \
	if [ "$$LOC_MIN" -lt "$$MOD_MIN" ]; then \
		echo "FATAL: local go$$LOCAL_GO < go.mod go$(GOMOD_VER)"; \
		exit 1; \
	fi; \
	echo "  local go$$LOCAL_GO >= go.mod go$(GOMOD_VER) — ok"
	@mkdir -p .bin/linux-$(CROSS_ARCH)
	$(eval XBIN := $(shell go env GOPATH)/bin/linux_$(CROSS_ARCH))
	@echo "-- golangci-lint $(GOLANGCI_LINT_VER)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(CROSS_ARCH) go install -trimpath -ldflags='-s -w' github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VER)
	@cp $(XBIN)/golangci-lint .bin/linux-$(CROSS_ARCH)/
	@echo "-- gofumpt $(GOFUMPT_VER)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(CROSS_ARCH) go install -trimpath -ldflags='-s -w' mvdan.cc/gofumpt@$(GOFUMPT_VER)
	@cp $(XBIN)/gofumpt .bin/linux-$(CROSS_ARCH)/
	@echo "-- goimports $(GOIMPORTS_VER)"
	@CGO_ENABLED=0 GOOS=linux GOARCH=$(CROSS_ARCH) go install -trimpath -ldflags='-s -w' golang.org/x/tools/cmd/goimports@$(GOIMPORTS_VER)
	@cp $(XBIN)/goimports .bin/linux-$(CROSS_ARCH)/
	@echo "-- snipe"
	@if [ -d "$(SNIPE_SRC)" ]; then \
		echo "  (from $(SNIPE_SRC))"; \
		cd "$(SNIPE_SRC)" && CGO_ENABLED=0 GOOS=linux GOARCH=$(CROSS_ARCH) \
			go build -trimpath -ldflags='-s -w' -o "$(CURDIR)/.bin/linux-$(CROSS_ARCH)/snipe" .; \
	else \
		CGO_ENABLED=0 GOOS=linux GOARCH=$(CROSS_ARCH) go install -trimpath -ldflags='-s -w' github.com/dkoosis/snipe@latest && \
			cp $(XBIN)/snipe .bin/linux-$(CROSS_ARCH)/; \
	fi
	@echo "-- bat $(BAT_VER)"
	@if [ -f ".bin/linux-$(CROSS_ARCH)/bat" ]; then \
		echo "  (exists, skipping download)"; \
	else \
		case "$(CROSS_ARCH)" in \
			amd64) BAT_TRIPLE="x86_64-unknown-linux-musl" ;; \
			arm64) BAT_TRIPLE="aarch64-unknown-linux-gnu" ;; \
		esac; \
		TMP=$$(mktemp -d); \
		echo "  downloading bat-$(BAT_VER)-$$BAT_TRIPLE"; \
		curl -fsSL "https://github.com/sharkdp/bat/releases/download/$(BAT_VER)/bat-$(BAT_VER)-$$BAT_TRIPLE.tar.gz" \
			| tar xz -C "$$TMP" && \
		cp "$$TMP"/bat-*/bat .bin/linux-$(CROSS_ARCH)/bat && \
		rm -rf "$$TMP"; \
	fi
	@echo "-- dtree"
	@cp .codex/dtree .bin/linux-$(CROSS_ARCH)/dtree
	@# UPX compress all ELF binaries (skip shell scripts)
	@if command -v upx >/dev/null 2>&1; then \
		echo "-- upx compressing"; \
		for f in .bin/linux-$(CROSS_ARCH)/*; do \
			case "$$(file -b "$$f")" in *ELF*) \
				upx -q --best "$$f" 2>/dev/null || true; \
			esac; \
		done; \
	else \
		echo "-- upx not found, skipping compression"; \
	fi
	@echo ""
	@echo "=== prebuilts ready ==="
	@ls -lh .bin/linux-$(CROSS_ARCH)/

help: ## Show this help
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

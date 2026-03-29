# fo Makefile
#
# Primary: check ci report deploy doctor cross
# Run `make help` for full target list.

.DEFAULT_GOAL := check

.PHONY: help check ci report deploy install doctor cross \
        vet lint test race fmt fmt-fix dupl vuln \
        changed snipe-index baseline \
        cross-amd64 cross-arm64 \
        lint-sarif clean issues

# ── Sandbox prebuilt versions ──
# Keep in sync with what .codex/setup.sh expects.
GOLANGCI_LINT_VER ?= v2.11.3
GOFUMPT_VER       ?= v0.9.2
GOIMPORTS_VER     ?= v0.39.0
BAT_VER           ?= v0.25.0
SNIPE_SRC         ?= $(HOME)/Projects/snipe
GOMOD_VER         := $(shell awk '/^go /{print $$2}' go.mod)
VERSION           ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Report stream — fo eats its own dog food
REPORT_CMD = \
	echo '--- tool:vet format:sarif ---'; \
	go vet ./... 2>&1 | fo wrap diag --tool govet; echo; \
	echo '--- tool:lint format:sarif ---'; \
	golangci-lint run --output.sarif.path=/dev/stdout --output.text.path=/dev/null ./... 2>/dev/null | head -1; echo; \
	echo '--- tool:test format:testjson ---'; \
	go test -json -count=1 ./... 2>&1; echo; \
	echo '--- tool:dupl format:jscpd ---'; \
	TMP_JSCPD=$$(mktemp -d); jscpd . --silent --reporters json --output $$TMP_JSCPD >/dev/null 2>&1; cat $$TMP_JSCPD/jscpd-report.json; echo; rm -rf $$TMP_JSCPD; \
	echo '--- tool:vuln format:sarif ---'; \
	TMP_VULN=$$(mktemp); govulncheck -format sarif ./... >$$TMP_VULN 2>/dev/null; cat $$TMP_VULN; rm -f $$TMP_VULN; echo

## ---------------------------------------------------------------------
## Primary
## ---------------------------------------------------------------------

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^## [^-]/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 4) } \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: vet lint test ## Fast validation: vet + lint + test + build
	@go build ./...
	@echo "=== check pass ==="

ci: snipe-index check race dupl vuln ## Full CI suite
	@echo "=== ci pass ==="

report: snipe-index ## Structured QA output for agents/tools (always exits 0)
	@( $(REPORT_CMD) ) | fo --format llm || true

deploy: install ## Build and install binary
	@echo "=== deployed ($$(which fo)) ==="

doctor: ## Validate required toolchain
	@echo "=== doctor ==="
	@MISSING=0; \
	for tool in go golangci-lint snipe jq gofumpt goimports govulncheck; do \
		if command -v "$$tool" >/dev/null 2>&1; then \
			printf "  ok  %-20s %s\n" "$$tool" "$$(command -v $$tool)"; \
		else \
			printf "  MISSING  %s\n" "$$tool"; \
			MISSING=$$((MISSING + 1)); \
		fi; \
	done; \
	for tool in fo jscpd dtree; do \
		if command -v "$$tool" >/dev/null 2>&1; then \
			printf "  ok  %-20s %s (optional)\n" "$$tool" "$$(command -v $$tool)"; \
		else \
			printf "  skip  %-20s (optional)\n" "$$tool"; \
		fi; \
	done; \
	echo ""; \
	if command -v snipe >/dev/null 2>&1; then \
		snipe status 2>/dev/null | jq -r '"  snipe: " + (.results[0].state // "unknown")' 2>/dev/null || echo "  snipe: status unavailable"; \
	fi; \
	echo "  go: $$(go version 2>/dev/null | cut -d' ' -f3)"; \
	if [ "$$MISSING" -gt 0 ]; then \
		echo ""; \
		echo "$$MISSING required tool(s) missing — install manually or run .codex/setup.sh"; \
		exit 1; \
	fi; \
	echo "=== doctor pass ==="

cross: cross-amd64 ## Cross-compile sandbox tools (default: amd64)

## ---------------------------------------------------------------------
## Checks
## ---------------------------------------------------------------------

vet: ## Run go vet
	go vet ./...

lint: vet ## Run golangci-lint
	golangci-lint run --output.text.path=stdout ./...

test: ## Run tests with coverage
	go test -count=1 -cover ./...

race: ## Run tests with race detector (slow)
	go test -race -timeout=5m -count=1 -cover ./...

fmt: ## Check formatting (exits non-zero if unformatted)
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

fmt-fix: ## Fix formatting in-place
	gofmt -w .

dupl: ## Check for code duplication (jscpd)
	jscpd .

vuln: ## Scan for known vulnerabilities
	govulncheck ./...

lint-sarif: vet ## Run linters with SARIF output
	golangci-lint run --output.sarif.path=stdout ./...

## ---------------------------------------------------------------------
## Advanced / Diagnostics
## ---------------------------------------------------------------------

changed: ## Vet + lint + test changed packages only
	@PKGS=$$( { git diff --name-only HEAD -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } \
		| xargs dirname 2>/dev/null | sort -u | sed 's|^|./|' | grep -v '^\./$$'); \
	if [ -z "$$PKGS" ]; then \
		echo "no changed Go packages"; \
	else \
		echo "changed packages: $$PKGS"; \
		go vet $$PKGS && \
		golangci-lint run $$PKGS && \
		go test -count=1 -cover $$PKGS && \
		echo "=== changed pass ==="; \
	fi

baseline: snipe-index ## Save QA report as baseline for sandbox diff
	@( $(REPORT_CMD) ) > .codex/baseline.txt 2>&1
	@echo "=== baseline saved to .codex/baseline.txt ($$(wc -l < .codex/baseline.txt) lines) ==="

snipe-index: ## Rebuild snipe index if stale
	@state=$$(snipe status 2>/dev/null | jq -r '.results[0].state // "unknown"'); \
	if [ "$$state" != "fresh" ]; then \
		echo "snipe index stale ($$state), rebuilding..."; \
		snipe index --embed-mode=off --enrich=false; \
	else \
		echo "snipe index fresh"; \
	fi

issues: ## Fetch open GitHub issues to docs/issues.json
	@mkdir -p docs
	@OWNER_REPO=$$(gh repo view --json nameWithOwner -q .nameWithOwner 2>/dev/null); \
	if [ -z "$$OWNER_REPO" ]; then echo "Could not determine repository."; exit 1; fi; \
	echo "Fetching issues for $$OWNER_REPO..."; \
	gh issue list -R "$$OWNER_REPO" --state open --limit 9999 --json \
		number,title,body,state,labels,assignees,milestone,createdAt,updatedAt,closedAt,author,comments \
		> docs/issues.json && \
	COUNT=$$(jq length docs/issues.json) && \
	echo "Exported $$COUNT issues to docs/issues.json"

clean: ## Remove build artifacts
	rm -f fo
	rm -rf build/ .bin/

install:
	go install ./cmd/fo/

## ---------------------------------------------------------------------
## Cross-compilation
## ---------------------------------------------------------------------

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

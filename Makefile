# fo Makefile
#
# Primary: scan check audit deploy doctor cross
#   scan          — changed pkgs only (fast inner loop)
#   check         — full repo: vet + lint + test + build
#   audit         — check + race + fuzz + dupe + nilcheck + deadcode + omitempty + vuln
#   audit-human   — stream through fo, forced human format (auto: human@TTY, llm piped)
#   audit-llm     — stream through fo, forced llm format (for agents/CI)
# Run `make help` for full target list.

.DEFAULT_GOAL := check

.PHONY: help scan check audit audit-human audit-llm report deploy install build doctor \
        vet lint test race fuzz fmt fmt-fix dupe nilcheck deadcode omitempty vuln standard \
        qa-goldens qa-goldens-update \
        snipe-index baseline \
        lint-sarif clean issues \
        demo demo-llm demo-live

# ── Sandbox prebuilt versions ──
# Keep in sync with what .sandbox/codex/setup.sh expects.
# golangci-lint: the ONE fleet pin lives in .sandbox/project.conf
# (repo-standard lint-pin rule) — read it, never write a second literal here.
GOLANGCI_LINT_VER ?= $(shell . ./.sandbox/project.conf 2>/dev/null && echo $$GOLANGCI_LINT_VERSION)
GOFUMPT_VER       ?= v0.9.2
GOIMPORTS_VER     ?= v0.39.0
BAT_VER           ?= v0.25.0
STANDARD_VER      := v0.4.0
OMITEMPTYSCALAR_VER := v0.4.0
GOVULNCHECK_VER   := v1.9.0
DEADCODE_VER      := v0.52.0
NILAWAY_VER       := v0.0.0-20260918162853-acb8859b9031
JSCPD_VER         := 5.1.2
FUZZTIME          ?= 10s
SNIPE_SRC         ?= $(HOME)/Projects/snipe
VERSION           ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# cross, cross-amd64, cross-arm64: the fleet's vendored recipes (conform owns the file).
# They build every tool with go build -o, so a set GOBIN cannot refuse them.
# The pins above come first, so the include's ?= defaults yield to them.
# What gets built comes from PROJECT_BINS and PREBUILT_TOOLS in .sandbox/project.conf.
include .sandbox/lib/Makefile.cross.mk

# Report stream — fo eats its own dog food.
# Race tests subsume plain tests (-race -json is a superset).
# Build errors go through wrap diag so they join the SARIF stream.
REPORT_CMD = \
	echo '--- tool:vet format:sarif ---'; \
	go vet ./... 2>&1 | fo wrap diag --tool govet; echo; \
	echo '--- tool:arch format:sarif ---'; \
	go-arch-lint check --json 2>/dev/null | fo wrap archlint; echo; \
	echo '--- tool:lint format:sarif ---'; \
	golangci-lint run --output.sarif.path=/dev/stdout --output.text.path=/dev/null ./... 2>/dev/null | head -1; echo; \
	echo '--- tool:build format:sarif ---'; \
	go build ./... 2>&1 | fo wrap diag --tool gobuild; echo; \
	echo '--- tool:test format:testjson ---'; \
	go test -race -json -timeout=5m -count=1 ./... 2>&1; echo; \
	echo '--- tool:dupl format:sarif ---'; \
	TMP_JSCPD=$$(mktemp -d); jscpd . --silent --reporters json --output $$TMP_JSCPD >/dev/null 2>&1; cat $$TMP_JSCPD/jscpd-report.json | fo wrap jscpd; echo; rm -rf $$TMP_JSCPD; \
	echo '--- tool:vuln format:sarif ---'; \
	TMP_VULN=$$(mktemp); govulncheck -format sarif ./... >$$TMP_VULN 2>/dev/null; cat $$TMP_VULN; rm -f $$TMP_VULN; echo

## ---------------------------------------------------------------------
## Primary
## ---------------------------------------------------------------------

help: ## Show this help
	@awk 'BEGIN {FS = ":.*##"; printf "\nUsage:\n  make \033[36m<target>\033[0m\n"} \
		/^## [^-]/ { printf "\n\033[1m%s\033[0m\n", substr($$0, 4) } \
		/^[a-zA-Z0-9_-]+:.*?## / { printf "  \033[36m%-18s\033[0m %s\n", $$1, $$2 }' $(MAKEFILE_LIST)

check: vet lint test build standard ## Full repo: vet + lint + test + build + repo-standard
	@echo "=== check pass ==="

# Dogfood the fleet gate: repo-standard runs at a pinned version (go run <pkg>@VER).
standard: ## Run repo-standard (fleet SDLC checker) against this repo
	go run github.com/dkoosis/repo-standard/cmd/repo-standard@$(STANDARD_VER)

audit: check race fuzz dupe nilcheck deadcode omitempty vuln ## Exhaustive gate: check + race + fuzz + dupe + nilcheck + deadcode + omitempty + vuln
	@echo "=== audit pass ==="

audit-human: snipe-index ## Force human-formatted audit
	@( $(REPORT_CMD) ) | fo --format human || true

audit-llm: snipe-index ## Force llm-formatted audit (agents/CI)
	@( $(REPORT_CMD) ) | fo --format llm || true

report: audit-llm ## Alias for audit-llm (back-compat)

deploy: install ## Build and install binary
	@echo "deployed /usr/local/bin/fo ($$(/usr/local/bin/fo 2>&1 | head -1))"

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
		echo "$$MISSING required tool(s) missing — install manually or run .sandbox/codex/setup.sh"; \
		exit 1; \
	fi; \
	echo "=== doctor pass ==="

## ---------------------------------------------------------------------
## Git hooks
## ---------------------------------------------------------------------

hooks: ## Route git hooks to the tracked .githooks/ dir (bd + repo gates, ccp-th5.2). Local-only, per-clone; run once after cloning.
	@missing=""; \
	for h in pre-commit post-merge pre-push post-checkout prepare-commit-msg; do \
		if [ ! -x ".githooks/$$h" ]; then missing="$$missing $$h"; fi; \
	done; \
	if [ -n "$$missing" ]; then \
		echo "make hooks: missing or non-executable dispatcher(s):$$missing" >&2; \
		exit 1; \
	fi
	git config core.hooksPath .githooks
	@echo "git hooks enabled (.githooks): pre-commit (bd + issue-sync + doc-governance), pre-push/post-merge/post-checkout/prepare-commit-msg (bd)."
	@bd hooks list 2>/dev/null || true

## ---------------------------------------------------------------------
## Checks
## ---------------------------------------------------------------------

vet: ## Run go vet
	go vet ./...

lint: vet ## Run golangci-lint
	golangci-lint run --output.text.path=stdout ./...

test: ## Run tests with coverage
	go test -count=1 -cover ./...

qa-goldens: ## Replay captured pipeline fixtures and diff against goldens
	@go test ./pkg/view/ -run TestPipelineGoldens -v

qa-goldens-update: ## Refresh pipeline goldens (review the diff before committing)
	@go test ./pkg/view/ -run TestPipelineGoldens -update -v

race: ## Run tests with race detector (slow)
	go test -race -timeout=5m -count=1 -cover ./...

fmt: ## Check formatting (exits non-zero if unformatted)
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)

fmt-fix: ## Fix formatting in-place
	gofmt -w .

dupe: ## Check for code duplication (jscpd, pinned)
	npx --yes jscpd@$(JSCPD_VER) . --silent --reporters console --ignore '**/.git/**,**/.sandbox/**,**/.beads/**,**/.claude/**,**/.orca/**'

nilcheck: ## Run nilaway (pinned)
	go run go.uber.org/nilaway/cmd/nilaway@$(NILAWAY_VER) -test=false -include-pkgs=github.com/dkoosis/fo ./...

# Runs each Fuzz function it finds, FUZZTIME apiece.
fuzz: ## Fuzz every Fuzz test (FUZZTIME=10s each)
	@set -e; \
	for pkg in $$(go list ./...); do \
		list=$$(go test -list '^Fuzz' $$pkg) || exit $$?; \
		for f in $$(printf '%s\n' "$$list" | sed -n '/^Fuzz/p'); do \
			echo "fuzz $$pkg $$f"; \
			go test -run '^$$' -fuzz "^$$f$$" -fuzztime=$(FUZZTIME) $$pkg; \
		done; \
	done

# deadcode lists each function no path from main reaches, and exits 0 either way,
# so the target fails on any output.
deadcode: ## Fail on any function unreachable from main (deadcode, pinned)
	@out=$$(go run golang.org/x/tools/cmd/deadcode@$(DEADCODE_VER) ./cmd/fo) || { echo "deadcode: could not run"; exit 1; }; \
	if [ -n "$$out" ]; then echo "$$out"; echo "deadcode: the functions above are unreachable; cut them"; exit 1; fi

# omitemptyscalar exits 3 on a ,omitempty tag on a bool (a false lost on write).
omitempty: ## Fail on a bool field tagged omitempty (omitemptyscalar, pinned)
	go run github.com/dkoosis/repo-standard/cmd/omitemptyscalar@$(OMITEMPTYSCALAR_VER) ./...

vuln: ## Scan for known vulnerabilities (govulncheck, pinned)
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VER) ./...

lint-sarif: vet ## Run linters with SARIF output
	golangci-lint run --output.sarif.path=stdout ./...

## ---------------------------------------------------------------------
## Advanced / Diagnostics
## ---------------------------------------------------------------------

scan: ## Vet + lint + test changed packages only (fast inner loop)
	@PKGS=$$( { git diff --name-only HEAD -- '*.go'; git ls-files --others --exclude-standard -- '*.go'; } \
		| xargs dirname 2>/dev/null | sort -u | sed 's|^|./|' | grep -v '^\./$$'); \
	if [ -z "$$PKGS" ]; then \
		echo "no changed Go packages"; \
	else \
		echo "changed packages: $$PKGS"; \
		go vet $$PKGS && \
		golangci-lint run $$PKGS && \
		go test -count=1 -cover $$PKGS && \
		echo "=== scan pass ==="; \
	fi

baseline: snipe-index ## Save QA report as baseline for sandbox diff
	@( $(REPORT_CMD) ) > .sandbox/baseline.txt 2>&1
	@echo "=== baseline saved to .sandbox/baseline.txt ($$(wc -l < .sandbox/baseline.txt) lines) ==="

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

DEMO_DIR := cmd/fo/testdata/demo

demo: install ## Run all demo fixtures through fo (human mode)
	@for f in $(DEMO_DIR)/*.json; do \
		echo ""; \
		echo "━━━ $$(basename $$f) ━━━"; \
		fo --format human < "$$f" || true; \
	done

demo-llm: install ## Run all demo fixtures through fo (llm mode)
	@for f in $(DEMO_DIR)/*.json; do \
		echo ""; \
		echo "━━━ $$(basename $$f) ━━━"; \
		fo --format llm < "$$f" || true; \
	done

demo-live: install ## Run real go test + vet through fo
	@echo "━━━ go test -json ━━━"
	@go test -json -count=1 ./... 2>&1 | fo --format human || true
	@echo ""
	@echo "━━━ go vet (wrapped) ━━━"
	@go vet ./... 2>&1 | fo wrap diag --tool govet | fo --format human || true

clean: ## Remove build artifacts
	rm -f fo
	rm -rf bin/ build/ .sandbox/bin/

build: ## Compile the fo binary to ./fo (no install)
	go build -ldflags "-X main.version=$(VERSION)" -o fo ./cmd/fo/

install: ## Install the fo binary to /usr/local/bin
	@mkdir -p /usr/local/bin 2>/dev/null || true
	@go build -ldflags "-X main.version=$(VERSION)" -o /usr/local/bin/fo ./cmd/fo/
	@gp="$$(go env GOPATH)"; [ -n "$$gp" ] && rm -f "$$gp/bin/fo" || true
	@echo "installed /usr/local/bin/fo ($$(/usr/local/bin/fo 2>&1 | head -1))"

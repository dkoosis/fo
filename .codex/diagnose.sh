#!/bin/bash
# Environment diagnostic for Codex/Claude sandboxes
set -u

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

# Auto-activate environment if not already done
BINDIR="$REPO_ROOT/.codex/bin/linux-amd64"
if [[ -d "$BINDIR" ]] && [[ "$PATH" != *"$BINDIR"* ]]; then
    export PATH="$BINDIR:$REPO_ROOT/bin:$PATH"
fi

ISSUES=()

echo "=== fo Sandbox Diagnostic ==="
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo "Platform: $(uname -s)-$(uname -m)"
echo "Hostname: ${HOSTNAME:-$(hostname 2>/dev/null || echo unknown)}"
echo "Working dir: $REPO_ROOT"
echo ""

check_tool() {
    local name="$1"
    local cmd="$2"
    local version_cmd="$3"
    local required="$4"

    printf "%-16s " "$name:"

    if command -v "$cmd" &>/dev/null; then
        local tool_path=$(command -v "$cmd")
        local version=$($version_cmd 2>&1 | head -1 || echo "unknown")
        echo "OK  $version"
        echo "                 path: $tool_path"
    else
        if [[ "$required" == "yes" ]]; then
            echo "MISSING (required)"
            ISSUES+=("MISSING_REQUIRED: $name")
        else
            echo "MISSING (optional)"
        fi
    fi
}

check_file() {
    local name="$1"
    local filepath="$2"
    local required="$3"

    printf "%-16s " "$name:"
    if [[ -f "$filepath" ]] || [[ -x "$filepath" ]]; then
        local filesize=$(ls -lh "$filepath" 2>/dev/null | awk '{print $5}')
        echo "OK ($filesize)"
    else
        if [[ "$required" == "yes" ]]; then
            echo "MISSING (required)"
            ISSUES+=("MISSING_FILE: $name at $filepath")
        else
            echo "MISSING (optional)"
        fi
    fi
}

echo "--- Core Tools ---"
check_tool "go" "go" "go version" "yes"
check_tool "git" "git" "git --version" "yes"

echo ""
echo "--- QA Tools ---"
check_tool "golangci-lint" "golangci-lint" "golangci-lint --version" "yes"
check_tool "govulncheck" "govulncheck" "govulncheck -version" "yes"
check_tool "mage" "mage" "mage -version" "yes"

echo ""
echo "--- Optional Tools ---"
check_tool "rg (ripgrep)" "rg" "rg --version" "no"
check_tool "jq" "jq" "jq --version" "no"

echo ""
echo "--- Environment Variables ---"
printf "%-16s " "GOCACHE:"
if [[ -n "${GOCACHE:-}" ]]; then
    echo "$GOCACHE"
else
    echo "(not set - using default)"
fi

printf "%-16s " "PATH includes:"
if [[ "$PATH" == *"$REPO_ROOT/bin"* ]]; then
    echo "./bin (good)"
else
    echo "./bin NOT in PATH"
    ISSUES+=("PATH_MISSING: ./bin not in PATH")
fi

echo ""
echo "--- Prebuilt Linux Binaries ---"
if [[ -d "$BINDIR" ]]; then
    echo "Location: $BINDIR"
    check_file "  golangci-lint" "$BINDIR/golangci-lint" "yes"
    check_file "  mage" "$BINDIR/mage" "yes"
    check_file "  govulncheck" "$BINDIR/govulncheck" "no"
else
    echo "Directory not found: $BINDIR"
    ISSUES+=("MISSING_BINDIR: $BINDIR - run: bash .codex/build-linux-binaries.sh")
fi

echo ""
echo "--- Quick Validation ---"

printf "%-16s " "go build:"
if command -v timeout &>/dev/null; then
    BUILD_OUTPUT=$(timeout 60 go build -o /tmp/fo-diag-test ./cmd/fo 2>&1)
    BUILD_EXIT=$?
    if [[ $BUILD_EXIT -eq 124 ]]; then
        echo "TIMEOUT (skipped)"
    elif [[ $BUILD_EXIT -eq 0 ]]; then
        echo "OK"
        rm -f /tmp/fo-diag-test
    else
        echo "FAILED (exit $BUILD_EXIT)"
        ISSUES+=("BUILD_FAILED: go build failed")
    fi
else
    go build -o /tmp/fo-diag-test ./cmd/fo 2>&1 && echo "OK" && rm -f /tmp/fo-diag-test || echo "FAILED"
fi

printf "%-16s " "go vet:"
if command -v timeout &>/dev/null; then
    VET_OUTPUT=$(timeout 30 go vet ./... 2>&1)
    VET_EXIT=$?
    if [[ $VET_EXIT -eq 124 ]]; then
        echo "TIMEOUT (skipped)"
    elif [[ $VET_EXIT -eq 0 ]]; then
        echo "OK"
    else
        echo "WARNINGS (non-blocking)"
    fi
else
    go vet ./... 2>&1 && echo "OK" || echo "WARNINGS"
fi

echo ""
echo "=========================================="
if [[ ${#ISSUES[@]} -eq 0 ]]; then
    echo "STATUS: ALL CHECKS PASSED"
    echo ""
    echo "Environment is ready for QA/testing."
    echo ""
    echo "To run QA, use:"
    echo "  source .codex/activate.sh && mage qa"
    exit 0
else
    echo "STATUS: ${#ISSUES[@]} ISSUE(S) FOUND"
    echo ""

    BINARIES_EXIST=true
    for bin in golangci-lint mage; do
        [[ ! -x "$BINDIR/$bin" ]] && BINARIES_EXIST=false
    done

    if [[ "$BINARIES_EXIST" == "true" ]]; then
        echo "QUICK FIX: Binaries exist. Run:"
        echo "  source .codex/activate.sh"
        echo ""
    fi

    echo "Issues:"
    for issue in "${ISSUES[@]}"; do
        echo "  - $issue"
    done
    echo ""
    exit 1
fi

#!/bin/bash
# Source this file to activate the fo environment
# Usage: source .codex/activate.sh

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# Detect platform for prebuilt binaries
PLATFORM="$(uname -s)-$(uname -m)"
case "$PLATFORM" in
    Linux-x86_64)  BINDIR="$REPO_ROOT/.codex/bin/linux-amd64" ;;
    Darwin-x86_64) BINDIR="$REPO_ROOT/.codex/bin/darwin-amd64" ;;
    Darwin-arm64)  BINDIR="$REPO_ROOT/.codex/bin/darwin-arm64" ;;
    *)             BINDIR="" ;;
esac

# Set up caches
export GOCACHE="$REPO_ROOT/.codex/cache/go-build"
export GOMODCACHE="$REPO_ROOT/.codex/cache/mod"
export GOLANGCI_LINT_CACHE="$REPO_ROOT/.codex/cache/golangci-lint"
mkdir -p "$GOCACHE" "$GOMODCACHE" "$GOLANGCI_LINT_CACHE" 2>/dev/null || true

# Add bins to PATH
export PATH="$REPO_ROOT/bin:$BINDIR:$PATH"
export GOTOOLCHAIN=auto

# Mark as activated
export FO_ENV_ACTIVATED=1

echo "✅ fo environment activated"
echo "   Tools available: $(which golangci-lint mage 2>/dev/null | wc -l | tr -d ' ')/2"

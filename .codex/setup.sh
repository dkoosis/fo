#!/usr/bin/env bash
# Codex cloud environment setup for fo
# Auto-discovered by Codex from .codex/setup.sh on first container creation.
# Cached ~12h; .codex/maintenance.sh refreshes cached containers.
# Exports don't persist into agent phase — use ~/.bashrc or install to PATH.
set -euo pipefail

# Derive repo root from this script's location (.codex/setup.sh → parent)
REPO_DIR="$(cd "$(dirname "$0")/.." && pwd)"
ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *)
    echo "Unsupported architecture: $ARCH" >&2
    exit 1
    ;;
esac
PREBUILT_DIR="$REPO_DIR/.bin/linux-$ARCH"
INSTALL_DIR="/usr/local/bin"

# shellcheck source=lib-doctor.sh
source "$(dirname "$0")/lib-doctor.sh"

echo "=== fo sandbox setup ==="

# --- 1. System aliases ---
# Ubuntu fd-find installs as fdfind; dtree and scripts expect fd
if have fdfind && ! have fd; then
  ln -sf "$(command -v fdfind)" "$INSTALL_DIR/fd"
  echo "  aliased fdfind -> fd"
fi

# --- 2. Prebuilt binaries (seconds, not minutes) ---
# All linux static binaries ship in the repo.
if [ -d "$PREBUILT_DIR" ]; then
  for tool in "$PREBUILT_DIR"/*; do
    [ -f "$tool" ] || continue
    toolname=$(basename "$tool")
    cp "$tool" "$INSTALL_DIR/$toolname"
    chmod +x "$INSTALL_DIR/$toolname"
    echo "  installed $toolname (prebuilt)"
  done
fi

# --- 3. Node tools ---
if have npm; then
  npm install -g jscpd@4 --silent >/dev/null 2>&1 && echo "  installed jscpd" || warn "optional tool jscpd failed to install"
fi

# --- 4. Snipe index (warm cache for code navigation) ---
if have snipe; then
  cd "$REPO_DIR"
  snipe index --embed-mode=off --enrich=false 2>/dev/null && echo "  snipe index built" || echo "  snipe index skipped"
fi

# --- 5. Go module cache (warm) ---
if ! download_go_modules; then
  fatal "go mod download failed"
fi

# --- 6. Diagnose environment ---
echo ""
echo "=== environment check ==="

# 6a. Go version compatibility (sets ACTUAL_GO_VER for 6c)
check_go_version

# 6b. golangci-lint Go version compatibility + auto-repair
if have golangci-lint; then
  LINT_GO_VER=$(golangci_lint_go_version)
  if [ -n "$LINT_GO_VER" ] && [ -n "$ACTUAL_GO_VER" ] && [ "$LINT_GO_VER" != "$ACTUAL_GO_VER" ]; then
    LINT_NUM=$(version_to_int "$LINT_GO_VER")
    ACTUAL_NUM=$(version_to_int "$ACTUAL_GO_VER")
    if [ "$LINT_NUM" -lt "$ACTUAL_NUM" ]; then
      echo "  repairing golangci-lint (built with go$LINT_GO_VER; sandbox has go$ACTUAL_GO_VER)"
      if install_golangci_lint >/dev/null; then
        repaired "golangci-lint built with go$LINT_GO_VER" "rebuilt for go$ACTUAL_GO_VER" "true"
      else
        fatal "golangci-lint built with go$LINT_GO_VER, rebuild failed"
      fi
    fi
  fi
fi

# 6c. Auto-repair: restore missing prebuilt tools (before verification)
restore_prebuilt_tools

# 6d. Required tools (after repair attempts)
for tool in "${REQUIRED_TOOLS[@]}"; do
  if have "$tool"; then
    printf "  ok  %s\n" "$tool"
  else
    printf "  MISSING  %s\n" "$tool"
    fatal "MISSING required tool: $tool"
  fi
done

# Optional tools (from OPTIONAL_TOOLS in lib-doctor.sh)
for tool in "${OPTIONAL_TOOLS[@]}"; do
  if have "$tool"; then
    printf "  ok  %s (optional)\n" "$tool"
  else
    printf "  skip  %s (optional)\n" "$tool"
    warn "optional tool $tool not available"
  fi
done

# --- 7. Write report and exit ---
doctor_exit "setup"

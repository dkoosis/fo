#!/usr/bin/env bash
# Codex cached container refresh for fo
# Runs when a cached container is reused for a new task.
# Keep lightweight — setup.sh already installed tools.
set -euo pipefail

SANDBOX_DIR="$(cd "$(dirname "$0")/.." && pwd)"
REPO_DIR="$(cd "$SANDBOX_DIR/.." && pwd)"
cd "$REPO_DIR"

ARCH=$(uname -m)
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64|arm64) ARCH="arm64" ;;
  *) ARCH="amd64" ;;
esac
PREBUILT_DIR="$SANDBOX_DIR/bin/linux-$ARCH"
INSTALL_DIR="/usr/local/bin"

# shellcheck source=../lib/lib-doctor.sh
source "$SANDBOX_DIR/lib/lib-doctor.sh"

echo "=== fo maintenance ==="

# Refresh go modules
if ! download_go_modules "refreshed"; then
  fatal "go mod download failed"
fi

# Rebuild snipe index
if have snipe; then
  snipe index --embed-mode=off --enrich=false 2>/dev/null && echo "  snipe index rebuilt" || echo "  snipe index skipped"
fi

# Repair: restore missing prebuilt tools
restore_prebuilt_tools

# Go version compatibility
check_go_version

# Verify required tools (after repair) — uses REQUIRED_TOOLS from lib
for tool in "${REQUIRED_TOOLS[@]}"; do
  if ! have "$tool"; then
    printf "  MISSING  %s\n" "$tool"
    fatal "MISSING required tool: $tool"
  fi
done

# Write report + human-readable summary + exit code
doctor_exit "maintenance"

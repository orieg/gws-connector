#!/bin/bash
# Downloads the gws-mcp binary to CLAUDE_PLUGIN_DATA if not present or outdated.
# Called by the SessionStart hook and can be run manually.

set -e

REPO="orieg/gws-connector"
DATA_DIR="${CLAUDE_PLUGIN_DATA:-$HOME/.claude/plugins/data/gws}"
BINARY="$DATA_DIR/gws-mcp"
VERSION_FILE="$DATA_DIR/.version"
PLUGIN_ROOT="${CLAUDE_PLUGIN_ROOT:-$(cd "$(dirname "$0")/.." && pwd)}"

# Read expected version from plugin.json
EXPECTED_VERSION=""
if command -v jq >/dev/null 2>&1 && [ -f "$PLUGIN_ROOT/.claude-plugin/plugin.json" ]; then
  EXPECTED_VERSION=$(jq -r '.version // empty' "$PLUGIN_ROOT/.claude-plugin/plugin.json" 2>/dev/null || true)
fi

# Check if binary exists and version matches
if [ -x "$BINARY" ] && [ -f "$VERSION_FILE" ]; then
  CURRENT_VERSION=$(cat "$VERSION_FILE" 2>/dev/null || true)
  if [ "$CURRENT_VERSION" = "$EXPECTED_VERSION" ] && [ -n "$EXPECTED_VERSION" ]; then
    exit 0
  fi
fi

mkdir -p "$DATA_DIR"

# Detect platform
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "gws-connector: unsupported architecture $ARCH" >&2; exit 1 ;;
esac

ASSET="gws-mcp-${OS}-${ARCH}"
TAG="v${EXPECTED_VERSION}"

# Fall back to latest release if no version known
if [ -z "$EXPECTED_VERSION" ]; then
  TAG="latest"
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
else
  URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
fi

echo "gws-connector: downloading $ASSET ($TAG)..." >&2
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$BINARY" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$BINARY" "$URL"
else
  echo "gws-connector: curl or wget required to download binary" >&2
  exit 1
fi

chmod +x "$BINARY"
echo "$EXPECTED_VERSION" > "$VERSION_FILE"
echo "gws-connector: installed $ASSET ($TAG) to $DATA_DIR" >&2

#!/bin/bash
# Post-install setup for Gemini CLI extension.
# Downloads the correct gws-mcp binary for the current platform.

set -e

REPO="orieg/gws-connector"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EXT_DIR="$(dirname "$SCRIPT_DIR")"
BIN_DIR="$EXT_DIR/bin"
BINARY="$BIN_DIR/gws-mcp"

# Read version from extension manifest
VERSION=""
if command -v jq >/dev/null 2>&1 && [ -f "$EXT_DIR/gemini-extension.json" ]; then
  VERSION=$(jq -r '.version // empty' "$EXT_DIR/gemini-extension.json" 2>/dev/null || true)
fi

# Skip if binary exists and matches version
VERSION_FILE="$BIN_DIR/.version"
if [ -x "$BINARY" ] && [ -f "$VERSION_FILE" ]; then
  CURRENT=$(cat "$VERSION_FILE" 2>/dev/null || true)
  if [ "$CURRENT" = "$VERSION" ] && [ -n "$VERSION" ]; then
    echo "gws-connector: binary already at v$VERSION"
    exit 0
  fi
fi

mkdir -p "$BIN_DIR"

OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)
case "$ARCH" in
  x86_64|amd64) ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *) echo "gws-connector: unsupported architecture $ARCH" >&2; exit 1 ;;
esac

ASSET="gws-mcp-${OS}-${ARCH}"
TAG="v${VERSION}"
if [ -z "$VERSION" ]; then
  TAG="latest"
  URL="https://github.com/$REPO/releases/latest/download/$ASSET"
else
  URL="https://github.com/$REPO/releases/download/$TAG/$ASSET"
fi

echo "gws-connector: downloading $ASSET ($TAG)..."
if command -v curl >/dev/null 2>&1; then
  curl -fsSL -o "$BINARY" "$URL"
elif command -v wget >/dev/null 2>&1; then
  wget -qO "$BINARY" "$URL"
else
  echo "gws-connector: curl or wget required" >&2
  exit 1
fi

chmod +x "$BINARY"
echo "$VERSION" > "$VERSION_FILE"
echo "gws-connector: installed $ASSET ($TAG)"

#!/bin/sh
# Dispatch to the correct prebuilt gws-mcp binary for this OS/arch.
# Bundled inside the .mcpb next to the platform binaries under server/.
set -e

DIR="$(cd "$(dirname "$0")" && pwd)"
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64 | amd64) ARCH="amd64" ;;
  arm64 | aarch64) ARCH="arm64" ;;
esac

BIN="$DIR/gws-mcp-${OS}-${ARCH}"
if [ ! -x "$BIN" ]; then
  echo "gws-connector: no bundled binary for ${OS}-${ARCH} (looked for $BIN)" >&2
  exit 1
fi

exec "$BIN" --use-dot-names "$@"

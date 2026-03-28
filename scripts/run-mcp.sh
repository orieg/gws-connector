#!/bin/bash
# Wrapper: ensures the gws-mcp binary exists, then exec's it.
# Used by gemini-extension.json so no manual setup is needed.

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
EXT_DIR="$(dirname "$SCRIPT_DIR")"

# Download binary if missing
bash "$SCRIPT_DIR/gemini-setup.sh" >&2

exec "$EXT_DIR/bin/gws-mcp" "$@"

#!/bin/bash
# Sets the release version in every manifest, then verifies them.
# Usage: scripts/bump-version.sh 0.4.2
# mcpb/manifest.json and server.json keep their __VERSION__ placeholders;
# the release workflow renders those from the tag.

set -euo pipefail

VERSION="${1:-}"
VERSION="${VERSION#v}"
if ! [[ "$VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "usage: $0 <major.minor.patch>" >&2
  exit 2
fi

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

# Rewrite only the version value in place so each file keeps its formatting.
# Each JSON manifest has exactly one "version" key.
set_version() {
  local file="$1" pattern="$2" replacement="$3" tmp
  tmp="$(mktemp)"
  sed -E "s/${pattern}/${replacement}/" "$file" > "$tmp"
  mv "$tmp" "$file"
}

JSON_PATTERN='("version"[[:space:]]*:[[:space:]]*")[^"]*(")'
set_version .claude-plugin/plugin.json "$JSON_PATTERN" "\\1${VERSION}\\2"
set_version .claude-plugin/marketplace.json "$JSON_PATTERN" "\\1${VERSION}\\2"
set_version gemini-extension.json "$JSON_PATTERN" "\\1${VERSION}\\2"
set_version Makefile '^VERSION[[:space:]]*:=.*' "VERSION := ${VERSION}"

scripts/check-versions.sh "$VERSION"

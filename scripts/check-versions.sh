#!/bin/bash
# Verifies every release manifest carries the same version.
# Usage: scripts/check-versions.sh [expected-version]
#   expected-version may be given with or without a leading "v" (e.g. a git tag).
# Called by CI on every PR and by the release workflow before building.

set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

command -v jq >/dev/null 2>&1 || { echo "check-versions: jq is required" >&2; exit 2; }

declare -a NAMES=(
  ".claude-plugin/plugin.json"
  ".claude-plugin/marketplace.json"
  "gemini-extension.json"
  "Makefile"
)
declare -a VALUES=(
  "$(jq -r '.version // empty' .claude-plugin/plugin.json)"
  "$(jq -r '.plugins[] | select(.name == "gws") | .version // empty' .claude-plugin/marketplace.json)"
  "$(jq -r '.version // empty' gemini-extension.json)"
  "$(sed -n 's/^VERSION[[:space:]]*:=[[:space:]]*//p' Makefile)"
)

EXPECTED="${1:-}"
EXPECTED="${EXPECTED#v}"
# Without an explicit version, compare everything against plugin.json.
[ -n "$EXPECTED" ] || EXPECTED="${VALUES[0]}"

status=0
for i in "${!NAMES[@]}"; do
  if [ "${VALUES[$i]}" = "$EXPECTED" ]; then
    printf '  ok        %-34s %s\n' "${NAMES[$i]}" "${VALUES[$i]}"
  else
    printf '  MISMATCH  %-34s %s (expected %s)\n' "${NAMES[$i]}" "${VALUES[$i]:-<missing>}" "$EXPECTED"
    status=1
  fi
done

if [ "$status" -ne 0 ]; then
  echo "check-versions: manifests disagree; fix with scripts/bump-version.sh <version>" >&2
fi
exit "$status"

#!/bin/bash
# Session init hook: outputs account summary for Claude context

STATE_DIR="${GWS_STATE_DIR:-$HOME/.claude/channels/gws}"
ACCOUNTS_FILE="$STATE_DIR/accounts.json"

if [ ! -f "$ACCOUNTS_FILE" ]; then
  echo "GWS Connector: No accounts configured. Run /gws:configure to get started."
  exit 0
fi

# Try jq first, fall back to python3, fall back to grep
if command -v jq >/dev/null 2>&1; then
  COUNT=$(jq '.accounts | length' "$ACCOUNTS_FILE" 2>/dev/null || echo "0")
  if [ "$COUNT" = "0" ]; then
    echo "GWS Connector: No accounts configured. Run /gws:configure to get started."
    exit 0
  fi
  echo "GWS Connector: $COUNT account(s) connected"
  jq -r '.accounts[] | "  - \(.label) (\(.email))\(if .default then " [DEFAULT]" else "" end)"' "$ACCOUNTS_FILE" 2>/dev/null
elif command -v python3 >/dev/null 2>&1; then
  COUNT=$(python3 -c "import json; d=json.load(open('$ACCOUNTS_FILE')); print(len(d.get('accounts',[])))" 2>/dev/null || echo "0")
  if [ "$COUNT" = "0" ]; then
    echo "GWS Connector: No accounts configured. Run /gws:configure to get started."
    exit 0
  fi
  echo "GWS Connector: $COUNT account(s) connected"
  python3 -c "
import json
with open('$ACCOUNTS_FILE') as f:
    data = json.load(f)
for a in data.get('accounts', []):
    default = ' [DEFAULT]' if a.get('default') else ''
    print(f\"  - {a['label']} ({a['email']}){default}\")
" 2>/dev/null
else
  # Minimal fallback — just report the file exists
  echo "GWS Connector: accounts configured (install jq for detailed status)"
fi

echo "Use account labels or emails to target a specific account in gws.* tools."

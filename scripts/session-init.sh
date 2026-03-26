#!/bin/bash
# Session init hook: outputs account summary for Claude context

STATE_DIR="${GWS_STATE_DIR:-$HOME/.claude/channels/gws}"
ACCOUNTS_FILE="$STATE_DIR/accounts.json"

if [ ! -f "$ACCOUNTS_FILE" ]; then
  echo "GWS Connector: No accounts configured. Run /gws:configure to get started."
  exit 0
fi

# Count accounts
COUNT=$(python3 -c "import json; d=json.load(open('$ACCOUNTS_FILE')); print(len(d.get('accounts',[])))" 2>/dev/null || echo "0")

if [ "$COUNT" = "0" ]; then
  echo "GWS Connector: No accounts configured. Run /gws:configure to get started."
  exit 0
fi

echo "GWS Connector: $COUNT account(s) connected"

# List accounts with labels
python3 -c "
import json
with open('$ACCOUNTS_FILE') as f:
    data = json.load(f)
for a in data.get('accounts', []):
    default = ' [DEFAULT]' if a.get('default') else ''
    print(f\"  - {a['label']} ({a['email']}){default}\")
" 2>/dev/null

echo "Use account labels or emails to target a specific account in gws.* tools."

---
name: set-default
description: Set which Google account is used by default when no account is specified.
command: gws:default
args: "<label-or-email>"
---

# Set Default Account

Change which account is used when no `account` parameter is specified in tool calls.

## Steps

1. Call `gws.accounts.set_default` with the provided label or email
2. Confirm the change

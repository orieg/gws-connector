---
name: remove-account
description: Disconnect a Google account from the GWS connector and delete its tokens.
command: gws:remove-account
args: "<label-or-email>"
---

# Remove Account

Disconnect a Google account and delete its stored tokens.

## Steps

1. Confirm with the user which account to remove
2. Call `gws.accounts.remove` with the account label or email
3. Report success

## Notes

- If the removed account was the default, the next remaining account becomes default
- Tokens are permanently deleted from the OS keychain

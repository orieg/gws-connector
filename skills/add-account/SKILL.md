---
name: add-account
description: Connect a new Google account to the GWS connector with a custom label.
command: gws:add-account
args: "<label>"
---

# Add Account

Connect a new Google account via OAuth authorization.

## Usage

The user provides a label (e.g., "work", "personal", "client-acme") that will be used to reference this account.

## Steps

1. Call `gws.accounts.add` with the provided label
2. This will open a browser for Google OAuth consent
3. The user authorizes access to Gmail, Calendar, and Drive
4. Report success with the connected email and total account count

## Notes

- If `GWS_GOOGLE_CLIENT_ID` is not set, the tool will return setup instructions
- Each account requires a separate OAuth authorization
- The first account added becomes the default

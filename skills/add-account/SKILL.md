---
name: add-account
description: Connect a new Google account to the GWS connector with a custom label. Supports per-account OAuth credentials for different organizations.
---

# Add Account

Connect a new Google account via OAuth authorization.

## Usage

The user provides a label (e.g., "work", "personal", "client-acme") that will be used to reference this account in all tool calls.

## Steps

1. Ask the user for a label for this account (e.g., "work", "personal", "client-acme")
2. Ask the user for their credentials. Accept either:
   - A **path to a `client_secret_*.json` file** downloaded from GCP — read it and extract `client_id` and `client_secret` from the `installed` object
   - A **Client ID and Client Secret** pasted directly
3. Call `gws.accounts.add` with the label AND `clientId` and `clientSecret` params
4. This opens a browser for Google OAuth consent
5. Report success with the connected email and total account count

## Credential handling

Every account gets its own credentials passed directly — do NOT rely on environment variables.

If the user has a `client_secret_*.json` file, read it with the Read tool. The JSON has this structure:
```json
{"installed": {"client_id": "...", "client_secret": "..."}}
```

If the user already connected an account using the same GCP project, you can reuse those credentials. Check existing accounts with `gws.accounts.list` — the user can confirm if the same project applies.

## Notes

- If no credentials are provided, the tool returns setup instructions (or run /gws:configure)
- The first account added becomes the default
- The browser must be accessible for the OAuth flow

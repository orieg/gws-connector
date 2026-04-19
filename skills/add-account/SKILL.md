---
name: add-account
description: Connect a new Google account to the GWS connector with a custom label. Supports per-account OAuth credentials for different organizations.
---

# Add Account

Connect a new Google account via OAuth authorization.

## Usage

The user provides a label (e.g., "work", "personal", "client-acme") that will be used to reference this account in all tool calls.

## Steps

1. Ask the user for a label for this account
2. Ask the user for their OAuth credentials. They can either:
   - **Point to a downloaded JSON file**: Read the `client_secret_*.json` file to extract `client_id` and `client_secret` from the `installed` object
   - **Provide Client ID and Client Secret directly** as text
   - If the user has already connected accounts, ask if this new account uses the same GCP project or a different one. If the same, offer to reuse the existing client ID (the secret is in the keychain; they may need to provide it again or point to the same JSON file).
3. Call `gws.accounts.add` with the label, `clientId`, and `clientSecret` params. The tool opens the browser and waits up to ~60 seconds for the user to complete consent.
4. The response is one of:
   - **Success** → report the connected email and total account count.
   - **Error** → report and stop.
   - **Status: pending** (user was slow) → call `gws.accounts.complete` with the returned `pendingId`. Repeat until success or error.
5. The client secret is stored securely in the OS keychain.

## Per-account credentials

Each account stores its own OAuth credentials. Different Google Workspace organizations need different GCP OAuth apps. For example:

- `personal` account: uses your personal GCP project's credentials
- `work` account at Company A: uses Company A's GCP project credentials
- `client-acme` account: uses Acme Corp's GCP project credentials

Each org's admin creates their own GCP project with OAuth credentials. See `/gws:configure` for step-by-step GCP setup.

## Notes

- If no credentials are provided, the tool returns setup instructions (or run /gws:configure)
- The first account added becomes the default
- The browser must be accessible for the OAuth flow
- Client secrets are stored in the OS keychain (macOS Keychain, GNOME Keyring), not in config files

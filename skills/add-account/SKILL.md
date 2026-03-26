---
name: add-account
description: Connect a new Google account to the GWS connector with a custom label. Supports per-account OAuth credentials for different organizations.
command: gws:add-account
args: "<label> [--client-id <id> --client-secret <secret>]"
---

# Add Account

Connect a new Google account via OAuth authorization.

## Usage

The user provides a label (e.g., "work", "personal", "client-acme") that will be used to reference this account in all tool calls.

## Steps

1. Ask the user for a label for this account
2. Ask if this account belongs to a different organization than previously connected accounts:
   - **Same org / personal Gmail**: Use global credentials (no extra params needed)
   - **Different org**: Ask for that org's OAuth Client ID and Client Secret (from their GCP project). Pass them as `clientId` and `clientSecret` params.
3. Call `gws.accounts.add` with the label (and per-account credentials if provided)
4. This opens a browser for Google OAuth consent
5. Report success with the connected email and total account count

## Per-account credentials

Different Google Workspace organizations may require different GCP OAuth apps. For example:

- `personal` account: uses global `GWS_GOOGLE_CLIENT_ID`
- `work` account at Company A: uses Company A's OAuth client ID
- `client-acme` account: uses Acme Corp's OAuth client ID

Each org's admin creates their own GCP project with OAuth credentials. Pass these when adding the account.

## Notes

- If no credentials are available at all, the tool returns setup instructions (or run /gws:configure)
- The first account added becomes the default
- The browser must be accessible for the OAuth flow

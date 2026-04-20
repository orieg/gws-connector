---
name: reauth
description: Re-authorize a connected GWS account to refresh tokens or pick up new OAuth scopes without changing account settings.
---

# Re-authorize Account

Re-run OAuth for an existing account. This refreshes the token and picks up any new scopes added in the GCP project — without changing the account label, default status, or credentials.

## When to use

- After adding new OAuth scopes in the GCP Console (e.g., adding `gmail.send`)
- When token refresh fails with auth errors
- When the user gets permission denied on a specific API
- When upgrading from v0.2.x to v0.3.0+ (Sheets and Docs scopes are new)

## Upgrading to v0.3.0 — tell the user before re-auth

If the user is re-authorizing because they just upgraded to v0.3.0 (or later),
they must approve two new scopes. Before calling `gws.accounts.reauth`, say
this to them (adapt to their wording):

> The v0.3.0 release adds Google Sheets and Google Docs support. Re-authorizing
> grants this server **read and write access to every spreadsheet and document
> in this account's Google Drive**, including files shared with the account.
> Make sure this matches your intent before approving the browser consent screen.
> You also need to have added the `spreadsheets` and `documents` scopes (and
> enabled the Sheets and Docs APIs) in the GCP project's OAuth consent screen —
> otherwise consent will fail.

Do not proceed with `gws.accounts.reauth` until the user has acknowledged this.

## Steps

1. If no account is specified, call `gws.accounts.list` and ask which account to re-authorize.
2. Call `gws.accounts.reauth` with the account label or email. The tool opens the browser and waits up to ~60 seconds for the user to finish signing in.
3. The response is one of:
   - **Success** → confirm to the user.
   - **Error** → report and stop.
   - **Status: pending** (user was slow) → call `gws.accounts.complete` with the returned `pendingId`. Repeat until success or error.
4. Tell the user to sign in with the **same Google account**.

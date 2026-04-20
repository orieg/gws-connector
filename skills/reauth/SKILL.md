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

## Steps

1. If no account is specified, call `gws.accounts.list` and ask which account to re-authorize.
2. Call `gws.accounts.reauth` with the account label or email. The tool opens the browser and waits up to ~60 seconds for the user to finish signing in.
3. The response is one of:
   - **Success** → confirm to the user.
   - **Error** → report and stop.
   - **Status: pending** (user was slow) → call `gws.accounts.complete` with the returned `pendingId`. Repeat until success or error.
4. Tell the user to sign in with the **same Google account**.

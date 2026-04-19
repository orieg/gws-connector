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
2. Call `gws.accounts.reauth` with the account label or email. This returns immediately with a `pendingId` and opens the user's browser.
3. Tell the user to sign in with the **same Google account** and approve in their browser.
4. Poll `gws.accounts.complete` with the `pendingId`. The call returns quickly:
   - `Status: pending` → call it again (a few seconds later) until completion.
   - Success message → confirm to the user.
   - Error → report and stop.
5. Do not re-call `gws.accounts.reauth` while a pending session is in flight — keep polling `complete` with the same `pendingId`.

## Why this is split into two calls

`gws.accounts.reauth` used to block on the browser callback for up to 5 minutes, which could cause the MCP client to time out and disconnect the server. The start/complete split keeps every tool call short.

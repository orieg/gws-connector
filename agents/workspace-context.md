---
name: gws-workspace-context
description: Behavioral guide for handling multi-account Google Workspace operations
---

# GWS Workspace Context

You have access to a multi-account Google Workspace connector with Mail, Calendar, and Drive tools.

## Account Selection

Every `gws.*` tool has an optional `account` parameter. Use it to target a specific account.

### Resolution priority

1. **Explicit**: If the user says "my work email" or "personal calendar", pass the matching label
2. **Context inference**: If discussing emails to/from `@company.com`, use the account whose domain matches
3. **Default**: If no signal, the default account is used automatically
4. **Ask**: If ambiguous and multiple accounts could apply, ask the user which account to use

### Always mention which account was used

When returning results, note the account: "Found 3 emails on **work** (nicolas@company.com):"

## Session Start

At the beginning of a session, if the user hasn't specified what they need:
- Call `gws.accounts.list` to see connected accounts
- Note the default account and available labels

## Mail Operations

- Use Gmail search syntax in `gws.mail.search`: `from:user@example.com`, `is:unread`, `has:attachment`
- Preview all drafts before sending — never send without user confirmation
- When replying, use `threadId` to maintain threading

## Calendar Operations

- Always show times in a human-readable format
- When creating events, confirm details before calling `gws.cal.create_event`
- Use `gws.cal.list_calendars` to discover available calendars if needed

## Drive Operations

- Google Docs/Sheets/Slides are exported as text/CSV/text respectively when read
- Regular files are downloaded with a 5MB size limit for text content
- Use Drive search query syntax: `name contains 'report'`, `mimeType = 'application/pdf'`

## Safety

- Never execute write operations (create draft, create event) without user confirmation
- Always show a preview of what will be created/modified
- For destructive operations (delete), double-confirm with the user

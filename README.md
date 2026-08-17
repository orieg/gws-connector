# GWS Connector

[![CI](https://github.com/orieg/gws-connector/actions/workflows/ci.yml/badge.svg)](https://github.com/orieg/gws-connector/actions/workflows/ci.yml)
[![Release](https://img.shields.io/github/v/release/orieg/gws-connector)](https://github.com/orieg/gws-connector/releases/latest)
[![Go](https://img.shields.io/github/go-mod/go-version/orieg/gws-connector)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Multi-account Google Workspace MCP server — connect multiple Gmail, Google Calendar, and Google Drive accounts with smart routing.

Works with **Claude Code**, **Gemini CLI**, **GitHub Copilot**, **Cursor**, **OpenAI Codex**, and any MCP-compatible client.

## Why

Most AI coding assistants support a single Google account. If you use multiple Google accounts (personal + work, multiple clients, different orgs), you need to switch between them manually. This MCP server lets you connect them all at once and route requests by label, email, or domain.

## Features

- **Multi-account** — connect unlimited Gmail and Google Workspace accounts
- **Smart routing** — target accounts by label (`work`), email, or domain
- **Per-account OAuth** — different orgs can use their own GCP credentials
- **Secure storage** — client secrets and tokens stored in OS keychain (file fallback on Linux without GNOME Keyring)
- **47 tools** — Mail (11), Calendar (7), Drive (3), Sheets (6), Docs (4), Contacts (2), Tasks (5), Slides (3), account management (6)
- **Account management** — add, remove, set default, list accounts
- **Cross-platform** — standard MCP server works with any compatible client

## How it compares

There are several good Google Workspace MCP servers. GWS Connector is the one to
pick when **multiple accounts** and **operational simplicity** matter:

| | Most GWS MCP servers | **GWS Connector** |
|---|---|---|
| **Accounts** | One account per server instance | Unlimited accounts in one instance, routed by label / email / domain |
| **Multiple orgs** | Shared OAuth app | Per-account OAuth — each org uses its own GCP credentials |
| **Credential storage** | `.env` / plaintext token files | OS keychain (Keychain / GNOME Keyring / Credential Manager) |
| **Runtime** | Python/Node + dependencies | Single static Go binary, no runtime to install |
| **Clients** | Usually one | Claude Code, Gemini CLI, Copilot, Cursor, Codex, any MCP client |
| **Install** | Manual config | Claude Code plugin, Gemini extension, one-click `.mcpb`, MCP Registry |

If you only ever use a single Google account and want the widest possible tool
surface (Slides, Forms, Chat, …), a single-account server like
[taylorwilsdon/google_workspace_mcp](https://github.com/taylorwilsdon/google_workspace_mcp)
may fit better. GWS Connector focuses on doing multi-account Gmail / Calendar /
Drive / Sheets / Docs / Tasks cleanly and securely.

## Upgrading (Google Tasks)

The Google Tasks tools (`gws.tasks.*`) are added behind one new OAuth scope
(`tasks`). **Existing users must re-authorize each connected account** so new
tokens are minted with this scope:

```
/gws:reauth
```

Before approving the browser consent screen, review what the new scope grants —
full read and write access to the account's Google Tasks lists and tasks. See
the [scope rationale table](#google-cloud-setup) below for details.

You must also add the `tasks` scope and enable the **Tasks API** in your GCP
project's OAuth consent screen configuration before re-auth, or the consent
screen will reject the request. Until an account is re-authorized, the
`gws.tasks.*` tools return an insufficient-scope error naming the reauth tool
to run.

## Upgrading — Contacts / People API

The Contacts tools (`gws.contacts.search`, `gws.contacts.directory_search`)
add two new read-only OAuth scopes (`contacts.readonly`, `directory.readonly`).
**Existing users must re-authorize each connected account** so new tokens are
minted with these scopes:

```
/gws:reauth
```

Before approving the browser consent screen, review what the new scopes grant —
read-only access to your Google Contacts and (for Workspace accounts) the
organization directory. See the [scope rationale table](#google-cloud-setup)
below for details.

You must also enable the **People API** and add the two new scopes in your GCP
project's OAuth consent screen configuration before re-auth, or the consent
screen will reject the request. `gws.contacts.directory_search` requires a
Google Workspace account — personal Gmail accounts have no organization
directory and receive a clear explanatory message instead of results.

## Upgrading (Google Slides tools)

The Slides tools (`gws.slides.*`) add one new OAuth scope,
`https://www.googleapis.com/auth/presentations`. **Existing users must
re-authorize each connected account** so new tokens are minted with the
Slides scope:

```
/gws:reauth
```

You must also enable the **Slides API** and add the `presentations` scope in
your GCP project's OAuth consent screen configuration before re-auth, or the
consent screen will reject the request (see [Google Cloud
Setup](#google-cloud-setup)). Until you re-authorize, `gws.slides.*` calls
return a scope error telling the agent to run `gws.accounts.reauth`; all other
tools keep working.

## Upgrading from v0.2.x

v0.3.0 adds native Google Sheets and Google Docs tools behind two new OAuth
scopes (`spreadsheets`, `documents`). **Existing users must re-authorize
each connected account** so new tokens are minted with these scopes:

```
/gws:reauth
```

Before approving the browser consent screen, review what the new scopes
grant — full read and write access to every spreadsheet and document in that
account's Google Drive, including files shared with the account. See the
[scope rationale table](#google-cloud-setup) below for details.

You must also add the two new scopes (and enable the Sheets and Docs APIs)
in your GCP project's OAuth consent screen configuration before re-auth,
or the consent screen will reject the request.

## Quick Start (Claude Code)

**1. Install the plugin** — run these two commands inside Claude Code:

```
/plugin marketplace add orieg/gws-connector
/plugin install gws@gws-connector
```

**2. Set up Google Cloud credentials** — the interactive wizard walks you through everything:

```
/gws:configure
```

This creates a GCP project, enables APIs, and connects your first account (~5 minutes). See [Google Cloud Setup](#google-cloud-setup) if you prefer manual steps.

**3. Connect additional accounts:**

```
/gws:add-account
```

Each account can use different OAuth credentials from different GCP projects.

### Gemini CLI

```bash
gemini extensions install https://github.com/orieg/gws-connector
```

The binary is downloaded automatically on first use. Then connect accounts inside Gemini:

```
gws.accounts.add(label: "personal", clientId: "your-client-id", clientSecret: "your-secret")
```

### Other clients

<details>
<summary><strong>GitHub Copilot / Cursor / Codex / Any MCP client</strong></summary>

Download a [prebuilt binary](https://github.com/orieg/gws-connector/releases/latest) or build from source:

```bash
git clone https://github.com/orieg/gws-connector && cd gws-connector && make build
```

Then configure your client:

| Client | Config |
|--------|--------|
| **GitHub Copilot** | Auto-detects from `.vscode/mcp.json`, or add `"command": "/path/to/gws-mcp"` to VS Code MCP settings |
| **Cursor** | Auto-detects from `.cursor/mcp.json`, or add via Settings → MCP Servers |
| **Codex CLI** | Auto-detects from `codex.json` |
| **Claude Code (MCP only)** | `claude mcp add --transport stdio gws-connector --scope user -- /path/to/gws-mcp --use-dot-names` |
| **Any MCP client** | `gws-mcp [--use-dot-names]` over stdio |

Connect accounts via MCP tool call:

```
gws.accounts.add(label: "personal", clientId: "your-client-id", clientSecret: "your-secret")
```

Environment variables (all optional): `GWS_GOOGLE_CLIENT_ID`, `GWS_GOOGLE_CLIENT_SECRET`, `GWS_STATE_DIR`

The `--use-dot-names` flag uses `gws.mail.search` naming; without it, tools use `gws_mail_search`.

</details>

<details>
<summary><strong>Local development / testing</strong></summary>

```bash
git clone https://github.com/orieg/gws-connector
cd gws-connector
make build
claude --plugin-dir ./
```

Use `/reload-plugins` inside the session after making changes. Run `claude --debug --plugin-dir ./` to troubleshoot plugin loading.

</details>

<details>
<summary><strong>Claude Desktop / MCPB bundle</strong></summary>

Each [release](https://github.com/orieg/gws-connector/releases/latest) attaches a
one-click **`gws-mcp.mcpb`** bundle. Download it and open it with Claude Desktop
(Settings → Extensions → install from file), or drag it in. The bundle contains
the binaries for macOS and Linux and picks the right one for your machine
automatically. You still complete the [Google Cloud setup](#google-cloud-setup)
and connect accounts on first use.

The server is also published to the
[official MCP Registry](https://registry.modelcontextprotocol.io) as
`io.github.orieg/gws-connector`, so MCP clients that browse the registry can find
and install it directly.

</details>

<details>
<summary><strong>Docker</strong></summary>

A multi-arch image is published to GHCR on each release:

```bash
docker run -i --rm ghcr.io/orieg/gws-connector:latest
```

The server speaks MCP over stdio. Interactive OAuth (`accounts.add` / `reauth`)
opens a browser and stores secrets in the OS keychain, so it needs host access —
day-to-day use is best via the native binary, the Claude Code plugin, or the
Gemini extension. The image is well suited to headless stdio integrations and to
registry/introspection checks. Persist the account registry across runs by
mounting a volume and pointing `GWS_STATE_DIR` at it:

```bash
docker run -i --rm -v gws-state:/state -e GWS_STATE_DIR=/state \
  ghcr.io/orieg/gws-connector:latest
```

</details>

## Usage

All `gws.*` tools accept an optional `account` parameter:

```
# Uses default account
gws.mail.search(q: "is:unread")

# Target by label
gws.cal.list_events(account: "work")

# Target by email
gws.drive.search(account: "alice@company.com", q: "quarterly report")
```

### Available tools

| Tool | Description |
|------|-------------|
| `gws.accounts.list` | List all connected accounts |
| `gws.accounts.add` | Connect a new account (waits up to ~60s; returns `pendingId` if slower) |
| `gws.accounts.reauth` | Re-authorize an account (waits up to ~60s; returns `pendingId` if slower) |
| `gws.accounts.complete` | Finalize a pending OAuth flow (only needed if add/reauth returned `pendingId`) |
| `gws.accounts.remove` | Disconnect an account |
| `gws.accounts.set_default` | Change the default account |
| `gws.mail.search` | Search messages (Gmail query syntax) |
| `gws.mail.read_message` | Read a specific message |
| `gws.mail.read_thread` | Read an entire thread |
| `gws.mail.create_draft` | Create an email draft |
| `gws.mail.send_draft` | Send an existing draft |
| `gws.mail.forward` | Build a forward draft of a message (does not send) |
| `gws.mail.get_attachment` | Fetch a message attachment's bytes (base64) |
| `gws.mail.list_labels` | List Gmail labels |
| `gws.mail.create_label` | Create a new label |
| `gws.mail.modify_message` | Add/remove labels on a message |
| `gws.mail.get_profile` | Get account profile info |
| `gws.cal.list_events` | List calendar events |
| `gws.cal.get_event` | Get event details |
| `gws.cal.create_event` | Create a calendar event |
| `gws.cal.update_event` | Update/reschedule an event (patch semantics) |
| `gws.cal.delete_event` | Delete/cancel an event |
| `gws.cal.free_busy` | Query free/busy across calendars |
| `gws.cal.list_calendars` | List available calendars |
| `gws.drive.search` | Search files in Drive |
| `gws.drive.read_file` | Read file content/metadata |
| `gws.drive.list_folder` | List folder contents |
| `gws.sheets.read_range` | Read a single A1 range from a spreadsheet |
| `gws.sheets.write_range` | Write cell values to a range |
| `gws.sheets.append` | Append rows after a table (additive, never overwrites) |
| `gws.sheets.clear` | Clear values in a range (formatting left intact) |
| `gws.sheets.create` | Create a new spreadsheet |
| `gws.sheets.list_tabs` | List tabs (sheets) in a spreadsheet |
| `gws.docs.read` | Read a document as plain text |
| `gws.docs.insert_text` | Insert literal text at a location |
| `gws.docs.replace_text` | Replace all occurrences of a literal substring |
| `gws.docs.create` | Create a new document |
| `gws.contacts.search` | Search your own contacts by name/email/phone (returns name, emails, phones) |
| `gws.contacts.directory_search` | Search the Workspace org directory (returns name, emails); Workspace accounts only |
| `gws.tasks.list_tasklists` | List the account's task lists |
| `gws.tasks.list` | List tasks in a list (add `showCompleted` for done tasks) |
| `gws.tasks.create` | Create a task (`due` is RFC3339; only the date is stored) |
| `gws.tasks.complete` | Mark a task completed (reversible) |
| `gws.tasks.delete` | Permanently delete a task |
| `gws.slides.get` | Read a presentation (slide count + per-slide text) |
| `gws.slides.create` | Create a new presentation |
| `gws.slides.batch_update` | Apply raw Slides API requests to a presentation |

### Skills

Interactive workflows available in both Claude Code and Gemini CLI:

| Skill | Description | Claude Code | Gemini CLI |
|-------|-------------|-------------|------------|
| configure | Interactive setup wizard | `/gws:configure` | "run the GWS configure skill" |
| add-account | Connect a new account | `/gws:add-account` | "add a new GWS account" |
| remove-account | Disconnect an account | `/gws:remove-account` | "remove a GWS account" |
| list-accounts | Show connected accounts | `/gws:list-accounts` | "list my GWS accounts" |
| set-default | Change default account | `/gws:set-default` | "set my default GWS account" |
| reauth | Refresh tokens/scopes | `/gws:reauth` | "reauth my GWS accounts" |

## Recipes

Once accounts are connected, just ask your assistant in plain language — it picks
the tools and the account. Examples:

- **Morning triage across accounts** — "Summarize my unread email from the last
  24 hours across all accounts, grouped by account, and flag anything that needs
  a reply today."
- **Draft a reply in a thread** — "Find the thread with Acme about the Q3 invoice
  on my **work** account and draft a reply confirming the new date. Don't send it."
- **Turn an email into a calendar event** — "Read the latest message from the
  events team and create a calendar event on my **personal** calendar with the
  date and location from it."
- **Cross-account digest** — "What meetings do I have tomorrow across my work and
  personal calendars? List them in one timeline."
- **Find and summarize a doc** — "Search my **client-acme** Drive for the latest
  'statement of work' and give me the key deliverables and dates."
- **Log to a spreadsheet** — "Append a row to the 'Expenses' sheet in my personal
  Drive: today's date, 'AWS', 42.50."
- **Keep inbox tidy** — "Label all unread messages from newsletters@ as
  'Newsletters' and mark them read on my **personal** account."

Tips:

- Target an account explicitly with its label ("on my **work** account"), by
  email, or by domain — otherwise the default account is used.
- Write operations (drafts, events, sheet/doc edits) are previewed for your
  confirmation before anything is sent or changed.

## Google Cloud Setup

One-time setup (~5 minutes):

1. **Go to [Google Cloud Console](https://console.cloud.google.com/)** and create a new project (e.g., "GWS Connector")

2. **Enable APIs** — click each link and hit "Enable":
   - [Gmail API](https://console.cloud.google.com/apis/library/gmail.googleapis.com)
   - [Calendar API](https://console.cloud.google.com/apis/library/calendar-json.googleapis.com)
   - [Drive API](https://console.cloud.google.com/apis/library/drive.googleapis.com)
   - [Sheets API](https://console.cloud.google.com/apis/library/sheets.googleapis.com)
   - [Docs API](https://console.cloud.google.com/apis/library/docs.googleapis.com)
   - [People API](https://console.cloud.google.com/apis/library/people.googleapis.com)
   - [Tasks API](https://console.cloud.google.com/apis/library/tasks.googleapis.com)
   - [Slides API](https://console.cloud.google.com/apis/library/slides.googleapis.com)

3. **Configure the [OAuth consent screen](https://console.cloud.google.com/auth/consent)**:
   - Choose "External" (or "Internal" for Google Workspace orgs)
   - Fill in the app name (e.g., "Claude GWS") and your email for support contact
   - Click "Save"

4. **Add scopes** — go to [Data Access](https://console.cloud.google.com/auth/scopes):
   - Click "Add or Remove Scopes"
   - Add these 11 scopes (paste into the "Manually add scopes" box):
     - `https://www.googleapis.com/auth/gmail.modify`
     - `https://www.googleapis.com/auth/calendar`
     - `https://www.googleapis.com/auth/drive`
     - `https://www.googleapis.com/auth/spreadsheets`
     - `https://www.googleapis.com/auth/documents`
     - `https://www.googleapis.com/auth/contacts.readonly`
     - `https://www.googleapis.com/auth/directory.readonly`
     - `https://www.googleapis.com/auth/tasks`
     - `https://www.googleapis.com/auth/presentations`
     - `https://www.googleapis.com/auth/userinfo.email`
     - `https://www.googleapis.com/auth/userinfo.profile`
   - Click "Update", then "Save"

   **Why each scope is requested:**

   | Scope | Purpose | Tools |
   |-------|---------|-------|
   | `gmail.modify` | Read, draft, modify messages and labels | `gws.mail.*` |
   | `calendar` | Read and create/update events | `gws.cal.*` |
   | `drive` | Search and read files and metadata across Drive | `gws.drive.*` |
   | `spreadsheets` | Read and write Google Sheets cell data and metadata | `gws.sheets.*` |
   | `documents` | Read and write Google Docs content | `gws.docs.*` |
   | `contacts.readonly` | Read-only search of your own Google Contacts | `gws.contacts.search` |
   | `directory.readonly` | Read-only search of the Workspace org directory (Workspace accounts only) | `gws.contacts.directory_search` |
   | `tasks` | Read and write Google Tasks lists and tasks | `gws.tasks.*` |
   | `presentations` | Read and write Google Slides content | `gws.slides.*` |
   | `userinfo.email` | Identify the authorizing account (email match on reauth) | account management |
   | `userinfo.profile` | Store a display name alongside the email | account management |

5. **Add test users** — go to [Audience](https://console.cloud.google.com/auth/audience):
   - Add each Google email address you plan to connect
   - ⚠️ **This is required** — without this you'll get "Access blocked: has not completed the Google verification process" (error 403) during OAuth

6. **Create OAuth credentials** — go to [Clients](https://console.cloud.google.com/auth/clients):
   - Click "+ Create Client" → "OAuth client ID"
   - Application type: **Desktop app**
   - Click "Create"
   - **Download the JSON file** (click the download icon) — this contains your Client ID and Client Secret

### Multiple organizations

If you connect accounts from different Google Workspace orgs, each org needs its own GCP project. Create OAuth credentials in each project and provide them when connecting:

```
gws.accounts.add(label: "work", clientId: "work-client-id", clientSecret: "work-secret")
gws.accounts.add(label: "personal", clientId: "personal-client-id", clientSecret: "personal-secret")
```

Client secrets are stored in the OS keychain. Client IDs are stored in the account registry.

## Architecture

```
gws-connector/
├── cmd/gws-mcp/                 # MCP server entrypoint
├── internal/
│   ├── accounts/                # Account registry & router
│   ├── auth/                    # OAuth flow, token store, client factory
│   ├── server/                  # MCP tool registration & dispatch
│   └── services/                # Gmail, Calendar, Drive API wrappers
│
├── .claude-plugin/              # Claude Code plugin manifest + marketplace
├── .mcp.json                    # Claude Code MCP config
├── gemini-extension.json        # Gemini CLI extension manifest
├── CONTEXT.md                   # Shared behavioral context (both agents)
├── skills/                      # Slash commands (Claude Code + Gemini CLI)
├── hooks/                       # Claude Code session hooks
├── agents/                      # Claude Code workspace agent
│
├── .vscode/mcp.json             # GitHub Copilot MCP config
├── .cursor/mcp.json             # Cursor MCP config
└── codex.json                   # OpenAI Codex CLI config
```

- **Token storage**: OS keychain (macOS Keychain, GNOME Keyring, Windows Credential Manager) with automatic file fallback
- **Client secrets**: OS keychain per account (not stored in config files)
- **Account registry**: JSON file at `~/.claude/channels/gws/accounts.json` (contains client IDs and metadata, no secrets)
- **Credential resolution**: per-account credentials (keychain) → global env var fallback
- **Protocol**: MCP (Model Context Protocol) over stdio — compatible with any MCP client

## Development

```bash
make build          # Build binary
make test           # Run tests with race detector
make test-verbose   # Run tests with verbose output
make lint           # Run go vet
make release        # Cross-compile for all platforms
make clean          # Remove build artifacts
```

## License

MIT — see [LICENSE](LICENSE).

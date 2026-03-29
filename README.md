# GWS Connector

Multi-account Google Workspace MCP server — connect multiple Gmail, Google Calendar, and Google Drive accounts with smart routing.

Works with **Claude Code**, **Gemini CLI**, **GitHub Copilot**, **Cursor**, **OpenAI Codex**, and any MCP-compatible client.

## Why

Most AI coding assistants support a single Google account. If you use multiple Google accounts (personal + work, multiple clients, different orgs), you need to switch between them manually. This MCP server lets you connect them all at once and route requests by label, email, or domain.

## Features

- **Multi-account** — connect unlimited Gmail and Google Workspace accounts
- **Smart routing** — target accounts by label (`work`), email, or domain
- **Per-account OAuth** — different orgs can use their own GCP credentials
- **Secure storage** — client secrets and tokens stored in OS keychain (file fallback on Linux without GNOME Keyring)
- **21 tools** — Mail (search, read, draft, send, labels), Calendar (list, get, create), Drive (search, read, list)
- **Account management** — add, remove, set default, list accounts
- **Cross-platform** — standard MCP server works with any compatible client

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
| `gws.accounts.add` | Connect a new account |
| `gws.accounts.remove` | Disconnect an account |
| `gws.accounts.set_default` | Change the default account |
| `gws.accounts.reauth` | Re-authorize an account |
| `gws.mail.search` | Search messages (Gmail query syntax) |
| `gws.mail.read_message` | Read a specific message |
| `gws.mail.read_thread` | Read an entire thread |
| `gws.mail.create_draft` | Create an email draft |
| `gws.mail.send_draft` | Send an existing draft |
| `gws.mail.list_labels` | List Gmail labels |
| `gws.mail.create_label` | Create a new label |
| `gws.mail.modify_message` | Add/remove labels on a message |
| `gws.mail.get_profile` | Get account profile info |
| `gws.cal.list_events` | List calendar events |
| `gws.cal.get_event` | Get event details |
| `gws.cal.create_event` | Create a calendar event |
| `gws.cal.list_calendars` | List available calendars |
| `gws.drive.search` | Search files in Drive |
| `gws.drive.read_file` | Read file content/metadata |
| `gws.drive.list_folder` | List folder contents |

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

## Google Cloud Setup

One-time setup (~5 minutes):

1. **Go to [Google Cloud Console](https://console.cloud.google.com/)** and create a new project (e.g., "GWS Connector")

2. **Enable APIs** — click each link and hit "Enable":
   - [Gmail API](https://console.cloud.google.com/apis/library/gmail.googleapis.com)
   - [Calendar API](https://console.cloud.google.com/apis/library/calendar-json.googleapis.com)
   - [Drive API](https://console.cloud.google.com/apis/library/drive.googleapis.com)

3. **Configure the [OAuth consent screen](https://console.cloud.google.com/auth/consent)**:
   - Choose "External" (or "Internal" for Google Workspace orgs)
   - Fill in the app name (e.g., "Claude GWS") and your email for support contact
   - Click "Save"

4. **Add scopes** — go to [Data Access](https://console.cloud.google.com/auth/scopes):
   - Click "Add or Remove Scopes"
   - Add these 5 scopes (paste into the "Manually add scopes" box):
     - `https://www.googleapis.com/auth/gmail.modify`
     - `https://www.googleapis.com/auth/calendar`
     - `https://www.googleapis.com/auth/drive`
     - `https://www.googleapis.com/auth/userinfo.email`
     - `https://www.googleapis.com/auth/userinfo.profile`
   - Click "Update", then "Save"

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

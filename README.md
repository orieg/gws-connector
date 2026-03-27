# GWS Connector

Multi-account Google Workspace MCP server — connect multiple Gmail, Google Calendar, and Google Drive accounts with smart routing.

Works with **Claude Code**, **GitHub Copilot**, **Cursor**, **Windsurf**, **OpenAI Codex**, and any MCP-compatible client.

## Why

Most AI coding assistants support a single Google account. If you use multiple Google accounts (personal + work, multiple clients, different orgs), you need to switch between them manually. This MCP server lets you connect them all at once and route requests by label, email, or domain.

## Features

- **Multi-account** — connect unlimited Gmail and Google Workspace accounts
- **Smart routing** — target accounts by label (`work`), email, or domain
- **Per-account OAuth** — different orgs can use their own GCP credentials
- **17 tools** — Mail (search, read, draft, labels), Calendar (list, get, create), Drive (search, read, list)
- **Account management** — add, remove, set default, list accounts
- **Cross-platform** — standard MCP server works with any compatible client

## Quick Start

### Prerequisites

- Go 1.21+ installed
- A Google Cloud project with OAuth credentials ([setup guide below](#google-cloud-setup))

### Build

```bash
git clone https://github.com/orieg/gws-connector
cd gws-connector
make build
```

### Configure credentials

```bash
# Add to ~/.zshrc or ~/.bashrc
export GWS_GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GWS_GOOGLE_CLIENT_SECRET="your-client-secret"
```

### Install per platform

<details>
<summary><strong>Claude Code</strong></summary>

**Option A — Plugin install (recommended):**

```
/plugin install /path/to/gws-connector
```

This auto-registers the MCP server, skills (`/gws:configure`, `/gws:add-account`), and hooks.

**Option B — Marketplace:**

```
/plugin marketplace add orieg/gws-connector
/plugin install gws-connector
```

Then run `/gws:configure` for an interactive setup wizard.

</details>

<details>
<summary><strong>GitHub Copilot (VS Code)</strong></summary>

The repo includes `.vscode/mcp.json`. After building, Copilot Chat will auto-detect the MCP server.

Or add manually to your VS Code settings:

```json
{
  "mcp": {
    "servers": {
      "gws-connector": {
        "command": "/path/to/gws-connector/bin/gws-mcp",
        "env": {
          "GWS_GOOGLE_CLIENT_ID": "your-client-id",
          "GWS_GOOGLE_CLIENT_SECRET": "your-secret"
        }
      }
    }
  }
}
```

</details>

<details>
<summary><strong>Cursor</strong></summary>

The repo includes `.cursor/mcp.json`. After building, Cursor will auto-detect the MCP server.

Or add manually via **Settings → MCP Servers → Add**:

```json
{
  "mcpServers": {
    "gws-connector": {
      "command": "/path/to/gws-connector/bin/gws-mcp",
      "env": {
        "GWS_GOOGLE_CLIENT_ID": "your-client-id",
        "GWS_GOOGLE_CLIENT_SECRET": "your-secret"
      }
    }
  }
}
```

</details>

<details>
<summary><strong>OpenAI Codex CLI</strong></summary>

Add to your `codex.json` (the repo includes one):

```json
{
  "mcpServers": {
    "gws-connector": {
      "command": "/path/to/gws-connector/bin/gws-mcp",
      "env": {
        "GWS_GOOGLE_CLIENT_ID": "your-client-id",
        "GWS_GOOGLE_CLIENT_SECRET": "your-secret"
      }
    }
  }
}
```

</details>

<details>
<summary><strong>Any MCP client</strong></summary>

The `gws-mcp` binary is a standard MCP server speaking JSON-RPC over stdio. Point your client to:

```
./bin/gws-mcp [--use-dot-names]
```

Environment variables:
- `GWS_GOOGLE_CLIENT_ID` — OAuth client ID (required)
- `GWS_GOOGLE_CLIENT_SECRET` — OAuth client secret (required)
- `GWS_STATE_DIR` — state directory (default: `~/.claude/channels/gws`)

The `--use-dot-names` flag uses `gws.mail.search` naming; without it, tools use `gws_mail_search`.

</details>

### Connect accounts

```
# Via MCP tool call
gws.accounts.add(label: "personal")

# Or in Claude Code
/gws:add-account
```

This opens a browser for Google OAuth. The first account becomes the default. Add more by running the command again.

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
| `gws.mail.search` | Search messages (Gmail query syntax) |
| `gws.mail.read_message` | Read a specific message |
| `gws.mail.read_thread` | Read an entire thread |
| `gws.mail.create_draft` | Create an email draft |
| `gws.mail.list_labels` | List Gmail labels |
| `gws.mail.get_profile` | Get account profile info |
| `gws.cal.list_events` | List calendar events |
| `gws.cal.get_event` | Get event details |
| `gws.cal.create_event` | Create a calendar event |
| `gws.cal.list_calendars` | List available calendars |
| `gws.drive.search` | Search files in Drive |
| `gws.drive.read_file` | Read file content/metadata |
| `gws.drive.list_folder` | List folder contents |

### Skills (Claude Code only)

| Command | Description |
|---------|-------------|
| `/gws:configure` | Interactive setup wizard |
| `/gws:add-account` | Connect a new account |
| `/gws:remove-account` | Disconnect an account |
| `/gws:list-accounts` | Show connected accounts |
| `/gws:set-default` | Change default account |

## Google Cloud Setup

One-time setup (~5 minutes):

1. **Go to [Google Cloud Console](https://console.cloud.google.com/)** and create a new project (e.g., "GWS Connector")

2. **Enable APIs** — click each link and hit "Enable":
   - [Gmail API](https://console.cloud.google.com/apis/library/gmail.googleapis.com)
   - [Calendar API](https://console.cloud.google.com/apis/library/calendar-json.googleapis.com)
   - [Drive API](https://console.cloud.google.com/apis/library/drive.googleapis.com)

3. **Configure [OAuth consent screen](https://console.cloud.google.com/apis/credentials/consent)**:
   - Choose "External" (or "Internal" for Google Workspace orgs)
   - Add required scopes: `gmail.modify`, `calendar`, `drive`, `userinfo.email`, `userinfo.profile`
   - Add your Google email as a test user

4. **Create [OAuth credentials](https://console.cloud.google.com/apis/credentials)**:
   - Click "+ Create Credentials" → "OAuth client ID"
   - Application type: **Desktop app**
   - Copy the Client ID and Client Secret

### Multiple organizations

If you connect accounts from different Google Workspace orgs, each org may need its own GCP project with OAuth credentials. Pass per-account credentials when adding:

```
gws.accounts.add(label: "work", clientId: "org-client-id", clientSecret: "org-secret")
```

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
├── .claude-plugin/plugin.json   # Claude Code plugin manifest
├── .mcp.json                    # Claude Code MCP config
├── skills/                      # Claude Code slash commands
├── hooks/                       # Claude Code session hooks
├── agents/                      # Claude Code workspace agent
│
├── .vscode/mcp.json             # GitHub Copilot MCP config
├── .cursor/mcp.json             # Cursor MCP config
└── codex.json                   # OpenAI Codex CLI config
```

- **Token storage**: OS keychain (macOS Keychain, GNOME Keyring, Windows Credential Manager) with automatic file fallback
- **Account registry**: JSON file at `~/.claude/channels/gws/accounts.json`
- **Credential resolution**: per-account OAuth credentials → global env vars
- **Protocol**: MCP (Model Context Protocol) over stdio — compatible with any MCP client

## Development

```bash
make build          # Build binary
make test           # Run tests (56 tests across 4 packages)
make test-verbose   # Run tests with verbose output
make lint           # Run go vet
make release        # Cross-compile for all platforms
make clean          # Remove build artifacts
```

## License

MIT — see [LICENSE](LICENSE).

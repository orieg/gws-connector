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

- A Google Cloud project with OAuth credentials ([setup guide below](#google-cloud-setup))

### Install

**Option A — Download prebuilt binary (no Go required):**

```bash
# macOS (Apple Silicon)
curl -Lo gws-mcp https://github.com/orieg/gws-connector/releases/latest/download/gws-mcp-darwin-arm64
chmod +x gws-mcp

# macOS (Intel)
curl -Lo gws-mcp https://github.com/orieg/gws-connector/releases/latest/download/gws-mcp-darwin-amd64
chmod +x gws-mcp

# Linux (x86_64)
curl -Lo gws-mcp https://github.com/orieg/gws-connector/releases/latest/download/gws-mcp-linux-amd64
chmod +x gws-mcp

# Linux (ARM64)
curl -Lo gws-mcp https://github.com/orieg/gws-connector/releases/latest/download/gws-mcp-linux-arm64
chmod +x gws-mcp
```

**Option B — Build from source (requires Go 1.25+):**

```bash
git clone https://github.com/orieg/gws-connector
cd gws-connector
make build
# Binary is at ./bin/gws-mcp
```

### Configure credentials

Credentials are provided **per-account** when you connect each account — no environment variables needed. When you create OAuth credentials in GCP, download the `client_secret_*.json` file. The `/gws:configure` skill reads it directly.

You can also pass credentials explicitly:

```
gws.accounts.add(label: "personal", clientId: "your-client-id", clientSecret: "your-secret")
```

Different accounts can use different GCP projects — see [Multiple organizations](#multiple-organizations).

### Install per platform

<details>
<summary><strong>Claude Code (plugin — recommended)</strong></summary>

Installing as a plugin gives you the MCP server **plus** skills, hooks, and agents.

**Option A — Local install (dev/testing):**

```bash
# Clone and build
git clone https://github.com/orieg/gws-connector
cd gws-connector
make build

# Launch Claude Code with the plugin loaded from the cloned repo
claude --plugin-dir ./
```

Use `/reload-plugins` inside the session after making changes.

**Option B — Marketplace:**

```
/plugin install gws-connector
```

Once loaded, run `/gws:configure` for an interactive setup wizard.

</details>

<details>
<summary><strong>Claude Code (MCP server only)</strong></summary>

If you only want the MCP tools without skills/hooks/agents, point Claude at the binary directly. Use a [prebuilt binary](#install) or build from source first, then:

```bash
claude mcp add --transport stdio gws-connector --scope user \
  -- /path/to/gws-mcp --use-dot-names
```

Verify with `claude mcp list` or `/mcp` inside a session.

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
        "command": "/path/to/gws-mcp",
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
      "command": "/path/to/gws-mcp",
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
      "command": "/path/to/gws-mcp",
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
gws-mcp [--use-dot-names]
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

# Or in Claude Code (plugin)
/gws:add-account
```

This opens a browser for Google OAuth. The first account becomes the default. Add more by running the command again with a different label.

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

### Skills (Claude Code plugin only)

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

3. **Configure [OAuth consent screen](https://console.cloud.google.com/auth/overview)**:
   - Under **Branding**, set the app name (e.g., "Claude GWS") and your email
   - Under **Audience**, choose "External" (or "Internal" for Google Workspace orgs)
   - Under **Audience → Test users**, add your Google email address(es)

4. **Add API scopes** under [Data Access](https://console.cloud.google.com/auth/scopes):
   - Click "Add or remove scopes"
   - Add: `gmail.modify`, `calendar`, `drive`, `userinfo.email`, `userinfo.profile`
   - Click "Update", then "Save"

5. **Create [OAuth credentials](https://console.cloud.google.com/apis/credentials)**:
   - Click "+ Create Client" → "OAuth client ID"
   - Application type: **Desktop app**
   - Copy the Client ID and Client Secret

### Multiple organizations

If you connect accounts from different Google Workspace orgs, each org may need its own GCP project with OAuth credentials. Pass per-account credentials when adding:

```
gws.accounts.add(label: "work", clientId: "org-client-id", clientSecret: "org-secret")
```

Accounts that don't specify credentials will use the global `GWS_GOOGLE_CLIENT_ID` / `GWS_GOOGLE_CLIENT_SECRET` environment variables.

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
make test           # Run tests with race detector
make test-verbose   # Run tests with verbose output
make lint           # Run go vet
make release        # Cross-compile for all platforms
make clean          # Remove build artifacts
```

## License

MIT — see [LICENSE](LICENSE).

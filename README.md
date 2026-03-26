# GWS Connector for Claude Code

Multi-account Google Workspace plugin for [Claude Code](https://docs.anthropic.com/en/docs/claude-code) — connect multiple Gmail, Google Calendar, and Google Drive accounts with smart routing.

## Why

Claude Code's built-in Google connectors support a single account. If you use multiple Google accounts (personal + work, multiple clients, different orgs), you need to switch between them manually. This plugin lets you connect them all at once and route requests by label, email, or domain.

## Features

- **Multi-account** — connect unlimited Gmail and Google Workspace accounts
- **Smart routing** — target accounts by label (`work`), email, or domain
- **Per-account OAuth** — different orgs can use their own GCP credentials
- **17 tools** — Mail (search, read, draft, labels), Calendar (list, get, create), Drive (search, read, list)
- **Account management** — add, remove, set default, list accounts
- **Native plugin** — skills, hooks, and MCP tools integrate directly with Claude Code

## Quick Start

### Prerequisites

- Go 1.21+ installed
- A Google Cloud project with OAuth credentials ([setup guide below](#google-cloud-setup))

### Install

```bash
git clone https://github.com/orieg/claude-multi-gws
cd claude-multi-gws
make build
```

Then install as a Claude Code plugin:

```
/plugin install /path/to/claude-multi-gws
```

Or add the marketplace and install from there:

```
/plugin marketplace add orieg/claude-multi-gws
/plugin install gws-connector
```

### Configure

Set your OAuth credentials:

```bash
# Add to ~/.zshrc or ~/.bashrc
export GWS_GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GWS_GOOGLE_CLIENT_SECRET="your-client-secret"
```

Or run the interactive setup wizard inside Claude Code:

```
/gws:configure
```

### Connect accounts

```
/gws:add-account
```

This opens a browser for Google OAuth. You'll be prompted for a label (e.g., `personal`, `work`). The first account becomes the default.

Add more accounts by running the command again. Each account can optionally use different OAuth credentials for different organizations.

## Usage

Once accounts are connected, all `gws.*` tools accept an optional `account` parameter:

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

### Skills (slash commands)

| Command | Description |
|---------|-------------|
| `/gws:configure` | Interactive setup wizard |
| `/gws:add-account` | Connect a new account |
| `/gws:remove-account` | Disconnect an account |
| `/gws:list-accounts` | Show connected accounts |
| `/gws:set-default` | Change default account |

## Google Cloud Setup

One-time setup (~5 minutes):

1. **Go to [Google Cloud Console](https://console.cloud.google.com/)** and create a new project (e.g., "Claude GWS Connector")

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
claude-multi-gws/
├── .claude-plugin/plugin.json   # Plugin manifest
├── .mcp.json                    # MCP server config
├── skills/                      # Slash command definitions
├── hooks/                       # Session start hook
├── agents/                      # Workspace context agent
├── cmd/gws-mcp/                 # MCP server entrypoint
└── internal/
    ├── accounts/                # Account registry & router
    ├── auth/                    # OAuth flow, token store, client factory
    ├── server/                  # MCP tool registration & dispatch
    └── services/                # Gmail, Calendar, Drive API wrappers
```

- **Token storage**: OS keychain (macOS Keychain, GNOME Keyring, Windows Credential Manager) with automatic file fallback
- **Account registry**: JSON file at `~/.claude/channels/gws/accounts.json`
- **Credential resolution**: per-account OAuth credentials → global env vars

## Development

```bash
make build          # Build binary
make test           # Run tests
make test-verbose   # Run tests with verbose output
make lint           # Run go vet
make release        # Cross-compile for all platforms
make clean          # Remove build artifacts
```

### Running tests

```bash
make test
# 56 tests across 4 packages: accounts, auth, server, services
```

## Coexistence with built-in connectors

This plugin works alongside Claude Code's built-in Gmail and Calendar connectors. Tool names don't conflict (`gws.mail.*` vs `gmail_*`). You can keep both or disconnect the built-in ones for a cleaner experience.

## License

MIT — see [LICENSE](LICENSE).

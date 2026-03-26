# Multi-Account Google Workspace Plugin for Claude Code / Cowork

> A Claude Code plugin that connects to multiple Google Workspace accounts (Gmail, Calendar, Drive) simultaneously with smart account routing.

---

## Table of Contents

1. [Problem Statement](#problem-statement)
2. [Research Findings](#research-findings)
3. [Architecture Decisions](#architecture-decisions)
4. [Design](#design)
5. [Implementation Plan](#implementation-plan)
6. [Verification](#verification)
7. [Open Questions](#open-questions)
8. [References](#references)

---

## Problem Statement

Claude Code currently connects to Google Calendar and Gmail via **single-account cloud-hosted OAuth MCP servers** managed by Claude.ai. This creates three limitations:

1. **Single account only** — no way to connect both a personal Gmail and a work Google Workspace account
2. **No Google Drive** — Drive is not available as a connected service
3. **No account routing** — Claude cannot distinguish between work and personal contexts

### Goal

Build a Claude Code / Cowork plugin that:
- Connects to **2+ Google accounts** (personal Gmail, work GWS, client accounts)
- Covers **Gmail, Calendar, and Drive**
- Routes requests to the correct account automatically or via user instruction
- Requires **near-zero setup** (OAuth-based, no manual token management)
- Integrates **natively** with Claude Code CLI and Claude Cowork web

---

## Research Findings

### How Google services connect today

| Aspect | Current State |
|--------|--------------|
| **Transport** | Cloud-hosted SSE MCP servers (`claude.ai Google Calendar`, `claude.ai Gmail`) |
| **Auth** | Claude Code handles full OAuth flow, encrypts tokens, auto-refreshes |
| **Config** | Tracked in `~/.claude.json` under `claudeAiMcpEverConnected` |
| **Accounts** | One account per service, no multi-account support |
| **Drive** | Not connected |
| **Token access** | Encrypted by Claude Code, not accessible to plugins |

### Gemini Google Workspace extension (reference implementation)

Located at `~/.gemini/extensions/google-workspace/`:
- **Type:** Local stdio MCP server (`node dist/index.js --use-dot-names`)
- **Multi-account aware:** Tracks accounts in `google_accounts.json` with `active` and `old` arrays
- **Services:** Gmail, Calendar, Drive, Docs, Sheets, Slides, Chat
- **Behavioral guide:** `WORKSPACE-Context.md` with patterns for session init, timezone detection, search, scheduling
- **Key insight:** Local server pattern is proven for Google Workspace MCP integration

### Claude Code plugin architecture

- **Manifest:** `.claude-plugin/plugin.json` (name, version, description, author, keywords)
- **Components:** Skills, agents, hooks, MCP servers
- **MCP server types:** `stdio` (local process), `sse` (cloud OAuth), `http` (REST + bearer)
- **Auth patterns:**
  - OAuth (automatic via Claude Code for SSE/HTTP)
  - Bearer tokens (env vars)
  - Dynamic headers (helper scripts via `headersHelper`)
- **Multi-tenancy:** Already supported via `X-Workspace-ID` / `X-Tenant-ID` headers
- **Plugin marketplace:** `~/.claude/plugins/marketplaces/`
- **State convention:** `~/.claude/channels/{service}/` for per-service state (used by Discord, Telegram)

### MCP auth tiers (spec 2025-11-25)

| Tier | Method | When to use |
|------|--------|-------------|
| 1 | Static API key from env | Simplest, sufficient if no OAuth needed |
| 2 | OAuth via CIMD | **Preferred** — server publishes client metadata URL, no registration endpoint |
| 3 | OAuth via DCR | Backward compat fallback for older hosts |

For this plugin, we use **local OAuth** (localhost redirect) since we need to manage multiple accounts ourselves, not delegate to Claude Code's built-in single-account flow.

### Token storage best practices

| Deployment | Store in |
|-----------|---------|
| Local / MCPB | OS keychain (`keytar` on Node, `keyring` on Python) — **never plaintext** |
| Remote, stateless | Nowhere — host sends bearer each request |
| Remote, stateful | Session store (Redis, etc.) |

### Multi-account patterns observed in the ecosystem

- **Discord/Telegram plugins:** Pairing codes, allowlists, config in `~/.claude/channels/`, reloaded per-request
- **Gemini extension:** `google_accounts.json` with active + old account arrays
- **MCP multi-tenancy:** Per-request workspace/tenant selection via headers or URL path

---

## Architecture Decisions

### Decision 1: Local stdio MCP server — Go binary

**Choice:** Local stdio server compiled as a **Go binary**

**Rationale:**
- Claude Code's built-in SSE/OAuth connectors support exactly 1 account per service with no plugin token access — can't extend them
- A local server owns the full token lifecycle: multiple OAuth flows, keychain storage, per-request account routing
- Gemini's extension uses the same pattern successfully
- Works identically in Claude Code CLI and Cowork
- **Go binary advantage over TypeScript:** Single binary distribution, zero runtime dependency (no Bun/Node install needed), ~10ms startup (important for stdio servers spawned per-session), trivial cross-compilation (`GOOS=darwin GOARCH=arm64 go build`)
- Mature Go libraries: `google.golang.org/api` (Google APIs), `golang.org/x/oauth2` (OAuth), `github.com/zalando/go-keyring` (OS keychain), `github.com/mark3labs/mcp-go` or official `github.com/modelcontextprotocol/go-sdk` (MCP protocol)

**Tradeoffs:**
- Non-standard for Claude Code plugins (most are TypeScript) — but the stdio MCP server is a standalone process, it doesn't share code with the plugin ecosystem
- OAuth localhost redirect is fragile in headless/remote environments (mitigated with encrypted file fallback)

### Decision 2: Per-account OAuth2 with localhost callback

**Choice:** Each account gets its own OAuth2 authorization flow via localhost redirect

**Rationale:**
- Google requires separate consent per account
- Localhost redirect is the standard pattern for desktop/CLI OAuth
- The MCP SDK includes a localhost-redirect helper
- `access_type=offline` + `prompt=consent` ensures we get a refresh token

**Flow:**
1. User runs `/gws:add-account <label>`
2. Plugin starts localhost HTTP callback server on a random high port
3. Browser opens to Google OAuth consent screen
4. After consent, callback captures auth code, exchanges for tokens
5. Tokens stored in keychain; account added to registry

### Decision 3: OS keychain for token storage

**Choice:** OS keychain (primary) with encrypted file fallback

**Rationale:**
- Keychain is the MCP-recommended approach for local servers
- Never plaintext on disk
- Per-account keying: `claude-gws-plugin:{email}`
- Encrypted file fallback (`~/.claude/channels/gws/tokens.enc`) for headless Linux/CI

### Decision 4: Dot-namespaced tools with optional `account` param

**Choice:** `google.<service>.<action>` naming with `--use-dot-names`, every tool has optional `account` parameter

**Rationale:**
- `gws.*` namespacing avoids collisions with existing cloud MCP tools (`gmail_search_messages` vs `gws.mail.search`) and avoids trademark issues with "Google"
- Optional `account` param keeps the API clean for single-account users while supporting multi-account
- Account accepts email address or label string
- Dot-name convention via `--use-dot-names` flag (matches Gemini extension)

### Decision 5: User-provided Google Cloud client_id (no shipped default)

**Choice:** Users provide their own GCP OAuth client_id via env vars. No default shipped.

**Rationale:**
- Avoids the burden of maintaining a GCP project, handling Google app verification (weeks-long review process for sensitive scopes like `gmail.modify`), and managing shared quota
- Creating a GCP project is a one-time ~5 minute setup (documented in README)
- For "installed app" (desktop/CLI) OAuth clients, Google doesn't rely on `client_secret` being secret — PKCE flow is used
- Enterprise users already need their own org credentials anyway
- **Future:** Once the plugin has traction, we can create a verified GCP project and ship a default client_id as an upgrade

**Setup:**
```bash
export GWS_GOOGLE_CLIENT_ID="your-client-id.apps.googleusercontent.com"
export GWS_GOOGLE_CLIENT_SECRET="your-client-secret"
```

### Decision 6: Account routing with labels and domain rules

**Choice:** Multi-level account resolution: explicit > domain rules > default > disambiguation

**Rationale:**
- Covers all interaction patterns: explicit ("use my work account"), contextual ("reply to john@company.com"), and implicit (default)
- Domain routing rules in `accounts.json` enable zero-effort context switching
- Disambiguation fallback prevents silent wrong-account errors

---

## Design

### Account Registry

**File:** `~/.claude/channels/gws/accounts.json`

```json
{
  "accounts": [
    {
      "email": "nicolas@brousse.info",
      "label": "personal",
      "displayName": "Nico",
      "addedAt": "2026-03-26T10:00:00Z",
      "services": ["gmail", "calendar", "drive"],
      "default": true
    },
    {
      "email": "nicolas@company.com",
      "label": "work",
      "displayName": "Nicolas B.",
      "addedAt": "2026-03-26T10:05:00Z",
      "services": ["gmail", "calendar", "drive"],
      "default": false
    }
  ],
  "routingRules": {
    "domains": {
      "company.com": "nicolas@company.com",
      "brousse.info": "nicolas@brousse.info"
    }
  }
}
```

**Properties:**
- `label` — user-assigned, used in natural language ("use my work account")
- `default` — determines which account is used when none specified
- `routingRules.domains` — maps email domain patterns to accounts for auto-routing
- Re-read on every tool call (skill-driven changes take effect immediately)

### Account Resolution Priority

```
1. Explicit `account` param → match by email or label
2. Domain routing rules → infer from email domain in context
3. Default account → `defaultAccount` from registry
4. Disambiguation error → Claude asks the user which account
```

### Tool Inventory

#### Gmail tools

| Tool | Key Parameters | Description |
|------|---------------|-------------|
| `gws.mail.search` | `query`, `maxResults?`, `account?` | Search messages |
| `gws.mail.read_message` | `messageId`, `account?` | Read full message |
| `gws.mail.read_thread` | `threadId`, `account?` | Read full thread |
| `gws.mail.create_draft` | `to`, `subject`, `body`, `cc?`, `bcc?`, `threadId?`, `account?` | Create draft |
| `gws.mail.send_draft` | `draftId`, `account?` | Send existing draft |
| `gws.mail.list_labels` | `account?` | List labels |
| `gws.mail.modify_labels` | `messageId`, `addLabelIds?`, `removeLabelIds?`, `account?` | Modify labels |
| `gws.mail.get_profile` | `account?` | Get profile info |
| `gws.mail.download_attachment` | `messageId`, `attachmentId`, `localPath`, `account?` | Download attachment |

#### Calendar tools

| Tool | Key Parameters | Description |
|------|---------------|-------------|
| `gws.cal.list_events` | `timeMin`, `timeMax`, `calendarId?`, `q?`, `account?` | List events |
| `gws.cal.get_event` | `eventId`, `calendarId?`, `account?` | Get event details |
| `gws.cal.create_event` | `event`, `calendarId?`, `sendUpdates?`, `account?` | Create event |
| `gws.cal.update_event` | `eventId`, `event`, `calendarId?`, `account?` | Update event |
| `gws.cal.delete_event` | `eventId`, `calendarId?`, `account?` | Delete event |
| `gws.cal.find_free_time` | `calendarIds`, `timeMin`, `timeMax`, `account?` | Find free slots |
| `gws.cal.find_meeting_times` | `attendees`, `duration`, `timeMin`, `timeMax`, `account?` | Find mutual availability |
| `gws.cal.list_calendars` | `account?` | List calendars |
| `gws.cal.respond_to_event` | `eventId`, `response`, `comment?`, `account?` | RSVP |

#### Drive tools

| Tool | Key Parameters | Description |
|------|---------------|-------------|
| `gws.drive.search` | `query`, `maxResults?`, `account?` | Search files |
| `gws.drive.get_file` | `fileId`, `account?` | Get file metadata |
| `gws.drive.read_file` | `fileId`, `account?` | Read file contents |
| `gws.drive.create_file` | `name`, `content`, `mimeType?`, `parentFolderId?`, `account?` | Create file |
| `gws.drive.update_file` | `fileId`, `content?`, `name?`, `account?` | Update file |
| `gws.drive.list_folder` | `folderId?`, `account?` | List folder contents |
| `gws.drive.download_file` | `fileId`, `localPath`, `account?` | Download to disk |

#### Meta tools

| Tool | Parameters | Description |
|------|-----------|-------------|
| `gws.accounts.list` | none | List all connected accounts |
| `gws.accounts.get_profile` | `account?` | Get user profile for an account |

### Plugin Directory Structure

```
gws-connector/
├── .claude-plugin/
│   └── plugin.json                # Plugin manifest
├── .mcp.json                      # stdio MCP server configuration
├── go.mod                         # Go module definition
├── go.sum
├── Makefile                       # Build targets: build, install, test, release
├── cmd/
│   └── gws-mcp/
│       └── main.go                # MCP server entry point
├── internal/
│   ├── server/
│   │   └── server.go              # MCP server setup, tool registration and dispatch
│   ├── auth/
│   │   ├── oauth.go               # OAuth2 flow with localhost callback
│   │   ├── tokenstore.go          # OS keychain + encrypted file fallback
│   │   └── client.go              # Authenticated Google API client factory with auto-refresh
│   ├── accounts/
│   │   ├── registry.go            # accounts.json CRUD
│   │   ├── router.go              # Account resolution (label, email, domain, default)
│   │   └── types.go               # Go types
│   ├── services/
│   │   ├── gmail.go               # Gmail tool implementations
│   │   ├── calendar.go            # Calendar tool implementations
│   │   └── drive.go               # Drive tool implementations
│   └── utils/
│       ├── errors.go              # Error types and formatting
│       └── pagination.go          # Pagination helpers
├── bin/                           # Pre-built binaries (gitignored, populated by Makefile)
│   └── gws-mcp                    # Compiled binary for current platform
├── skills/
│   ├── add-account/SKILL.md       # /gws:add-account <label>
│   ├── remove-account/SKILL.md    # /gws:remove-account <label>
│   ├── list-accounts/SKILL.md     # /gws:accounts
│   ├── set-default/SKILL.md       # /gws:default <label>
│   └── configure/SKILL.md         # /gws:configure (first-time setup wizard)
├── agents/
│   └── workspace-context.md       # Behavioral guide for multi-account handling
├── hooks/
│   └── hooks.json                 # SessionStart: inject account context
├── scripts/
│   └── session-init.sh            # Outputs account summary at session start
├── README.md
└── LICENSE
```

### Go Dependencies

```
google.golang.org/api              # Google APIs (Gmail, Calendar, Drive)
golang.org/x/oauth2                # OAuth2 client
github.com/zalando/go-keyring      # Cross-platform OS keychain
github.com/mark3labs/mcp-go        # MCP protocol SDK (or official go-sdk)
```

### MCP Server Configuration (.mcp.json)

```json
{
  "gws-connector": {
    "command": "${CLAUDE_PLUGIN_ROOT}/bin/gws-mcp",
    "args": ["--use-dot-names"],
    "env": {
      "GWS_STATE_DIR": "${HOME}/.claude/channels/gws",
      "GWS_GOOGLE_CLIENT_ID": "${GWS_GOOGLE_CLIENT_ID}",
      "GWS_GOOGLE_CLIENT_SECRET": "${GWS_GOOGLE_CLIENT_SECRET}"
    }
  }
}
```

> The binary is pre-compiled via `make build` (or downloaded from GitHub releases). No runtime dependency needed.

### Plugin Manifest (.claude-plugin/plugin.json)

```json
{
  "name": "gws-connector",
  "version": "0.1.0",
  "description": "Multi-account workspace integration — Mail, Calendar, and Drive with smart account routing for Gmail and Google Workspace",
  "author": {
    "name": "orieg",
    "url": "https://github.com/orieg"
  },
  "repository": "https://github.com/orieg/claude-multi-gws",
  "license": "MIT",
  "keywords": [
    "gws", "mail", "calendar", "drive", "workspace",
    "multi-account", "oauth", "mcp"
  ]
}
```

### Account Management UX

#### First-time setup: `/gws:configure`
1. Detects no accounts configured
2. Guides through first OAuth flow
3. Labels account interactively ("What label? e.g., personal, work")
4. Sets as default
5. Offers to add more accounts
6. If cloud MCPs detected, explains coexistence

#### Adding: `/gws:add-account <label>`
```
> /gws:add-account work
Opening browser for Google sign-in...
[User authorizes in browser]
Added work account (nicolas@company.com). You now have 2 accounts connected.
```

#### Listing: `/gws:accounts`
```
Google Workspace accounts:
1. personal (nicolas@brousse.info) [DEFAULT]
   Services: Gmail, Calendar, Drive
2. work (nicolas@company.com)
   Services: Gmail, Calendar, Drive
```

#### Removing: `/gws:remove-account <label>`
Confirms with user, deletes tokens from keychain, removes from registry. If removed account was default, remaining account becomes default.

### Smart Account Selection by Claude

The `agents/workspace-context.md` behavioral guide instructs Claude to:
- Parse user intent for account hints ("my work calendar", "personal email")
- Use domain routing rules for contact-based inference ("reply to john@company.com" → work account)
- Fall back to default when no signal
- Ask for disambiguation when ambiguous
- Always mention which account was used in responses

### Coexistence with Built-in Cloud Connectors

| Aspect | Cloud MCP | This Plugin |
|--------|-----------|-------------|
| Tool names | `gmail_search_messages`, `gcal_list_events` | `gws.mail.search`, `gws.cal.list_events` (no trademark conflict) |
| Accounts | 1 per service | N per service |
| Drive | No | Yes |
| Account routing | N/A | Label, domain, default |

No name collisions. Both can coexist. The behavioral guide tells Claude to prefer the plugin when multi-account is relevant.

### Session Context via Hook

**hooks/hooks.json:**
```json
{
  "hooks": {
    "SessionStart": [
      {
        "hooks": [
          {
            "type": "command",
            "command": "bash ${CLAUDE_PLUGIN_ROOT}/scripts/session-init.sh",
            "timeout": 10
          }
        ]
      }
    ]
  }
}
```

The script reads `accounts.json` and outputs:
```
Google Workspace: 2 accounts connected
  - personal (nicolas@brousse.info) [DEFAULT]
  - work (nicolas@company.com)
Use account labels or emails to target a specific account.
```

---

## Implementation Plan

### Phase 1: Core Infrastructure

**Goal:** Plugin skeleton with multi-account OAuth and token management

**Files:**
- `.claude-plugin/plugin.json`, `.mcp.json`, `go.mod`, `Makefile`
- `cmd/gws-mcp/main.go` — MCP server entry point (stdio transport)
- `internal/server/server.go` — Tool registration scaffold
- `internal/auth/oauth.go` — OAuth2 flow with localhost callback server
- `internal/auth/tokenstore.go` — OS keychain storage with encrypted file fallback
- `internal/auth/client.go` — Authenticated Google API client factory with auto-refresh
- `internal/accounts/registry.go` — accounts.json CRUD
- `internal/accounts/router.go` — Account resolution logic
- `internal/accounts/types.go` — Go types
- `skills/configure/SKILL.md` — First-time setup wizard
- `skills/add-account/SKILL.md` — Add account command
- `skills/remove-account/SKILL.md` — Remove account command
- `skills/list-accounts/SKILL.md` — List accounts command
- `gws.accounts.list` meta tool

**Deliverable:** Can authenticate 2+ Google accounts, store tokens securely, list accounts. Binary builds with `make build`.

### Phase 2: Mail + Calendar

**Goal:** Full Gmail and Calendar tools with multi-account support

**Files:**
- `internal/services/gmail.go` — All mail tools
- `internal/services/calendar.go` — All calendar tools
- `agents/workspace-context.md` — Behavioral guide for Claude
- `hooks/hooks.json` + `scripts/session-init.sh` — Session context injection

**Deliverable:** Search, read, draft emails; list, create, update events — all with account selection.

### Phase 3: Drive

**Goal:** Full Drive integration

**Files:**
- `internal/services/drive.go` — All Drive tools

**Deliverable:** Search, read, create, download files across accounts.

### Phase 4: Polish

**Goal:** Production readiness

**Features:**
- Smart domain-based account routing
- Token health check + re-auth prompting
- Caching (timezone, profile, label list)
- Rate limiting and retry logic
- Comprehensive README with setup walkthrough (including GCP project creation)
- Cloud MCP coexistence guidance in `/gws:configure`
- GitHub releases with pre-built binaries for macOS (arm64/amd64) and Linux

---

## Verification

### Multi-account OAuth
- [ ] Add personal account, verify token in keychain, verify `accounts.json`
- [ ] Add work account, verify both accounts in registry
- [ ] Force token expiry, verify auto-refresh works
- [ ] Revoke access in Google Security settings, verify re-auth prompt

### Tool routing
- [ ] Explicit `account` param routes correctly
- [ ] Label-based routing works ("work", "personal")
- [ ] Default account used when no `account` param
- [ ] Domain routing rules infer correct account
- [ ] Ambiguous context triggers disambiguation

### Cross-platform
- [ ] Claude Code CLI: install plugin, add accounts, test all tools
- [ ] Claude Cowork web: verify plugin loads, OAuth works, tools available
- [ ] `claude --mcp-debug`: verify connection, tool discovery, auth in logs

### Coexistence
- [ ] With `claude.ai Gmail` + `claude.ai Calendar` also connected, verify no tool name collisions
- [ ] Claude correctly distinguishes between cloud and plugin tools

---

## Resolved Decisions

| Question | Decision | Rationale |
|----------|----------|-----------|
| OAuth client_id | User-provided (no default shipped) | Avoids GCP project maintenance, app verification overhead. Ship a default later once verified. |
| Runtime | Go binary | Single binary, zero dependency, fast startup (~10ms), trivial cross-compilation |
| Scopes | All upfront (mail + calendar + drive) | One consent screen per account. Simpler UX. |
| Cross-account | Per-account only (no fan-out) | Simpler. Claude makes separate calls per account when needed. |
| Namespace | `gws.*` | Avoids trademark issues with "Google". Short, recognizable. |

## Open Questions

1. **Go MCP SDK** — `github.com/mark3labs/mcp-go` (popular community) vs `github.com/modelcontextprotocol/go-sdk` (official) — need to evaluate maturity and stdio support
2. **Plugin distribution** — Target the Claude Code marketplace, or start as manual git-clone install + GitHub releases?
3. **Keychain library** — `github.com/zalando/go-keyring` vs `github.com/99designs/keyring` — need to evaluate macOS Keychain and Linux secret-service support
4. **Build/install UX** — Should `make install` copy the binary into the plugin dir, or should we use `go install` and reference the binary from PATH?

---

## References

### Key files on this system

| File | Purpose |
|------|---------|
| `~/.claude/plugins/marketplaces/claude-plugins-official/external_plugins/discord/` | Reference stdio MCP server plugin (state in `~/.claude/channels/`, skill-driven config) |
| `~/.gemini/extensions/google-workspace/` | Google Workspace MCP server reference, `accounts.json` schema |
| `~/.gemini/extensions/google-workspace/WORKSPACE-Context.md` | Behavioral guide template to adapt |
| `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/plugin-dev/skills/mcp-integration/` | MCP integration patterns, auth, tool design |
| `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/plugin-dev/skills/plugin-structure/` | Plugin manifest, component patterns |
| `~/.claude/plugins/marketplaces/claude-plugins-official/plugins/mcp-server-dev/skills/build-mcp-server/references/auth.md` | MCP auth tiers (CIMD, DCR, keychain) |

### MCP specification

- MCP spec 2025-11-25: CIMD promoted to SHOULD, DCR demoted to MAY
- Token audience validation: RFC 8707 (MUST validate token was minted for this server)
- SDK helpers: `@modelcontextprotocol/sdk/server/auth` (mcpAuthRouter, bearerAuth, proxyProvider)

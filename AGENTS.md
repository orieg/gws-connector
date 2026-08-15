# AGENTS.md

Guidance for AI coding agents (and humans) working in this repository. This
follows the [agents.md](https://agents.md) convention and complements
[`CONTRIBUTING.md`](CONTRIBUTING.md).

> Note: [`CONTEXT.md`](CONTEXT.md) is different — it is the *runtime* behavioral
> guide loaded by MCP clients that describes how to **use** the `gws.*` tools.
> This file describes how to **develop** the project.

## What this project is

A multi-account Google Workspace MCP server in Go. It exposes Gmail, Calendar,
Drive, Sheets, and Docs as MCP tools with per-account OAuth and label/email/domain
routing. It ships as a Claude Code plugin, a Gemini CLI extension, and a plain
MCP server over stdio.

## Build, test, lint

```bash
make build          # build ./bin/gws-mcp
make test           # go test with -race (authoritative check)
make lint           # go vet
gofmt -w <files>    # ALWAYS gofmt Go files you edit
```

Always run `make test` and `make lint` before finishing a change. CI additionally
runs `govulncheck`; keep dependencies patched.

## Codebase map

| Path | Responsibility |
|------|----------------|
| `cmd/gws-mcp/` | Entrypoint; env vars; starts MCP server on stdio |
| `internal/server/server.go` | **Tool registration & dispatch** — start here to add/change a tool |
| `internal/services/` | Gmail, Calendar, Drive, Sheets, Docs API handlers |
| `internal/auth/` | OAuth flows, token store (OS keychain + file fallback), client factory |
| `internal/accounts/` | Account registry (JSON) and routing resolution |
| `skills/` | Slash-command workflows (Claude Code + Gemini CLI) |
| `hooks/`, `agents/` | Claude Code session hooks and workspace-context agent |
| `docs/` | GitHub Pages landing page + privacy policy |

## Conventions

- **Tool naming**: `s.toolName("gws", "<group>", "<verb>")`. The `--use-dot-names`
  flag switches between `gws.mail.search` and `gws_mail_search` — never hardcode a
  separator.
- **Tool annotations** (required on every new tool):
  - Read-only (search/read/list/get): `mcp.WithReadOnlyHintAnnotation(true)`
  - Overwrites or deletes data (remove, `sheets.write_range`, `docs.replace_text`):
    `mcp.WithDestructiveHintAnnotation(true)`
  - Additive writes (create/insert draft/event/label/doc): omit both hints.
- **Account parameter**: every service tool takes an optional `account` param
  (`accountParam` in each `register*Tools`). Preserve this.
- **Security**: never log tokens or put them in error messages; state files are
  `0600`, dirs `0700`; validate params used in Google API queries.
- **Commits**: Conventional Commits (`feat(scope): …`, `fix(scope): …`). Atomic.
- **Docs stay in sync**: adding/changing a tool means updating the README tool
  table and, if it changes runtime behavior, `CONTEXT.md`.

## Adding a tool (end to end)

1. Register it in the right `register*Tools` func in `internal/server/server.go`
   with a description, params, and the correct annotation.
2. Implement the handler in the matching `internal/services/*.go`.
3. Add/extend a test under `internal/server/`.
4. Update the README tool table (and `CONTEXT.md` if behavior-relevant).
5. `gofmt -w`, then `make test && make lint`.

## Gotchas

- `internal/server/server.go` is large; make surgical edits and re-run `gofmt`.
- Some diagnostics in that file are pre-existing (a deprecated migration field,
  `WriteString(fmt.Sprintf(...))` patterns) — don't treat them as regressions
  from your change unless you touched those lines.
- Tests hit the keychain/registry via temp dirs; don't point them at a real
  `~/.claude` state directory.
- Do not commit real Client IDs, secrets, or tokens.

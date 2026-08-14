# Contributing to GWS Connector

Thanks for your interest in contributing! This project is a multi-account Google
Workspace MCP server written in Go, distributed as a Claude Code plugin, a Gemini
CLI extension, and a standalone MCP server.

## Ways to contribute

- **Report bugs** — open an [issue](https://github.com/orieg/gws-connector/issues/new/choose)
  with steps to reproduce, your client (Claude Code, Gemini CLI, Cursor, …), and
  the connector version.
- **Request features or new tools** — describe the Google Workspace workflow you
  want to automate.
- **Improve docs and recipes** — setup walkthroughs, example prompts, and usage
  recipes are high-value and easy to review.
- **Submit code** — see the workflow below.

## Development setup

Requirements: Go (see the version in [`go.mod`](go.mod)) and `make`.

```bash
git clone https://github.com/orieg/gws-connector
cd gws-connector
make build      # builds ./bin/gws-mcp
make test       # runs tests with the race detector
make lint       # go vet
```

To test the plugin live in Claude Code:

```bash
claude --plugin-dir ./
```

Use `/reload-plugins` after changes. See the "Local development / testing"
section of the [README](README.md) for more.

## Architecture

```
cmd/gws-mcp/          # MCP server entrypoint
internal/accounts/    # Account registry & label/email/domain routing
internal/auth/        # OAuth flow, token store (keychain), client factory
internal/server/      # MCP tool registration & dispatch
internal/services/    # Gmail, Calendar, Drive, Sheets, Docs API wrappers
```

New tools are registered in [`internal/server/server.go`](internal/server/server.go).
See [`AGENTS.md`](AGENTS.md) for a task-oriented map of the codebase (useful for
both humans and AI coding agents).

## Pull request workflow

1. **Fork** and create a branch: `feat/short-description` or `fix/short-description`.
2. **Make focused, atomic commits.** Use Conventional Commit messages:
   `type(scope): description` where type is one of
   `feat`, `fix`, `docs`, `refactor`, `chore`, `ci`, `test`.
3. **Run `make test` and `make lint`** and make sure they pass.
4. **Update docs** — if you add or change a tool, update the tool table in the
   [README](README.md) and, if relevant, [`CONTEXT.md`](CONTEXT.md).
5. **Open a PR** against `main` using the
   [pull request template](.github/pull_request_template.md). Describe the change
   and how you tested it. CI runs build, tests, and `govulncheck`.

## Adding a new tool — checklist

- Register it in the appropriate `register*Tools` function in
  `internal/server/server.go`.
- Add the correct annotation:
  `mcp.WithReadOnlyHintAnnotation(true)` for read-only tools, or
  `mcp.WithDestructiveHintAnnotation(true)` for tools that overwrite or delete
  data. Omit both for additive-write tools (create/insert).
- Implement the handler in the relevant `internal/services/` file.
- Add or extend a test in `internal/server/`.
- Document it in the README tool table and, for the agent behavioral guide,
  `CONTEXT.md`.

## Security

Never log tokens or include them in error messages. State files use `0600`
permissions, directories `0700`. Validate parameters passed into Google API
queries. To report a vulnerability, see [`SECURITY.md`](SECURITY.md) — please do
not open a public issue for security problems.

## License

By contributing, you agree that your contributions are licensed under the
project's [MIT License](LICENSE).

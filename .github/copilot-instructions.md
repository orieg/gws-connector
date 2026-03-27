# Copilot Code Review Instructions

## Project Overview

gws-connector is a multi-account Google Workspace MCP server (Go). It connects Gmail, Calendar, and Drive to Claude Code and other MCP clients via JSON-RPC over stdio.

## Architecture

- `cmd/gws-mcp/` — Entry point, reads env vars, starts MCP server on stdio
- `internal/accounts/` — Account registry (JSON file) and routing (label/email resolution)
- `internal/auth/` — OAuth2 flows, token storage (OS keychain with file fallback), API client factory
- `internal/services/` — Gmail, Calendar, Drive tool handlers
- `internal/server/` — MCP tool registration and dispatch

## Review Focus Areas

### Security (high priority)
- OAuth token handling: tokens must never be logged or included in error messages
- File permissions: state files must use 0600, directories 0700
- Input sanitization: parameters used in Google API queries (especially Drive `folderId`) must be validated
- No hardcoded credentials — all secrets come from env vars or OS keychain
- HTML output in OAuth callback must use `html.EscapeString()`, not manual replacement

### Correctness
- Account routing: label/email matching must be case-insensitive (`strings.EqualFold`)
- Error handling: all errors from external calls must be wrapped with context using `%w`
- Token refresh: persisted tokens must be saved after refresh; save errors must be logged

### Go idioms
- Use `fmt.Errorf("context: %w", err)` for error wrapping
- Prefer early returns over deep nesting
- No exported symbols unless needed by another package
- Tests use `t.TempDir()` for filesystem isolation

### Out of scope
- Performance optimization (single-user MCP server, not a web service)
- Concurrency patterns beyond the OAuth callback goroutine

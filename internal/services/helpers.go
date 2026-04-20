package services

import (
	"encoding/json"
	"fmt"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// TextResult creates a successful text result.
func TextResult(text string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: text,
			},
		},
	}
}

// ErrorResult creates an error result.
func ErrorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			mcp.TextContent{
				Type: "text",
				Text: err.Error(),
			},
		},
	}
}

// TextAndJSONResult returns a tool result containing a primary human-readable
// summary and a second content block carrying the raw structured payload as
// pretty-printed JSON.
//
// Sheets and Docs handlers use this so agents that want structured data can
// parse the JSON block while the text block continues to give a useful
// one-glance summary in chat traces. If json.Marshal fails (it shouldn't on
// well-typed payloads), the failure is appended to the text block and the
// JSON block is omitted — we don't let a render error swallow the tool
// call's actual result.
func TextAndJSONResult(summary string, payload any) *mcp.CallToolResult {
	content := []mcp.Content{mcp.TextContent{Type: "text", Text: summary}}
	if payload != nil {
		b, err := json.MarshalIndent(payload, "", "  ")
		if err == nil {
			content = append(content, mcp.TextContent{Type: "text", Text: string(b)})
		} else {
			content = append(content, mcp.TextContent{
				Type: "text",
				Text: fmt.Sprintf("[structured payload render error: %v]", err),
			})
		}
	}
	return &mcp.CallToolResult{Content: content}
}

// scopeOrErr wraps a failing Google API error with handler-specific context
// unless the underlying failure is an insufficient-scope 403 — in which
// case the returned error is a *auth.ScopeError that tells the agent which
// reauth tool to call.
//
// Usage pattern at any handler error site:
//
//	if err != nil {
//	    return ErrorResult(scopeOrErr(acct, "Gmail", err,
//	        "searching mail on %s: %w", acct.Label, err)), nil
//	}
//
// operation is the API-family noun ("Gmail", "Sheets", "Docs", ...).
// format + args follow fmt.Errorf semantics and should include %w for err.
func scopeOrErr(acct *accounts.Account, operation string, err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	if wrapped := auth.CheckScopeError(err, acct.Label, acct.Email, operation); wrapped != err {
		return wrapped
	}
	return fmt.Errorf(format, args...)
}

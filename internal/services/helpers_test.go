package services

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/api/googleapi"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

func firstText(res *mcp.CallToolResult) string {
	if res == nil || len(res.Content) == 0 {
		return ""
	}
	tc, _ := res.Content[0].(mcp.TextContent)
	return tc.Text
}

func TestTextResult(t *testing.T) {
	res := TextResult("hello")
	if res.IsError {
		t.Error("TextResult must not be an error")
	}
	if firstText(res) != "hello" {
		t.Errorf("text mismatch: %q", firstText(res))
	}
}

func TestErrorResult(t *testing.T) {
	res := ErrorResult(errors.New("boom"))
	if !res.IsError {
		t.Error("ErrorResult must set IsError=true")
	}
	if firstText(res) != "boom" {
		t.Errorf("text mismatch: %q", firstText(res))
	}
}

func TestTextAndJSONResult_NilPayloadIsTextOnly(t *testing.T) {
	res := TextAndJSONResult("summary", nil)
	if len(res.Content) != 1 {
		t.Errorf("nil payload should yield 1 content block, got %d", len(res.Content))
	}
}

func TestTextAndJSONResult_PrettyJSON(t *testing.T) {
	type row struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}
	res := TextAndJSONResult("ok", row{Name: "alice", Age: 30})
	if len(res.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(res.Content))
	}
	text := firstText(res)
	if text != "ok" {
		t.Errorf("first block text mismatch: %q", text)
	}
	second, ok := res.Content[1].(mcp.TextContent)
	if !ok {
		t.Fatalf("second block is not TextContent: %T", res.Content[1])
	}
	// Must be valid JSON that round-trips.
	var got map[string]any
	if err := json.Unmarshal([]byte(second.Text), &got); err != nil {
		t.Errorf("second block is not valid JSON: %v\n%s", err, second.Text)
	}
	// Indent should produce newlines.
	if !strings.Contains(second.Text, "\n") {
		t.Errorf("expected pretty-printed JSON with newlines, got %q", second.Text)
	}
}

// Payloads that cannot be marshaled (e.g., contain a channel) must not
// swallow the tool call — they fall back to an error note in a content
// block.
func TestTextAndJSONResult_MarshalFailureDoesntDropResult(t *testing.T) {
	bad := map[string]any{"ch": make(chan int)}
	res := TextAndJSONResult("ok", bad)
	if len(res.Content) != 2 {
		t.Fatalf("expected 2 content blocks, got %d", len(res.Content))
	}
	second, _ := res.Content[1].(mcp.TextContent)
	if !strings.Contains(second.Text, "render error") {
		t.Errorf("expected render-error note, got %q", second.Text)
	}
}

func TestScopeOrErr_NilReturnsNil(t *testing.T) {
	acct := &accounts.Account{Label: "l", Email: "e@x"}
	if err := scopeOrErr(acct, "Sheets", nil, "X: %w", nil); err != nil {
		t.Errorf("expected nil for nil input, got %v", err)
	}
}

func TestScopeOrErr_PlainErrorIsFormatted(t *testing.T) {
	acct := &accounts.Account{Label: "personal", Email: "n@x"}
	orig := errors.New("not found")
	err := scopeOrErr(acct, "Sheets", orig, "reading on %s: %w", acct.Label, orig)
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if !strings.Contains(err.Error(), "reading on personal") {
		t.Errorf("format string not applied: %v", err)
	}
	if !errors.Is(err, orig) {
		t.Errorf("original error not wrapped with %%w: %v", err)
	}
}

func TestScopeOrErr_ScopeErrorIsReturnedAsScopeError(t *testing.T) {
	acct := &accounts.Account{Label: "personal", Email: "n@x"}
	gerr := &googleapi.Error{
		Code:    403,
		Message: "Request had insufficient authentication scopes.",
	}
	err := scopeOrErr(acct, "Sheets", gerr, "reading on %s: %w", acct.Label, gerr)
	var se *auth.ScopeError
	if !errors.As(err, &se) {
		t.Fatalf("expected *auth.ScopeError, got %T: %v", err, err)
	}
	if se.Operation != "Sheets" || se.AccountLabel != "personal" {
		t.Errorf("scope error fields wrong: %+v", se)
	}
	// Must NOT carry the handler's wrapping context when it's a scope error —
	// the reauth prompt should be clean.
	if strings.Contains(err.Error(), "reading on") {
		t.Errorf("scope error should not carry wrap context: %v", err)
	}
}

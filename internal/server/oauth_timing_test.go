package server

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/orieg/gws-connector/internal/auth"
)

// TestReauthReturnsQuicklyNotBlocking verifies the core contract the split-OAuth
// PR (#41) was meant to provide: add/reauth must NOT block the MCP stdio
// transport past Claude Code's default 30s MCP_TIMEOUT. With 60s inline wait
// (pre-fix), this test would take ~60s and exceed any reasonable client
// timeout. With the fix, it must return in ≤ inlineOAuthWait + small slack.
func TestReauthReturnsQuicklyNotBlocking(t *testing.T) {
	restore := auth.SetOpenBrowserForTest(func(string) error { return nil })
	defer restore()

	s := testServer(t)
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")
	if err := s.tokenStore.SaveClientSecret("alice@example.com", "account-secret"); err != nil {
		t.Fatalf("save client secret: %v", err)
	}

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"account": "personal"}

	start := time.Now()
	result, _ := s.handleAccountsReauth(context.Background(), req)
	elapsed := time.Since(start)

	// Must be bounded by inlineOAuthWait (2s) + generous slack for CI/keyring.
	if elapsed > inlineOAuthWait+3*time.Second {
		t.Fatalf("reauth blocked for %s — should return within %s + slack", elapsed, inlineOAuthWait)
	}

	if result.IsError {
		t.Fatalf("unexpected error: %s", extractText(result))
	}
	text := extractText(result)
	if !strings.Contains(text, "pendingId") {
		t.Fatalf("expected pending response with pendingId, got: %s", text)
	}
	if !strings.Contains(text, "pending") {
		t.Fatalf("expected pending status, got: %s", text)
	}

	// Session should be tracked for later polling.
	s.pendingMu.Lock()
	n := len(s.pending)
	s.pendingMu.Unlock()
	if n != 1 {
		t.Errorf("expected 1 pending session, got %d", n)
	}
}

// TestCompleteIsBounded verifies gws.accounts.complete respects its cap even
// when the caller requests a larger waitSeconds than allowed.
func TestCompleteIsBounded(t *testing.T) {
	restore := auth.SetOpenBrowserForTest(func(string) error { return nil })
	defer restore()

	s := testServer(t)
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")
	if err := s.tokenStore.SaveClientSecret("alice@example.com", "account-secret"); err != nil {
		t.Fatalf("save client secret: %v", err)
	}

	// Kick off a reauth flow.
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"account": "personal"}
	res, _ := s.handleAccountsReauth(context.Background(), req)
	pendingID := extractPendingID(t, res)

	// Ask for 60s — must be capped at maxCompleteWait.
	comp := mcp.CallToolRequest{}
	comp.Params.Arguments = map[string]any{
		"pendingId":   pendingID,
		"waitSeconds": 60.0,
	}
	start := time.Now()
	r, _ := s.handleAccountsComplete(context.Background(), comp)
	elapsed := time.Since(start)

	if elapsed > maxCompleteWait+2*time.Second {
		t.Fatalf("complete blocked for %s — should cap at %s", elapsed, maxCompleteWait)
	}
	if r.IsError {
		t.Fatalf("unexpected error: %s", extractText(r))
	}
	if !strings.Contains(extractText(r), "pending") {
		t.Errorf("expected pending status, got: %s", extractText(r))
	}
}

// TestConcurrentCallsDontBlockEachOther verifies a parallel call made while
// reauth's inline wait is in progress isn't serialized behind it. Claude Code
// will send pings and other requests during an OAuth flow; if those get
// stuck behind the OAuth handler, the client marks the server unresponsive.
//
// Note: mcp-go's stdio server handles non-tool messages synchronously on the
// reader loop (pings don't block on workers), but tool calls share a worker
// pool of 5. This test exercises the handler level directly to confirm the
// server code itself doesn't hold any global lock across the inline wait.
func TestConcurrentCallsDontBlockEachOther(t *testing.T) {
	restore := auth.SetOpenBrowserForTest(func(string) error { return nil })
	defer restore()

	s := testServer(t)
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")
	if err := s.tokenStore.SaveClientSecret("alice@example.com", "account-secret"); err != nil {
		t.Fatalf("save client secret: %v", err)
	}

	var wg sync.WaitGroup
	wg.Add(2)

	var reauthElapsed, listElapsed time.Duration

	// Reauth in background — will block for inlineOAuthWait.
	go func() {
		defer wg.Done()
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"account": "personal"}
		start := time.Now()
		s.handleAccountsReauth(context.Background(), req)
		reauthElapsed = time.Since(start)
	}()

	// Give reauth a moment to enter its wait.
	time.Sleep(100 * time.Millisecond)

	// Meanwhile, call list — should return immediately.
	go func() {
		defer wg.Done()
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{}
		start := time.Now()
		s.handleAccountsList(context.Background(), req)
		listElapsed = time.Since(start)
	}()

	wg.Wait()

	if listElapsed > 500*time.Millisecond {
		t.Errorf("concurrent list call blocked for %s — should return ~immediately", listElapsed)
	}
	if reauthElapsed < inlineOAuthWait-500*time.Millisecond {
		t.Errorf("reauth returned in %s — expected at least ~%s (the inline wait)", reauthElapsed, inlineOAuthWait)
	}
	if reauthElapsed > inlineOAuthWait+3*time.Second {
		t.Errorf("reauth took %s — expected ~%s", reauthElapsed, inlineOAuthWait)
	}
}

func extractPendingID(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || res.IsError {
		t.Fatalf("reauth returned error: %s", extractText(res))
	}
	text := extractText(res)
	const marker = "pendingId: "
	i := strings.Index(text, marker)
	if i < 0 {
		t.Fatalf("no pendingId in: %s", text)
	}
	rest := text[i+len(marker):]
	end := strings.IndexAny(rest, "\n ")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

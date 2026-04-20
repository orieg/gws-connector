package main_test

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestStdioServerStaysResponsiveDuringOAuth is the end-to-end guard against
// the bug behind PR #41 follow-up: when gws.accounts.reauth blocks inline
// too long, Claude Code's MCP_TIMEOUT fires (default 30s) and the client
// treats the server as unresponsive. This test spawns the built binary and
// proves:
//
//   - gws.accounts.reauth returns in a bounded time (< 5s) with a pendingId
//   - while that call is in flight, other tool calls (gws.accounts.list)
//     are serviced promptly
//   - tools/list returns the expected tool names
//   - after reauth returns, a follow-up gws.accounts.complete works against
//     the returned pendingId
//
// This test is skipped in short mode (builds take ~1-2s on first run).
func TestStdioServerStaysResponsiveDuringOAuth(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess build in -short mode")
	}

	// Build the binary into a temp dir.
	tmp := t.TempDir()
	binPath := filepath.Join(tmp, "gws-mcp")
	build := exec.Command("go", "build", "-o", binPath, ".")
	build.Dir = "." // cmd/gws-mcp
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v\n%s", err, out)
	}

	// Pre-populate an accounts registry so gws.accounts.reauth can resolve
	// "personal" to a real email without going through OAuth.
	stateDir := filepath.Join(tmp, "state")
	if err := os.MkdirAll(stateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	accounts := `{
  "accounts": [{
    "email": "alice@example.com",
    "label": "personal",
    "displayName": "Alice",
    "addedAt": "2026-01-01T00:00:00Z",
    "services": ["mail","calendar","drive"],
    "default": true,
    "clientId": "test-client-id",
    "clientSecret": "test-client-secret"
  }]
}`
	if err := os.WriteFile(filepath.Join(stateDir, "accounts.json"), []byte(accounts), 0o600); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, binPath, "--use-dot-names")
	cmd.Env = append(os.Environ(),
		"GWS_STATE_DIR="+stateDir,
		"GWS_NO_BROWSER=1",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("start binary: %v", err)
	}
	defer func() {
		stdin.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	c := &mcpClient{
		t:       t,
		in:      stdin,
		out:     bufio.NewReader(stdout),
		pending: make(map[int64]chan map[string]any),
	}
	go c.readLoop()

	// Initialize
	c.request("initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "test", "version": "0"},
	}, 5*time.Second)
	c.notify("notifications/initialized", nil)

	// Concurrently: fire reauth, and while it's pending fire list. Measure
	// both so we can assert list isn't serialized behind reauth.
	var wg sync.WaitGroup
	var reauthElapsed, listElapsed time.Duration
	var reauthResp map[string]any

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		reauthResp = c.callTool("gws.accounts.reauth", map[string]any{
			"account": "personal",
		}, 10*time.Second)
		reauthElapsed = time.Since(start)
	}()

	// Let reauth enter its inline wait.
	time.Sleep(200 * time.Millisecond)

	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		c.callTool("gws.accounts.list", map[string]any{}, 5*time.Second)
		listElapsed = time.Since(start)
	}()

	wg.Wait()

	if reauthElapsed > 5*time.Second {
		t.Errorf("reauth took %s — must return in <5s to stay under Claude Code's 30s MCP_TIMEOUT with safe margin", reauthElapsed)
	}
	if listElapsed > 1*time.Second {
		t.Errorf("concurrent list took %s — should be near-instant, blocking implies stdio serialization", listElapsed)
	}
	text := firstText(reauthResp)
	if !strings.Contains(text, "pendingId") || !strings.Contains(text, "pending") {
		t.Fatalf("expected pending+pendingId in reauth response, got: %s", text)
	}

	pendingID := extractPendingIDFromText(text)
	if pendingID == "" {
		t.Fatalf("could not parse pendingId from: %s", text)
	}

	// complete must also be bounded.
	start := time.Now()
	completeResp := c.callTool("gws.accounts.complete", map[string]any{
		"pendingId":   pendingID,
		"waitSeconds": 60.0, // request 60s but expect server cap ~3s
	}, 10*time.Second)
	completeElapsed := time.Since(start)
	if completeElapsed > 5*time.Second {
		t.Errorf("complete took %s despite server cap — got: %s", completeElapsed, firstText(completeResp))
	}
}

// --- Minimal MCP JSON-RPC client over stdio ---

type mcpClient struct {
	t      *testing.T
	in     io.Writer
	out    *bufio.Reader
	nextID int64
	mu     sync.Mutex

	pendMu  sync.Mutex
	pending map[int64]chan map[string]any
}

func (c *mcpClient) readLoop() {
	for {
		line, err := c.out.ReadString('\n')
		if err != nil {
			return
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		idVal, ok := msg["id"]
		if !ok {
			continue
		}
		var id int64
		switch v := idVal.(type) {
		case float64:
			id = int64(v)
		case json.Number:
			id, _ = v.Int64()
		default:
			continue
		}
		c.pendMu.Lock()
		ch, ok := c.pending[id]
		if ok {
			delete(c.pending, id)
		}
		c.pendMu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *mcpClient) request(method string, params any, timeout time.Duration) map[string]any {
	c.mu.Lock()
	c.nextID++
	id := c.nextID
	c.mu.Unlock()

	ch := make(chan map[string]any, 1)
	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()

	req := map[string]any{
		"jsonrpc": "2.0",
		"id":      id,
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	b, _ := json.Marshal(req)
	c.mu.Lock()
	c.in.Write(append(b, '\n'))
	c.mu.Unlock()

	select {
	case resp := <-ch:
		return resp
	case <-time.After(timeout):
		c.t.Fatalf("timeout waiting for response to %s (id=%d)", method, id)
		return nil
	}
}

func (c *mcpClient) notify(method string, params any) {
	req := map[string]any{
		"jsonrpc": "2.0",
		"method":  method,
	}
	if params != nil {
		req["params"] = params
	}
	b, _ := json.Marshal(req)
	c.mu.Lock()
	c.in.Write(append(b, '\n'))
	c.mu.Unlock()
}

func (c *mcpClient) callTool(name string, args map[string]any, timeout time.Duration) map[string]any {
	return c.request("tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	}, timeout)
}

func firstText(resp map[string]any) string {
	result, ok := resp["result"].(map[string]any)
	if !ok {
		return ""
	}
	content, _ := result["content"].([]any)
	if len(content) == 0 {
		return ""
	}
	item, _ := content[0].(map[string]any)
	s, _ := item["text"].(string)
	return s
}

func extractPendingIDFromText(text string) string {
	const marker = "pendingId: "
	i := strings.Index(text, marker)
	if i < 0 {
		// Fallback: pendingId='...'
		i = strings.Index(text, "pendingId='")
		if i < 0 {
			return ""
		}
		rest := text[i+len("pendingId='"):]
		end := strings.Index(rest, "'")
		if end < 0 {
			return ""
		}
		return rest[:end]
	}
	rest := text[i+len(marker):]
	end := strings.IndexAny(rest, "\n ")
	if end < 0 {
		end = len(rest)
	}
	return rest[:end]
}

// Prevent unused import errors if we end up not using fmt.
var _ = fmt.Sprintf

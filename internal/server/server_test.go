package server

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func testServer(t *testing.T) *Server {
	t.Helper()
	dir := t.TempDir()
	cfg := Config{
		StateDir:    dir,
		ClientID:    "test-client-id",
		ClientSecret: "test-client-secret",
		UseDotNames: true,
	}
	return New(cfg)
}

func callTool(t *testing.T, s *Server, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args

	// Find and call the handler via the MCP server
	// Since we can't directly call through MCPServer, we test handlers directly
	return nil // placeholder — individual handler tests below
}

// --- Account management handler tests ---

func TestHandleAccountsListEmpty(t *testing.T) {
	s := testServer(t)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := s.handleAccountsList(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAccountsList error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "No accounts connected") {
		t.Errorf("expected 'No accounts connected', got: %s", text)
	}
}

func TestHandleAccountsListWithAccounts(t *testing.T) {
	s := testServer(t)

	// Add accounts directly via store
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")
	s.accountStore.Add("bob@work.com", "work", "Bob", "")

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, err := s.handleAccountsList(context.Background(), req)
	if err != nil {
		t.Fatalf("handleAccountsList error: %v", err)
	}
	text := extractText(result)
	if !strings.Contains(text, "personal") || !strings.Contains(text, "alice@example.com") {
		t.Errorf("expected alice listed: %s", text)
	}
	if !strings.Contains(text, "work") || !strings.Contains(text, "bob@work.com") {
		t.Errorf("expected bob listed: %s", text)
	}
	if !strings.Contains(text, "[DEFAULT]") {
		t.Errorf("expected [DEFAULT] marker: %s", text)
	}
	if !strings.Contains(text, "2") {
		t.Errorf("expected count of 2: %s", text)
	}
}

func TestHandleAccountsAddMissingLabel(t *testing.T) {
	s := testServer(t)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, _ := s.handleAccountsAdd(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when label is missing")
	}
	text := extractText(result)
	if !strings.Contains(text, "label is required") {
		t.Errorf("expected 'label is required', got: %s", text)
	}
}

func TestHandleAccountsAddMissingCredentials(t *testing.T) {
	dir := t.TempDir()
	s := New(Config{
		StateDir:    dir,
		ClientID:    "", // no credentials
		ClientSecret: "",
		UseDotNames: true,
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"label": "work"}

	result, _ := s.handleAccountsAdd(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when credentials missing")
	}
	text := extractText(result)
	if !strings.Contains(text, "OAuth credentials are required") {
		t.Errorf("expected credentials error, got: %s", text)
	}
}

func TestHandleAccountsAddPerAccountCredentialsOverride(t *testing.T) {
	dir := t.TempDir()
	s := New(Config{
		StateDir:    dir,
		ClientID:    "", // no global credentials
		ClientSecret: "",
		UseDotNames: true,
	})

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"label":        "work",
		"clientId":     "per-account-id",
		"clientSecret": "per-account-secret",
	}

	// Use a cancelled context so the OAuth flow exits immediately
	// without trying to open a browser and wait
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	result, _ := s.handleAccountsAdd(ctx, req)
	text := extractText(result)
	// It should have passed the credentials check (not returned "OAuth credentials are required")
	// and failed at the OAuth flow instead (context cancelled or browser error)
	if strings.Contains(text, "OAuth credentials are required") {
		t.Errorf("per-account credentials should override empty globals, got: %s", text)
	}
}

func TestHandleAccountsRemoveNotFound(t *testing.T) {
	s := testServer(t)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"account": "nonexistent"}

	result, _ := s.handleAccountsRemove(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for nonexistent account")
	}
}

func TestHandleAccountsRemoveSuccess(t *testing.T) {
	s := testServer(t)
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"account": "personal"}

	result, _ := s.handleAccountsRemove(context.Background(), req)
	if result.IsError {
		t.Errorf("unexpected error: %s", extractText(result))
	}
	text := extractText(result)
	if !strings.Contains(text, "Removed") {
		t.Errorf("expected 'Removed', got: %s", text)
	}

	// Verify account is gone
	accts, _ := s.accountRouter.ListAccounts()
	if len(accts) != 0 {
		t.Error("account should be removed")
	}
}

func TestHandleAccountsSetDefaultSuccess(t *testing.T) {
	s := testServer(t)
	s.accountStore.Add("alice@example.com", "personal", "Alice", "")
	s.accountStore.Add("bob@work.com", "work", "Bob", "")

	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"account": "work"}

	result, _ := s.handleAccountsSetDefault(context.Background(), req)
	if result.IsError {
		t.Errorf("unexpected error: %s", extractText(result))
	}

	// Verify default changed
	acct, _ := s.accountStore.GetDefault()
	if acct.Email != "bob@work.com" {
		t.Errorf("expected bob as default, got %s", acct.Email)
	}
}

func TestToolNamingDotNames(t *testing.T) {
	s := testServer(t)
	name := s.toolName("gws", "mail", "search")
	if name != "gws.mail.search" {
		t.Errorf("expected gws.mail.search, got %s", name)
	}
}

func TestToolNamingUnderscore(t *testing.T) {
	dir := t.TempDir()
	s := New(Config{StateDir: dir, UseDotNames: false})
	name := s.toolName("gws", "mail", "search")
	if name != "gws_mail_search" {
		t.Errorf("expected gws_mail_search, got %s", name)
	}
}

// --- E2E: verify tool registration ---

func TestAllToolsRegistered(t *testing.T) {
	s := testServer(t)

	tools := s.mcpServer.ListTools()

	expected := []string{
		// Account management
		"gws.accounts.list",
		"gws.accounts.add",
		"gws.accounts.remove",
		"gws.accounts.set_default",
		// Mail
		"gws.mail.search",
		"gws.mail.read_message",
		"gws.mail.read_thread",
		"gws.mail.create_draft",
		"gws.mail.list_labels",
		"gws.mail.get_profile",
		// Calendar
		"gws.cal.list_events",
		"gws.cal.get_event",
		"gws.cal.create_event",
		"gws.cal.list_calendars",
		// Drive
		"gws.drive.search",
		"gws.drive.read_file",
		"gws.drive.list_folder",
	}

	for _, name := range expected {
		if _, ok := tools[name]; !ok {
			t.Errorf("expected tool %q to be registered", name)
		}
	}

	t.Logf("Registered %d tools total", len(tools))
}

func TestToolHasAccountParam(t *testing.T) {
	s := testServer(t)

	tools := s.mcpServer.ListTools()

	// All non-account-management tools should have an "account" parameter
	toolsWithAccount := []string{
		"gws.mail.search",
		"gws.cal.list_events",
		"gws.drive.search",
	}

	for _, toolName := range toolsWithAccount {
		if tool, ok := tools[toolName]; ok {
			schema, _ := json.Marshal(tool.Tool.InputSchema)
			if !strings.Contains(string(schema), "account") {
				t.Errorf("tool %q should have 'account' parameter", toolName)
			}
		} else {
			t.Errorf("tool %q not found", toolName)
		}
	}
}

// --- Helpers ---

func extractText(result *mcp.CallToolResult) string {
	if result == nil || len(result.Content) == 0 {
		return ""
	}
	if tc, ok := result.Content[0].(mcp.TextContent); ok {
		return tc.Text
	}
	return ""
}

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
		StateDir:     dir,
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		UseDotNames:  true,
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
		StateDir:     dir,
		ClientID:     "", // no credentials
		ClientSecret: "",
		UseDotNames:  true,
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
		StateDir:     dir,
		ClientID:     "", // no global credentials
		ClientSecret: "",
		UseDotNames:  true,
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
		"gws.accounts.reauth",
		"gws.accounts.complete",
		"gws.accounts.remove",
		"gws.accounts.set_default",
		// Mail
		"gws.mail.search",
		"gws.mail.read_message",
		"gws.mail.read_thread",
		"gws.mail.create_draft",
		"gws.mail.send_draft",
		"gws.mail.forward",
		"gws.mail.get_attachment",
		"gws.mail.list_labels",
		"gws.mail.create_label",
		"gws.mail.modify_message",
		"gws.mail.get_profile",
		// Calendar
		"gws.cal.list_events",
		"gws.cal.get_event",
		"gws.cal.create_event",
		"gws.cal.update_event",
		"gws.cal.delete_event",
		"gws.cal.free_busy",
		"gws.cal.list_calendars",
		// Drive
		"gws.drive.search",
		"gws.drive.read_file",
		"gws.drive.list_folder",
		// Sheets
		"gws.sheets.read_range",
		"gws.sheets.write_range",
		"gws.sheets.append",
		"gws.sheets.clear",
		"gws.sheets.create",
		"gws.sheets.list_tabs",
		// Docs
		"gws.docs.read",
		"gws.docs.insert_text",
		"gws.docs.replace_text",
		"gws.docs.create",
		// Contacts / People
		"gws.contacts.search",
		"gws.contacts.directory_search",
		// Tasks
		"gws.tasks.list_tasklists",
		"gws.tasks.list",
		"gws.tasks.create",
		"gws.tasks.complete",
		"gws.tasks.delete",
		// Slides
		"gws.slides.get",
		"gws.slides.create",
		"gws.slides.batch_update",
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
		"gws.sheets.read_range",
		"gws.sheets.write_range",
		"gws.docs.read",
		"gws.docs.insert_text",
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

// Guards the mark3labs/mcp-go gotcha: NewTool defaults destructiveHint to
// true, so update_event (a reversible, additive modify) must set it false
// explicitly, while delete_event must keep it true.
func TestCalendarWriteToolAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	// update_event: reversible modify — neither readOnly nor destructive true.
	if tool, ok := tools["gws.cal.update_event"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("update_event destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
		if a.ReadOnlyHint != nil && *a.ReadOnlyHint {
			t.Errorf("update_event readOnlyHint should not be true, got %s", boolVal(a.ReadOnlyHint))
		}
	} else {
		t.Error("gws.cal.update_event not registered")
	}

	// delete_event: genuinely destructive.
	if tool, ok := tools["gws.cal.delete_event"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != true {
			t.Errorf("delete_event destructiveHint should be true, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.cal.delete_event not registered")
	}

	// free_busy: read-only.
	if tool, ok := tools["gws.cal.free_busy"]; ok {
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint != true {
			t.Errorf("free_busy readOnlyHint should be true, got %s", boolVal(a.ReadOnlyHint))
		}
	} else {
		t.Error("gws.cal.free_busy not registered")
	}
}

// Guards the same mark3labs/mcp-go gotcha for Sheets write tools: append is
// additive (adds rows, never overwrites) so destructiveHint must be false,
// while clear empties cells and must keep destructiveHint true.
func TestSheetsWriteToolAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	if tool, ok := tools["gws.sheets.append"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("append destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.sheets.append not registered")
	}

	if tool, ok := tools["gws.sheets.clear"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != true {
			t.Errorf("clear destructiveHint should be true, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.sheets.clear not registered")
	}
}

// Guards the read-only Contacts tools: both readOnly=true, destructive=false.
func TestContactsToolAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	for _, name := range []string{"gws.contacts.search", "gws.contacts.directory_search"} {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint != true {
			t.Errorf("%s readOnlyHint should be true, got %s", name, boolVal(a.ReadOnlyHint))
		}
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("%s destructiveHint should be false, got %s", name, boolVal(a.DestructiveHint))
		}
	}
}

// Guards Tasks tools: list_tasklists/list read-only; create/complete additive;
// delete destructive.
func TestTasksToolAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	readOnly := []string{"gws.tasks.list_tasklists", "gws.tasks.list"}
	for _, name := range readOnly {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint != true {
			t.Errorf("%s readOnlyHint should be true, got %s", name, boolVal(a.ReadOnlyHint))
		}
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("%s destructiveHint should be false, got %s", name, boolVal(a.DestructiveHint))
		}
	}

	nonDestructive := []string{"gws.tasks.create", "gws.tasks.complete"}
	for _, name := range nonDestructive {
		tool, ok := tools[name]
		if !ok {
			t.Errorf("%s not registered", name)
			continue
		}
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("%s destructiveHint should be false, got %s", name, boolVal(a.DestructiveHint))
		}
	}

	if tool, ok := tools["gws.tasks.delete"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != true {
			t.Errorf("delete destructiveHint should be true, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.tasks.delete not registered")
	}
}

// Guards the new Gmail tools: forward builds a draft (additive), get_attachment
// is read-only.
func TestMailForwardAttachmentAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	if tool, ok := tools["gws.mail.forward"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("forward destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.mail.forward not registered")
	}

	if tool, ok := tools["gws.mail.get_attachment"]; ok {
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint != true {
			t.Errorf("get_attachment readOnlyHint should be true, got %s", boolVal(a.ReadOnlyHint))
		}
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("get_attachment destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.mail.get_attachment not registered")
	}
}

// Guards Slides tools: get read-only, create additive, batch_update destructive.
func TestSlidesToolAnnotations(t *testing.T) {
	s := testServer(t)
	tools := s.mcpServer.ListTools()

	boolVal := func(p *bool) string {
		if p == nil {
			return "unset"
		}
		if *p {
			return "true"
		}
		return "false"
	}

	if tool, ok := tools["gws.slides.get"]; ok {
		a := tool.Tool.Annotations
		if a.ReadOnlyHint == nil || *a.ReadOnlyHint != true {
			t.Errorf("slides.get readOnlyHint should be true, got %s", boolVal(a.ReadOnlyHint))
		}
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("slides.get destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.slides.get not registered")
	}

	if tool, ok := tools["gws.slides.create"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != false {
			t.Errorf("slides.create destructiveHint should be false, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.slides.create not registered")
	}

	if tool, ok := tools["gws.slides.batch_update"]; ok {
		a := tool.Tool.Annotations
		if a.DestructiveHint == nil || *a.DestructiveHint != true {
			t.Errorf("slides.batch_update destructiveHint should be true, got %s", boolVal(a.DestructiveHint))
		}
	} else {
		t.Error("gws.slides.batch_update not registered")
	}
}

func TestHandleAccountsCompleteMissingPendingId(t *testing.T) {
	s := testServer(t)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{}

	result, _ := s.handleAccountsComplete(context.Background(), req)
	if !result.IsError {
		t.Error("expected error when pendingId missing")
	}
	if !strings.Contains(extractText(result), "pendingId is required") {
		t.Errorf("unexpected: %s", extractText(result))
	}
}

func TestHandleAccountsCompleteUnknownPendingId(t *testing.T) {
	s := testServer(t)
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"pendingId": "does-not-exist"}

	result, _ := s.handleAccountsComplete(context.Background(), req)
	if !result.IsError {
		t.Error("expected error for unknown pendingId")
	}
	if !strings.Contains(extractText(result), "unknown pendingId") {
		t.Errorf("unexpected: %s", extractText(result))
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

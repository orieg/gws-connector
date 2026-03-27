package server

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
	"github.com/orieg/gws-connector/internal/services"
)

// Config holds server configuration.
type Config struct {
	StateDir     string
	ClientID     string
	ClientSecret string
	UseDotNames  bool
}

// Server is the GWS MCP server.
type Server struct {
	config        Config
	mcpServer     *mcpserver.MCPServer
	accountStore  *accounts.Store
	accountRouter *accounts.Router
	tokenStore    *auth.TokenStore
	clientFactory *auth.ClientFactory
	mailSvc       *services.MailService
	calSvc        *services.CalendarService
	driveSvc      *services.DriveService
}

// New creates a new GWS MCP server.
func New(cfg Config) *Server {
	store := accounts.NewStore(cfg.StateDir)
	router := accounts.NewRouter(store)
	tokenStore := auth.NewTokenStore(cfg.StateDir)
	clientFactory := auth.NewClientFactory(tokenStore, cfg.ClientID, cfg.ClientSecret, store)

	s := &Server{
		config:        cfg,
		accountStore:  store,
		accountRouter: router,
		tokenStore:    tokenStore,
		clientFactory: clientFactory,
		mailSvc:       services.NewMailService(router, clientFactory),
		calSvc:        services.NewCalendarService(router, clientFactory),
		driveSvc:      services.NewDriveService(router, clientFactory),
	}

	s.mcpServer = mcpserver.NewMCPServer(
		"gws-connector",
		"0.1.0",
		mcpserver.WithToolCapabilities(true),
	)

	s.migrateClientSecrets()
	s.registerTools()
	return s
}

// migrateClientSecrets moves any client secrets from accounts.json (legacy)
// into the OS keychain. This handles upgrades from older versions.
func (s *Server) migrateClientSecrets() {
	toMigrate := s.accountStore.MigrateClientSecrets()
	for _, acct := range toMigrate {
		if err := s.tokenStore.SaveClientSecret(acct.Email, acct.ClientSecret); err != nil {
			fmt.Fprintf(os.Stderr, "gws-connector: failed to migrate client secret for %s to keychain: %v\n", acct.Email, err)
			continue
		}
		if err := s.accountStore.ClearClientSecret(acct.Email); err != nil {
			fmt.Fprintf(os.Stderr, "gws-connector: failed to clear migrated client secret for %s: %v\n", acct.Email, err)
		}
	}
}

// toolName applies dot-naming if configured.
func (s *Server) toolName(parts ...string) string {
	if s.config.UseDotNames {
		return strings.Join(parts, ".")
	}
	return strings.Join(parts, "_")
}

// registerTools registers all MCP tools.
func (s *Server) registerTools() {
	// Meta: account management
	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "accounts", "list"),
			mcp.WithDescription("List all connected Google Workspace accounts with their labels and default status"),
		),
		s.handleAccountsList,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "accounts", "add"),
			mcp.WithDescription("Connect a new Google account via OAuth. Opens browser for authorization. "+
				"Each account stores its own credentials. For accounts in different organizations, "+
				"provide that org's clientId/clientSecret from their GCP project."),
			mcp.WithString("label", mcp.Required(), mcp.Description("A short label for this account (e.g., 'work', 'personal', 'client-acme')")),
			mcp.WithString("clientId", mcp.Description("OAuth Client ID for this account's GCP project.")),
			mcp.WithString("clientSecret", mcp.Description("OAuth Client Secret for this account's GCP project. Stored securely in OS keychain.")),
		),
		s.handleAccountsAdd,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "accounts", "remove"),
			mcp.WithDescription("Disconnect a Google account and delete its tokens"),
			mcp.WithString("account", mcp.Required(), mcp.Description("Account label or email to remove")),
		),
		s.handleAccountsRemove,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "accounts", "set_default"),
			mcp.WithDescription("Set the default account used when no account is specified"),
			mcp.WithString("account", mcp.Required(), mcp.Description("Account label or email to set as default")),
		),
		s.handleAccountsSetDefault,
	)

	// Mail tools
	s.registerMailTools()

	// Calendar tools
	s.registerCalendarTools()

	// Drive tools
	s.registerDriveTools()
}

// Serve starts the MCP server on stdio.
func (s *Server) Serve() error {
	return mcpserver.ServeStdio(s.mcpServer)
}

// --- Account management handlers ---

func (s *Server) handleAccountsList(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	accts, err := s.accountRouter.ListAccounts()
	if err != nil {
		return errorResult(err), nil
	}

	if len(accts) == 0 {
		return textResult("No accounts connected. Run /gws:add-account to connect a Google account."), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Connected accounts (%d):\n\n", len(accts)))
	for i, a := range accts {
		def := ""
		if a.Default {
			def = " [DEFAULT]"
		}
		sb.WriteString(fmt.Sprintf("%d. %s (%s)%s\n", i+1, a.Label, a.Email, def))
		sb.WriteString(fmt.Sprintf("   Services: %s\n", strings.Join(a.Services, ", ")))
	}
	return textResult(sb.String()), nil
}

func (s *Server) handleAccountsAdd(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	label, _ := req.GetArguments()["label"].(string)
	if label == "" {
		return errorResult(fmt.Errorf("label is required")), nil
	}

	// Capture original per-account credentials from request (before fallback)
	reqClientID, _ := req.GetArguments()["clientId"].(string)
	reqClientSecret, _ := req.GetArguments()["clientSecret"].(string)

	// Resolve effective credentials: per-account → global fallback
	clientID := reqClientID
	clientSecret := reqClientSecret
	if clientID == "" {
		clientID = s.config.ClientID
	}
	if clientSecret == "" {
		clientSecret = s.config.ClientSecret
	}

	if clientID == "" || clientSecret == "" {
		return errorResult(fmt.Errorf(
			"OAuth credentials are required. Pass clientId and clientSecret parameters.\n\n" +
				"You can get these from your GCP project's OAuth client credentials.\n" +
				"Run /gws:configure for step-by-step setup instructions.")), nil
	}

	token, info, err := auth.OAuthFlow(ctx, clientID, clientSecret)
	if err != nil {
		return errorResult(fmt.Errorf("OAuth authorization failed: %w", err)), nil
	}

	// Store token
	if err := s.tokenStore.Save(info.Email, token); err != nil {
		return errorResult(fmt.Errorf("saving token: %w", err)), nil
	}

	// Store client secret in keychain (not in accounts.json)
	if err := s.tokenStore.SaveClientSecret(info.Email, clientSecret); err != nil {
		return errorResult(fmt.Errorf("saving client secret: %w", err)), nil
	}

	// Register account — store per-account client ID if explicitly provided
	perAcctID := reqClientID
	if err := s.accountStore.Add(info.Email, label, info.DisplayName, perAcctID); err != nil {
		return errorResult(fmt.Errorf("registering account: %w", err)), nil
	}

	accts, _ := s.accountRouter.ListAccounts()
	credNote := "using global credentials"
	if perAcctID != "" {
		credNote = "using custom credentials for this org"
	}
	return textResult(fmt.Sprintf(
		"Successfully connected %s (%s) as '%s' (%s). You now have %d account(s) connected.",
		info.DisplayName, info.Email, label, credNote, len(accts),
	)), nil
}

func (s *Server) handleAccountsRemove(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, _ := req.GetArguments()["account"].(string)
	if account == "" {
		return errorResult(fmt.Errorf("account is required")), nil
	}

	// Resolve to get email before removing
	acct, err := s.accountRouter.Resolve(account)
	if err != nil {
		return errorResult(err), nil
	}

	// Delete token and client secret from keychain
	s.tokenStore.Delete(acct.Email)
	s.tokenStore.DeleteClientSecret(acct.Email)

	// Remove from registry
	if err := s.accountStore.Remove(account); err != nil {
		return errorResult(err), nil
	}

	return textResult(fmt.Sprintf("Removed account '%s' (%s). Tokens deleted.", acct.Label, acct.Email)), nil
}

func (s *Server) handleAccountsSetDefault(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	account, _ := req.GetArguments()["account"].(string)
	if account == "" {
		return errorResult(fmt.Errorf("account is required")), nil
	}

	if err := s.accountStore.SetDefault(account); err != nil {
		return errorResult(err), nil
	}

	acct, _ := s.accountRouter.Resolve(account)
	return textResult(fmt.Sprintf("Default account set to '%s' (%s).", acct.Label, acct.Email)), nil
}

// --- Placeholder registrations for service tools ---

func (s *Server) registerMailTools() {
	accountParam := mcp.WithString("account", mcp.Description("Account label or email. Uses default if omitted."))

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "search"),
			mcp.WithDescription("Search emails using Gmail search syntax (e.g., 'from:user@example.com is:unread')"),
			mcp.WithString("query", mcp.Description("Gmail search query")),
			mcp.WithNumber("maxResults", mcp.Description("Maximum messages to return (default: 20)")),
			accountParam,
		),
		s.mailSvc.Search,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "read_message"),
			mcp.WithDescription("Read the full content of an email message"),
			mcp.WithString("messageId", mcp.Required(), mcp.Description("The message ID")),
			accountParam,
		),
		s.mailSvc.ReadMessage,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "read_thread"),
			mcp.WithDescription("Read all messages in an email thread"),
			mcp.WithString("threadId", mcp.Required(), mcp.Description("The thread ID")),
			accountParam,
		),
		s.mailSvc.ReadThread,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "create_draft"),
			mcp.WithDescription("Create an email draft"),
			mcp.WithString("to", mcp.Description("Recipient email(s), comma-separated")),
			mcp.WithString("subject", mcp.Description("Email subject")),
			mcp.WithString("body", mcp.Required(), mcp.Description("Email body (plain text)")),
			mcp.WithString("cc", mcp.Description("CC recipients, comma-separated")),
			mcp.WithString("bcc", mcp.Description("BCC recipients, comma-separated")),
			mcp.WithString("threadId", mcp.Description("Thread ID for reply drafts")),
			accountParam,
		),
		s.mailSvc.CreateDraft,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "list_labels"),
			mcp.WithDescription("List all Gmail labels for the account"),
			accountParam,
		),
		s.mailSvc.ListLabels,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "create_label"),
			mcp.WithDescription("Create a new Gmail label"),
			mcp.WithString("name", mcp.Required(), mcp.Description("Label name (e.g., 'Projects/Alpha')")),
			mcp.WithString("backgroundColor", mcp.Description("Label background color hex (e.g., '#16a765')")),
			mcp.WithString("textColor", mcp.Description("Label text color hex (e.g., '#ffffff')")),
			accountParam,
		),
		s.mailSvc.CreateLabel,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "modify_message"),
			mcp.WithDescription("Add or remove labels from a Gmail message (use for archiving, starring, marking read/unread, or applying custom labels)"),
			mcp.WithString("messageId", mcp.Required(), mcp.Description("The message ID")),
			mcp.WithArray("addLabelIds", mcp.Description("Label IDs to add (e.g., ['STARRED', 'Label_123'])")),
			mcp.WithArray("removeLabelIds", mcp.Description("Label IDs to remove (e.g., ['INBOX', 'UNREAD'])")),
			accountParam,
		),
		s.mailSvc.ModifyMessage,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "mail", "get_profile"),
			mcp.WithDescription("Get Gmail profile info (email, messages total, threads total)"),
			accountParam,
		),
		s.mailSvc.GetProfile,
	)
}

func (s *Server) registerCalendarTools() {
	accountParam := mcp.WithString("account", mcp.Description("Account label or email. Uses default if omitted."))

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "cal", "list_events"),
			mcp.WithDescription("List calendar events within a time range"),
			mcp.WithString("timeMin", mcp.Required(), mcp.Description("Start of range (RFC3339, e.g., 2026-03-26T00:00:00Z)")),
			mcp.WithString("timeMax", mcp.Required(), mcp.Description("End of range (RFC3339)")),
			mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
			mcp.WithString("q", mcp.Description("Free text search query")),
			mcp.WithNumber("maxResults", mcp.Description("Maximum events to return (default: 50)")),
			accountParam,
		),
		s.calSvc.ListEvents,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "cal", "get_event"),
			mcp.WithDescription("Get full details of a calendar event"),
			mcp.WithString("eventId", mcp.Required(), mcp.Description("Event ID")),
			mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
			accountParam,
		),
		s.calSvc.GetEvent,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "cal", "create_event"),
			mcp.WithDescription("Create a new calendar event"),
			mcp.WithString("summary", mcp.Required(), mcp.Description("Event title")),
			mcp.WithString("start", mcp.Required(), mcp.Description("Start time (RFC3339)")),
			mcp.WithString("end", mcp.Required(), mcp.Description("End time (RFC3339)")),
			mcp.WithString("description", mcp.Description("Event description")),
			mcp.WithString("location", mcp.Description("Event location")),
			mcp.WithString("calendarId", mcp.Description("Calendar ID (default: primary)")),
			accountParam,
		),
		s.calSvc.CreateEvent,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "cal", "list_calendars"),
			mcp.WithDescription("List all calendars for the account"),
			accountParam,
		),
		s.calSvc.ListCalendars,
	)
}

func (s *Server) registerDriveTools() {
	accountParam := mcp.WithString("account", mcp.Description("Account label or email. Uses default if omitted."))

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "drive", "search"),
			mcp.WithDescription("Search for files in Google Drive"),
			mcp.WithString("query", mcp.Required(), mcp.Description("Drive search query (e.g., 'name contains report')")),
			mcp.WithNumber("maxResults", mcp.Description("Maximum files to return (default: 20)")),
			accountParam,
		),
		s.driveSvc.Search,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "drive", "read_file"),
			mcp.WithDescription("Read the content of a file from Google Drive"),
			mcp.WithString("fileId", mcp.Required(), mcp.Description("File ID")),
			accountParam,
		),
		s.driveSvc.ReadFile,
	)

	s.mcpServer.AddTool(
		mcp.NewTool(s.toolName("gws", "drive", "list_folder"),
			mcp.WithDescription("List files in a Drive folder"),
			mcp.WithString("folderId", mcp.Description("Folder ID (default: root)")),
			mcp.WithNumber("maxResults", mcp.Description("Maximum files to return (default: 50)")),
			accountParam,
		),
		s.driveSvc.ListFolder,
	)
}

// --- Helpers (delegate to services package) ---

var (
	textResult  = services.TextResult
	errorResult = services.ErrorResult
)

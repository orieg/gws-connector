package services

import (
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/api/drive/v3"

	"github.com/orieg/claude-multi-gws/internal/accounts"
	"github.com/orieg/claude-multi-gws/internal/auth"
)

// DriveService implements Drive-related MCP tools.
type DriveService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewDriveService creates a new drive service.
func NewDriveService(router *accounts.Router, clients *auth.ClientFactory) *DriveService {
	return &DriveService{router: router, clients: clients}
}

func (d *DriveService) resolveAndGetService(ctx context.Context, args map[string]any) (*drive.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := d.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := d.clients.DriveService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// Search searches for files in Drive.
func (d *DriveService) Search(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	query, _ := req.GetArguments()["query"].(string)
	maxResults := int64(20)
	if mr, ok := req.GetArguments()["maxResults"].(float64); ok && mr > 0 {
		maxResults = int64(mr)
	}

	call := svc.Files.List().
		Q(query).
		PageSize(maxResults).
		Fields("files(id, name, mimeType, modifiedTime, size, webViewLink, owners)")

	resp, err := call.Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("searching drive on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Files) == 0 {
		return TextResult(fmt.Sprintf("No files found on %s (%s) for query: %s", acct.Label, acct.Email, query)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Files on %s (%s) — %d found:\n\n", acct.Label, acct.Email, len(resp.Files)))

	for i, f := range resp.Files {
		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, f.Name))
		sb.WriteString(fmt.Sprintf("   Type: %s\n", f.MimeType))
		sb.WriteString(fmt.Sprintf("   Modified: %s\n", f.ModifiedTime))
		if f.Size > 0 {
			sb.WriteString(fmt.Sprintf("   Size: %s\n", formatSize(f.Size)))
		}
		sb.WriteString(fmt.Sprintf("   ID: %s\n", f.Id))
		if f.WebViewLink != "" {
			sb.WriteString(fmt.Sprintf("   Link: %s\n", f.WebViewLink))
		}
		sb.WriteString("\n")
	}

	return TextResult(sb.String()), nil
}

// ReadFile reads the text content of a file.
func (d *DriveService) ReadFile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	fileId, _ := req.GetArguments()["fileId"].(string)
	if fileId == "" {
		return ErrorResult(fmt.Errorf("fileId is required")), nil
	}

	// Get file metadata first to determine type
	file, err := svc.Files.Get(fileId).Fields("name, mimeType, size").Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("getting file metadata on %s: %w", acct.Label, err)), nil
	}

	var content string

	// Google Docs types need export
	switch file.MimeType {
	case "application/vnd.google-apps.document":
		resp, err := svc.Files.Export(fileId, "text/plain").Download()
		if err != nil {
			return ErrorResult(fmt.Errorf("exporting doc on %s: %w", acct.Label, err)), nil
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		content = string(data)

	case "application/vnd.google-apps.spreadsheet":
		resp, err := svc.Files.Export(fileId, "text/csv").Download()
		if err != nil {
			return ErrorResult(fmt.Errorf("exporting sheet on %s: %w", acct.Label, err)), nil
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		content = string(data)

	case "application/vnd.google-apps.presentation":
		resp, err := svc.Files.Export(fileId, "text/plain").Download()
		if err != nil {
			return ErrorResult(fmt.Errorf("exporting slides on %s: %w", acct.Label, err)), nil
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(resp.Body)
		content = string(data)

	default:
		// Regular file — download content
		resp, err := svc.Files.Get(fileId).Download()
		if err != nil {
			return ErrorResult(fmt.Errorf("downloading file on %s: %w", acct.Label, err)), nil
		}
		defer resp.Body.Close()
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024)) // 1MB limit
		content = string(data)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("File: %s\nType: %s\nAccount: %s (%s)\n\n", file.Name, file.MimeType, acct.Label, acct.Email))
	sb.WriteString(content)

	return TextResult(sb.String()), nil
}

// ListFolder lists files in a Drive folder.
func (d *DriveService) ListFolder(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	folderId, _ := req.GetArguments()["folderId"].(string)
	maxResults := int64(50)
	if mr, ok := req.GetArguments()["maxResults"].(float64); ok && mr > 0 {
		maxResults = int64(mr)
	}

	query := "'root' in parents and trashed = false"
	if folderId != "" {
		query = fmt.Sprintf("'%s' in parents and trashed = false", folderId)
	}

	call := svc.Files.List().
		Q(query).
		PageSize(maxResults).
		Fields("files(id, name, mimeType, modifiedTime, size)").
		OrderBy("folder,name")

	resp, err := call.Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("listing folder on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Files) == 0 {
		return TextResult(fmt.Sprintf("Folder is empty on %s (%s).", acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Files on %s (%s) — %d items:\n\n", acct.Label, acct.Email, len(resp.Files)))

	for i, f := range resp.Files {
		icon := "  "
		if strings.Contains(f.MimeType, "folder") {
			icon = "[folder] "
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s\n", i+1, icon, f.Name))
		sb.WriteString(fmt.Sprintf("   Type: %s | Modified: %s | ID: %s\n", f.MimeType, f.ModifiedTime, f.Id))
	}

	return TextResult(sb.String()), nil
}

func formatSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

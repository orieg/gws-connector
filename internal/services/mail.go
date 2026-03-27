package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"regexp"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/api/gmail/v1"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// MailService implements Gmail-related MCP tools.
type MailService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewMailService creates a new mail service.
func NewMailService(router *accounts.Router, clients *auth.ClientFactory) *MailService {
	return &MailService{router: router, clients: clients}
}

func (m *MailService) resolveAndGetService(ctx context.Context, args map[string]any) (*gmail.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := m.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := m.clients.GmailService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// Search searches for emails.
func (m *MailService) Search(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	query, _ := req.GetArguments()["query"].(string)
	maxResults := int64(20)
	if mr, ok := req.GetArguments()["maxResults"].(float64); ok && mr > 0 {
		maxResults = int64(mr)
	}
	if maxResults > 500 {
		maxResults = 500
	}

	call := svc.Users.Messages.List("me").Q(query).MaxResults(maxResults)
	resp, err := call.Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("searching mail on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Messages) == 0 {
		return TextResult(fmt.Sprintf("No messages found for query '%s' on %s (%s).", query, acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d message(s) on %s (%s):\n\n", len(resp.Messages), acct.Label, acct.Email))

	for i, msg := range resp.Messages {
		// Fetch message metadata
		full, err := svc.Users.Messages.Get("me", msg.Id).Format("metadata").MetadataHeaders("From", "Subject", "Date").Do()
		if err != nil {
			sb.WriteString(fmt.Sprintf("%d. [Error fetching message %s]\n", i+1, msg.Id))
			continue
		}

		from, subject, date := "", "", ""
		for _, h := range full.Payload.Headers {
			switch h.Name {
			case "From":
				from = h.Value
			case "Subject":
				subject = h.Value
			case "Date":
				date = h.Value
			}
		}

		sb.WriteString(fmt.Sprintf("%d. **%s**\n   From: %s\n   Date: %s\n   ID: %s | Thread: %s\n\n",
			i+1, subject, from, date, msg.Id, msg.ThreadId))
	}

	return TextResult(sb.String()), nil
}

// ReadMessage reads a full email message.
func (m *MailService) ReadMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	messageId, _ := req.GetArguments()["messageId"].(string)
	if messageId == "" {
		return ErrorResult(fmt.Errorf("messageId is required")), nil
	}

	msg, err := svc.Users.Messages.Get("me", messageId).Format("full").Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("reading message on %s: %w", acct.Label, err)), nil
	}

	return TextResult(formatMessage(msg, acct)), nil
}

// ReadThread reads all messages in a thread.
func (m *MailService) ReadThread(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	threadId, _ := req.GetArguments()["threadId"].(string)
	if threadId == "" {
		return ErrorResult(fmt.Errorf("threadId is required")), nil
	}

	thread, err := svc.Users.Threads.Get("me", threadId).Format("full").Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("reading thread on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Thread %s on %s (%s) — %d message(s):\n\n", threadId, acct.Label, acct.Email, len(thread.Messages)))
	for i, msg := range thread.Messages {
		sb.WriteString(fmt.Sprintf("--- Message %d/%d ---\n", i+1, len(thread.Messages)))
		sb.WriteString(formatMessage(msg, acct))
		sb.WriteString("\n")
	}

	return TextResult(sb.String()), nil
}

// CreateDraft creates an email draft.
func (m *MailService) CreateDraft(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	to, _ := req.GetArguments()["to"].(string)
	subject, _ := req.GetArguments()["subject"].(string)
	body, _ := req.GetArguments()["body"].(string)
	cc, _ := req.GetArguments()["cc"].(string)
	bcc, _ := req.GetArguments()["bcc"].(string)
	threadId, _ := req.GetArguments()["threadId"].(string)

	// Build RFC 2822 message
	var raw strings.Builder
	if to != "" {
		raw.WriteString(fmt.Sprintf("To: %s\r\n", to))
	}
	if cc != "" {
		raw.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	if bcc != "" {
		raw.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
	}
	if subject != "" {
		raw.WriteString(fmt.Sprintf("Subject: %s\r\n", subject))
	}
	raw.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	raw.WriteString("\r\n")
	raw.WriteString(body)

	encoded := base64.URLEncoding.EncodeToString([]byte(raw.String()))

	draft := &gmail.Draft{
		Message: &gmail.Message{
			Raw:      encoded,
			ThreadId: threadId,
		},
	}

	created, err := svc.Users.Drafts.Create("me", draft).Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("creating draft on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Draft created on %s (%s).\nDraft ID: %s\nTo: %s\nSubject: %s",
		acct.Label, acct.Email, created.Id, to, subject,
	)), nil
}

// ListLabels lists Gmail labels.
func (m *MailService) ListLabels(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.Users.Labels.List("me").Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("listing labels on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Labels on %s (%s):\n\n", acct.Label, acct.Email))
	for _, l := range resp.Labels {
		sb.WriteString(fmt.Sprintf("- %s (ID: %s, type: %s)\n", l.Name, l.Id, l.Type))
	}

	return TextResult(sb.String()), nil
}

// CreateLabel creates a new Gmail label.
func (m *MailService) CreateLabel(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	name, _ := req.GetArguments()["name"].(string)
	if name == "" {
		return ErrorResult(fmt.Errorf("name is required")), nil
	}

	label := &gmail.Label{
		Name:                    name,
		LabelListVisibility:     "labelShow",
		MessageListVisibility:   "show",
	}

	// Optional color
	bgColor, _ := req.GetArguments()["backgroundColor"].(string)
	textColor, _ := req.GetArguments()["textColor"].(string)
	if bgColor != "" || textColor != "" {
		label.Color = &gmail.LabelColor{
			BackgroundColor: bgColor,
			TextColor:       textColor,
		}
	}

	created, err := svc.Users.Labels.Create("me", label).Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("creating label on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Label created on %s (%s).\n  Name: %s\n  ID: %s",
		acct.Label, acct.Email, created.Name, created.Id,
	)), nil
}

// ModifyMessage adds or removes labels from a Gmail message.
func (m *MailService) ModifyMessage(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	messageId, _ := req.GetArguments()["messageId"].(string)
	if messageId == "" {
		return ErrorResult(fmt.Errorf("messageId is required")), nil
	}

	modReq := &gmail.ModifyMessageRequest{}

	if addLabels, ok := req.GetArguments()["addLabelIds"].([]any); ok {
		for _, l := range addLabels {
			if s, ok := l.(string); ok {
				modReq.AddLabelIds = append(modReq.AddLabelIds, s)
			}
		}
	}
	if removeLabels, ok := req.GetArguments()["removeLabelIds"].([]any); ok {
		for _, l := range removeLabels {
			if s, ok := l.(string); ok {
				modReq.RemoveLabelIds = append(modReq.RemoveLabelIds, s)
			}
		}
	}

	if len(modReq.AddLabelIds) == 0 && len(modReq.RemoveLabelIds) == 0 {
		return ErrorResult(fmt.Errorf("at least one of addLabelIds or removeLabelIds is required")), nil
	}

	msg, err := svc.Users.Messages.Modify("me", messageId, modReq).Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("modifying message on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Message %s modified on %s (%s).\n  Labels: %s",
		msg.Id, acct.Label, acct.Email, strings.Join(msg.LabelIds, ", "),
	)), nil
}

// GetProfile returns Gmail profile info.
func (m *MailService) GetProfile(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	profile, err := svc.Users.GetProfile("me").Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("getting profile for %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Profile for %s:\n  Email: %s\n  Total messages: %d\n  Total threads: %d\n  History ID: %d",
		acct.Label, profile.EmailAddress, profile.MessagesTotal, profile.ThreadsTotal, profile.HistoryId,
	)), nil
}

// --- helpers ---

func formatMessage(msg *gmail.Message, acct *accounts.Account) string {
	var sb strings.Builder

	from, to, subject, date := "", "", "", ""
	for _, h := range msg.Payload.Headers {
		switch h.Name {
		case "From":
			from = h.Value
		case "To":
			to = h.Value
		case "Subject":
			subject = h.Value
		case "Date":
			date = h.Value
		}
	}

	sb.WriteString(fmt.Sprintf("Account: %s (%s)\n", acct.Label, acct.Email))
	sb.WriteString(fmt.Sprintf("From: %s\nTo: %s\nSubject: %s\nDate: %s\n", from, to, subject, date))
	sb.WriteString(fmt.Sprintf("ID: %s | Thread: %s\n\n", msg.Id, msg.ThreadId))

	body := extractBody(msg.Payload)
	if body != "" {
		sb.WriteString(body)
	} else {
		sb.WriteString("[No text content]")
	}

	return sb.String()
}

func extractBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}

	// Collect all text/plain and text/html parts
	var plain, html string
	collectParts(payload, &plain, &html)

	if plain != "" {
		return plain
	}
	if html != "" {
		return htmlToText(html)
	}
	return ""
}

// collectParts recursively finds the first text/plain and text/html parts.
func collectParts(part *gmail.MessagePart, plain, html *string) {
	if part == nil {
		return
	}

	if part.Body != nil && part.Body.Data != "" {
		decoded, err := base64.URLEncoding.DecodeString(part.Body.Data)
		if err == nil {
			switch part.MimeType {
			case "text/plain":
				if *plain == "" {
					*plain = string(decoded)
				}
			case "text/html":
				if *html == "" {
					*html = string(decoded)
				}
			}
		}
	}

	for _, child := range part.Parts {
		collectParts(child, plain, html)
	}
}

// Precompiled regexes for HTML-to-text conversion.
var (
	reAnchor     = regexp.MustCompile(`(?i)<a\s[^>]*href\s*=\s*["']([^"']*)["'][^>]*>(.*?)</a>`)
	reBlock      = regexp.MustCompile(`(?i)</(p|div|tr|li|h[1-6]|blockquote)>`)
	reBr         = regexp.MustCompile(`(?i)<br\s*/?>`)
	reStyle      = regexp.MustCompile(`(?is)<style[^>]*>.*?</style>`)
	reScript     = regexp.MustCompile(`(?is)<script[^>]*>.*?</script>`)
	reTag        = regexp.MustCompile(`<[^>]+>`)
	reWhitespace = regexp.MustCompile(`[ \t]+`)
	reBlankLines = regexp.MustCompile(`\n{3,}`)
)

// htmlToText converts HTML to readable plain text, preserving link URLs.
func htmlToText(html string) string {
	// Remove style/script blocks
	s := reStyle.ReplaceAllString(html, "")
	s = reScript.ReplaceAllString(s, "")

	// Preserve links: <a href="url">text</a> → text (url)
	s = reAnchor.ReplaceAllStringFunc(s, func(match string) string {
		sub := reAnchor.FindStringSubmatch(match)
		if len(sub) < 3 {
			return match
		}
		href, text := sub[1], strings.TrimSpace(sub[2])
		// Strip any nested tags from link text
		text = reTag.ReplaceAllString(text, "")
		text = strings.TrimSpace(text)
		if text == "" || strings.EqualFold(text, href) {
			return href + " "
		}
		return text + " (" + href + ") "
	})

	// Block-level closing tags → newlines
	s = reBlock.ReplaceAllString(s, "\n")
	s = reBr.ReplaceAllString(s, "\n")

	// Strip remaining tags
	s = reTag.ReplaceAllString(s, "")

	// Decode common HTML entities
	s = strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&apos;", "'",
		"&nbsp;", " ",
	).Replace(s)

	// Collapse whitespace
	s = reWhitespace.ReplaceAllString(s, " ")
	// Trim each line
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = strings.TrimSpace(line)
	}
	s = strings.Join(lines, "\n")
	// Collapse excessive blank lines
	s = reBlankLines.ReplaceAllString(s, "\n\n")

	return strings.TrimSpace(s)
}

package services

import (
	"context"
	"encoding/base64"
	"fmt"
	"mime"
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
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "searching mail on %s: %w", acct.Label, err)), nil
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

	format, _ := req.GetArguments()["format"].(string)
	raw := strings.EqualFold(format, "raw")

	msg, err := svc.Users.Messages.Get("me", messageId).Format("full").Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "reading message on %s: %w", acct.Label, err)), nil
	}

	return TextResult(formatMessage(msg, acct, raw)), nil
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

	format, _ := req.GetArguments()["format"].(string)
	raw := strings.EqualFold(format, "raw")

	thread, err := svc.Users.Threads.Get("me", threadId).Format("full").Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "reading thread on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Thread %s on %s (%s) — %d message(s):\n\n", threadId, acct.Label, acct.Email, len(thread.Messages)))
	for i, msg := range thread.Messages {
		sb.WriteString(fmt.Sprintf("--- Message %d/%d ---\n", i+1, len(thread.Messages)))
		sb.WriteString(formatMessage(msg, acct, raw))
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
	contentType, _ := req.GetArguments()["contentType"].(string)

	if contentType == "" {
		// Auto-detect: if body looks like HTML, use text/html
		if strings.Contains(body, "<html") || strings.Contains(body, "<body") || strings.Contains(body, "<p>") {
			contentType = "text/html"
		} else {
			contentType = "text/plain"
		}
	}

	// Build RFC 2822 message
	var raw strings.Builder
	raw.WriteString("MIME-Version: 1.0\r\n")
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
		raw.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	}
	raw.WriteString(fmt.Sprintf("Content-Type: %s; charset=UTF-8\r\n", contentType))
	raw.WriteString("Content-Transfer-Encoding: base64\r\n")
	raw.WriteString("\r\n")
	raw.WriteString(base64.StdEncoding.EncodeToString([]byte(body)))

	encoded := base64.URLEncoding.EncodeToString([]byte(raw.String()))

	draft := &gmail.Draft{
		Message: &gmail.Message{
			Raw:      encoded,
			ThreadId: threadId,
		},
	}

	created, err := svc.Users.Drafts.Create("me", draft).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "creating draft on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Draft created on %s (%s).\nDraft ID: %s\nTo: %s\nSubject: %s",
		acct.Label, acct.Email, created.Id, to, subject,
	)), nil
}

// SendDraft sends an existing draft by its ID.
func (m *MailService) SendDraft(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	draftId, _ := req.GetArguments()["draftId"].(string)
	if draftId == "" {
		return ErrorResult(fmt.Errorf("draftId is required")), nil
	}

	msg, err := svc.Users.Drafts.Send("me", &gmail.Draft{Id: draftId}).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "sending draft on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Email sent from %s (%s).\nMessage ID: %s\nThread ID: %s",
		acct.Label, acct.Email, msg.Id, msg.ThreadId,
	)), nil
}

// Forward builds a forward DRAFT of an existing message (it does not send).
// The draft carries the original message's headers and body in the standard
// "---------- Forwarded message ---------" quoted block, optionally prepended
// with a note. The caller sends it with mail.send_draft.
//
// Attachments on the original message are listed by filename in the quoted
// block (so the recipient knows what was there) but the binary data is NOT
// re-attached — Gmail's forward re-encoding of arbitrary MIME attachments is
// a much larger surface. Use mail.get_attachment to pull an attachment's
// bytes if you need to act on it.
func (m *MailService) Forward(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	messageId, _ := req.GetArguments()["messageId"].(string)
	if messageId == "" {
		return ErrorResult(fmt.Errorf("messageId is required")), nil
	}
	to, _ := req.GetArguments()["to"].(string)
	if to == "" {
		return ErrorResult(fmt.Errorf("to is required")), nil
	}
	cc, _ := req.GetArguments()["cc"].(string)
	bcc, _ := req.GetArguments()["bcc"].(string)
	comment, _ := req.GetArguments()["comment"].(string)

	orig, err := svc.Users.Messages.Get("me", messageId).Format("full").Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "reading message to forward on %s: %w", acct.Label, err)), nil
	}

	origFrom, origTo, origSubject, origDate := "", "", "", ""
	for _, h := range orig.Payload.Headers {
		switch h.Name {
		case "From":
			origFrom = h.Value
		case "To":
			origTo = h.Value
		case "Subject":
			origSubject = h.Value
		case "Date":
			origDate = h.Value
		}
	}

	subject := origSubject
	if !hasForwardPrefix(subject) {
		subject = "Fwd: " + subject
	}

	// Build the forwarded body (plain text).
	var body strings.Builder
	if comment != "" {
		body.WriteString(comment)
		body.WriteString("\n\n")
	}
	body.WriteString("---------- Forwarded message ---------\n")
	body.WriteString(fmt.Sprintf("From: %s\n", origFrom))
	body.WriteString(fmt.Sprintf("Date: %s\n", origDate))
	body.WriteString(fmt.Sprintf("Subject: %s\n", origSubject))
	body.WriteString(fmt.Sprintf("To: %s\n", origTo))
	if atts := collectAttachments(orig.Payload); len(atts) > 0 {
		body.WriteString("Attachments (not re-attached — use mail.get_attachment):\n")
		for _, a := range atts {
			body.WriteString(fmt.Sprintf("  - %s (%s, %d bytes, attachmentId: %s)\n", a.Filename, a.MimeType, a.Size, a.AttachmentId))
		}
	}
	body.WriteString("\n")
	if orig := extractBody(orig.Payload); orig != "" {
		body.WriteString(orig)
	} else {
		body.WriteString("[No text content in original message]")
	}

	// Build RFC 2822 message (mirrors CreateDraft).
	var rawMsg strings.Builder
	rawMsg.WriteString("MIME-Version: 1.0\r\n")
	rawMsg.WriteString(fmt.Sprintf("To: %s\r\n", to))
	if cc != "" {
		rawMsg.WriteString(fmt.Sprintf("Cc: %s\r\n", cc))
	}
	if bcc != "" {
		rawMsg.WriteString(fmt.Sprintf("Bcc: %s\r\n", bcc))
	}
	rawMsg.WriteString(fmt.Sprintf("Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject)))
	rawMsg.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	rawMsg.WriteString("Content-Transfer-Encoding: base64\r\n")
	rawMsg.WriteString("\r\n")
	rawMsg.WriteString(base64.StdEncoding.EncodeToString([]byte(body.String())))

	encoded := base64.URLEncoding.EncodeToString([]byte(rawMsg.String()))

	draft := &gmail.Draft{
		Message: &gmail.Message{
			Raw:      encoded,
			ThreadId: orig.ThreadId,
		},
	}

	created, err := svc.Users.Drafts.Create("me", draft).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "creating forward draft on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Forward draft created on %s (%s).\nDraft ID: %s\nTo: %s\nSubject: %s\n\nThe email was NOT sent. Send it with mail.send_draft (draftId=%s).",
		acct.Label, acct.Email, created.Id, to, subject, created.Id,
	)), nil
}

// GetAttachment fetches a single attachment's bytes from a message. The raw
// data is base64url internally (Gmail's encoding); the returned JSON payload
// re-encodes it as standard base64 in "data_base64" alongside the filename,
// MIME type, and size read from the message's part metadata.
func (m *MailService) GetAttachment(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	messageId, _ := req.GetArguments()["messageId"].(string)
	if messageId == "" {
		return ErrorResult(fmt.Errorf("messageId is required")), nil
	}
	attachmentId, _ := req.GetArguments()["attachmentId"].(string)
	if attachmentId == "" {
		return ErrorResult(fmt.Errorf("attachmentId is required")), nil
	}

	// Read the message metadata to resolve the attachment's filename/mimeType.
	filename, mimeType := "", ""
	if msg, err := svc.Users.Messages.Get("me", messageId).Format("full").Do(); err == nil {
		for _, a := range collectAttachments(msg.Payload) {
			if a.AttachmentId == attachmentId {
				filename = a.Filename
				mimeType = a.MimeType
				break
			}
		}
	}

	att, err := svc.Users.Messages.Attachments.Get("me", messageId, attachmentId).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "getting attachment on %s: %w", acct.Label, err)), nil
	}

	decoded, err := base64.URLEncoding.DecodeString(att.Data)
	if err != nil {
		// Gmail may pad-strip; fall back to the raw-URL variant.
		decoded, err = base64.RawURLEncoding.DecodeString(att.Data)
		if err != nil {
			return ErrorResult(fmt.Errorf("decoding attachment data on %s: %w", acct.Label, err)), nil
		}
	}

	summary := fmt.Sprintf(
		"Fetched attachment from message %s on %s (%s).\n  Filename: %s\n  MIME type: %s\n  Size: %d bytes\n\n%s",
		messageId, acct.Label, acct.Email, orDash(filename), orDash(mimeType), len(decoded), UntrustedContentNote,
	)

	payload := map[string]any{
		"message_id":    messageId,
		"attachment_id": attachmentId,
		"filename":      filename,
		"mime_type":     mimeType,
		"size":          len(decoded),
		"data_base64":   base64.StdEncoding.EncodeToString(decoded),
	}
	return TextAndJSONResult(summary, payload), nil
}

// ListLabels lists Gmail labels.
func (m *MailService) ListLabels(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := m.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.Users.Labels.List("me").Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "listing labels on %s: %w", acct.Label, err)), nil
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
		Name:                  name,
		LabelListVisibility:   "labelShow",
		MessageListVisibility: "show",
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
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "creating label on %s: %w", acct.Label, err)), nil
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
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "modifying message on %s: %w", acct.Label, err)), nil
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
		return ErrorResult(scopeOrErr(acct, "Gmail", err, "getting profile for %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Profile for %s:\n  Email: %s\n  Total messages: %d\n  Total threads: %d\n  History ID: %d",
		acct.Label, profile.EmailAddress, profile.MessagesTotal, profile.ThreadsTotal, profile.HistoryId,
	)), nil
}

// --- helpers ---

func formatMessage(msg *gmail.Message, acct *accounts.Account, raw bool) string {
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
	sb.WriteString(fmt.Sprintf("ID: %s | Thread: %s\n", msg.Id, msg.ThreadId))

	// Surface attachments so agents know what to fetch with mail.get_attachment.
	if atts := collectAttachments(msg.Payload); len(atts) > 0 {
		sb.WriteString(fmt.Sprintf("Attachments (%d):\n", len(atts)))
		for _, a := range atts {
			sb.WriteString(fmt.Sprintf("  - %s (%s, %d bytes) attachmentId: %s\n", a.Filename, a.MimeType, a.Size, a.AttachmentId))
		}
	}
	sb.WriteString("\n")

	if raw {
		body := extractRawBody(msg.Payload)
		if body != "" {
			sb.WriteString(body)
		} else {
			sb.WriteString("[No text content]")
		}
	} else {
		body := extractBody(msg.Payload)
		if body != "" {
			sb.WriteString(body)
		} else {
			sb.WriteString("[No text content]")
		}
	}

	return sb.String()
}

// mailAttachment describes one attachment part of a Gmail message.
type mailAttachment struct {
	Filename     string
	MimeType     string
	AttachmentId string
	Size         int64
}

// collectAttachments walks the MIME tree and returns every part that is an
// attachment (has a filename and an external attachment ID). Inline text
// bodies (no filename, no attachmentId) are skipped.
func collectAttachments(payload *gmail.MessagePart) []mailAttachment {
	var out []mailAttachment
	var walk func(part *gmail.MessagePart)
	walk = func(part *gmail.MessagePart) {
		if part == nil {
			return
		}
		if part.Filename != "" && part.Body != nil && part.Body.AttachmentId != "" {
			out = append(out, mailAttachment{
				Filename:     part.Filename,
				MimeType:     part.MimeType,
				AttachmentId: part.Body.AttachmentId,
				Size:         part.Body.Size,
			})
		}
		for _, child := range part.Parts {
			walk(child)
		}
	}
	walk(payload)
	return out
}

// hasForwardPrefix reports whether subject already carries a forward prefix,
// so Forward doesn't stack "Fwd: Fwd:".
func hasForwardPrefix(subject string) bool {
	s := strings.ToLower(strings.TrimSpace(subject))
	return strings.HasPrefix(s, "fwd:") || strings.HasPrefix(s, "fw:")
}

// orDash returns "-" for an empty string, so summaries never render a blank field.
func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

// extractRawBody returns the original body without conversion — prefers HTML, falls back to plain text.
func extractRawBody(payload *gmail.MessagePart) string {
	if payload == nil {
		return ""
	}

	var plain, html string
	collectParts(payload, &plain, &html)

	if html != "" {
		return html
	}
	return plain
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

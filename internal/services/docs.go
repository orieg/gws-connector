package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	docs "google.golang.org/api/docs/v1"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// DocsService implements Google Docs MCP tools.
type DocsService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewDocsService creates a new docs service.
func NewDocsService(router *accounts.Router, clients *auth.ClientFactory) *DocsService {
	return &DocsService{router: router, clients: clients}
}

func (d *DocsService) resolveAndGetService(ctx context.Context, args map[string]any) (*docs.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := d.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := d.clients.DocsService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// Read reads a document's plain-text content. The structured Docs tree is
// included in the JSON payload for callers that need it; the primary text
// block is flat plain text so read/write round-trips are lossless-ish with
// the insert_text and replace_text tools.
func (d *DocsService) Read(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	documentID, _ := req.GetArguments()["document_id"].(string)
	if documentID == "" {
		return ErrorResult(fmt.Errorf("document_id is required")), nil
	}

	doc, err := svc.Documents.Get(documentID).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Docs", err, "reading document on %s: %w", acct.Label, err)), nil
	}

	plain := extractDocPlainText(doc)

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"Read document %q on %s (%s).\n  ID: %s\n\n",
		doc.Title, acct.Label, acct.Email, doc.DocumentId))
	sb.WriteString(fmt.Sprintf(
		"<untrusted-document-content account=%q source=\"docs/%s\">\n",
		acct.Label, documentID))
	sb.WriteString(plain)
	sb.WriteString("\n</untrusted-document-content>")

	payload := map[string]any{
		"document_id":  doc.DocumentId,
		"title":        doc.Title,
		"plain_text":   plain,
		"revision_id":  doc.RevisionId,
		"content_tree": doc.Body, // raw structural tree for callers that want it
	}
	return TextAndJSONResult(sb.String(), payload), nil
}

// InsertText inserts literal text at a location in the document. location is
// either "end" (append to end of body) or a 1-based integer index that the
// tool converts to the Docs API's 0-based UTF-16 index.
func (d *DocsService) InsertText(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	documentID, _ := req.GetArguments()["document_id"].(string)
	if documentID == "" {
		return ErrorResult(fmt.Errorf("document_id is required")), nil
	}
	text, _ := req.GetArguments()["text"].(string)
	if text == "" {
		return ErrorResult(fmt.Errorf("text is required (non-empty)")), nil
	}

	location, _ := req.GetArguments()["location"].(string)
	if location == "" {
		location = "end"
	}

	insertReq := &docs.InsertTextRequest{Text: text}
	switch {
	case strings.EqualFold(location, "end"):
		insertReq.EndOfSegmentLocation = &docs.EndOfSegmentLocation{}
	default:
		idx, perr := parse1BasedIndex(location)
		if perr != nil {
			return ErrorResult(fmt.Errorf("location: %w", perr)), nil
		}
		insertReq.Location = &docs.Location{Index: idx}
	}

	body := &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{InsertText: insertReq}},
	}
	if _, err := svc.Documents.BatchUpdate(documentID, body).Do(); err != nil {
		return ErrorResult(scopeOrErr(acct, "Docs", err, "inserting text on %s: %w", acct.Label, err)), nil
	}

	summary := fmt.Sprintf(
		"Inserted %d character(s) into document %s on %s (%s) at %s.",
		len([]rune(text)), documentID, acct.Label, acct.Email, location)

	payload := map[string]any{
		"document_id":       documentID,
		"inserted_characters": len([]rune(text)),
		"location":          location,
	}
	return TextAndJSONResult(summary, payload), nil
}

// ReplaceText replaces all occurrences of a literal string with another. No
// regex support (to keep the tool contract simple and unsurprising for
// agents). match_case defaults to true.
func (d *DocsService) ReplaceText(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	documentID, _ := req.GetArguments()["document_id"].(string)
	if documentID == "" {
		return ErrorResult(fmt.Errorf("document_id is required")), nil
	}
	find, _ := req.GetArguments()["find"].(string)
	if find == "" {
		return ErrorResult(fmt.Errorf("find is required (non-empty)")), nil
	}
	replace, hasReplace := req.GetArguments()["replace"].(string)
	if !hasReplace {
		return ErrorResult(fmt.Errorf("replace is required (may be an empty string to delete matches)")), nil
	}

	matchCase := true
	if v, ok := req.GetArguments()["match_case"].(bool); ok {
		matchCase = v
	}

	body := &docs.BatchUpdateDocumentRequest{
		Requests: []*docs.Request{{
			ReplaceAllText: &docs.ReplaceAllTextRequest{
				ContainsText: &docs.SubstringMatchCriteria{
					Text:          find,
					MatchCase:     matchCase,
					SearchByRegex: false,
				},
				ReplaceText: replace,
			},
		}},
	}

	resp, err := svc.Documents.BatchUpdate(documentID, body).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Docs", err, "replacing text on %s: %w", acct.Label, err)), nil
	}

	var replaced int64
	for _, r := range resp.Replies {
		if r.ReplaceAllText != nil {
			replaced += r.ReplaceAllText.OccurrencesChanged
		}
	}

	summary := fmt.Sprintf(
		"Replaced %d occurrence(s) in document %s on %s (%s).",
		replaced, documentID, acct.Label, acct.Email)

	payload := map[string]any{
		"document_id":         documentID,
		"occurrences_changed": replaced,
		"find":                find,
		"replace":             replace,
		"match_case":          matchCase,
	}
	return TextAndJSONResult(summary, payload), nil
}

// Create creates a new document. Optional initial_text is inserted at the
// start after creation (best-effort; creation errors still hide a failed
// seed behind a warning so the caller knows the doc exists).
func (d *DocsService) Create(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := d.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	title, _ := req.GetArguments()["title"].(string)
	if title == "" {
		return ErrorResult(fmt.Errorf("title is required")), nil
	}

	doc, err := svc.Documents.Create(&docs.Document{Title: title}).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Docs", err, "creating document on %s: %w", acct.Label, err)), nil
	}

	var seedErr error
	if initial, ok := req.GetArguments()["initial_text"].(string); ok && initial != "" {
		seedBody := &docs.BatchUpdateDocumentRequest{
			Requests: []*docs.Request{{
				InsertText: &docs.InsertTextRequest{
					Text:                 initial,
					EndOfSegmentLocation: &docs.EndOfSegmentLocation{},
				},
			}},
		}
		if _, err := svc.Documents.BatchUpdate(doc.DocumentId, seedBody).Do(); err != nil {
			seedErr = err
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"Created document %q on %s (%s).\n  ID: %s\n  URL: https://docs.google.com/document/d/%s/edit",
		doc.Title, acct.Label, acct.Email, doc.DocumentId, doc.DocumentId))
	if seedErr != nil {
		sb.WriteString(fmt.Sprintf(
			"\n  WARNING: document created but initial_text insert failed: %v", seedErr))
	}

	payload := map[string]any{
		"document_id": doc.DocumentId,
		"title":       doc.Title,
		"url":         fmt.Sprintf("https://docs.google.com/document/d/%s/edit", doc.DocumentId),
	}
	if seedErr != nil {
		payload["seed_error"] = seedErr.Error()
	}
	return TextAndJSONResult(sb.String(), payload), nil
}

// --- helpers ---

// extractDocPlainText walks the document body tree and returns a flat
// plain-text rendering. Preserves newlines between paragraphs; strips
// everything else (styles, tables, inline objects). Good enough for agent
// reads and for round-tripping with insert_text / replace_text.
func extractDocPlainText(doc *docs.Document) string {
	if doc == nil || doc.Body == nil {
		return ""
	}
	var sb strings.Builder
	for _, el := range doc.Body.Content {
		appendElementText(&sb, el)
	}
	return strings.TrimRight(sb.String(), "\n")
}

func appendElementText(sb *strings.Builder, el *docs.StructuralElement) {
	if el == nil {
		return
	}
	switch {
	case el.Paragraph != nil:
		for _, pe := range el.Paragraph.Elements {
			if pe.TextRun != nil {
				sb.WriteString(pe.TextRun.Content)
			}
		}
	case el.Table != nil:
		for _, row := range el.Table.TableRows {
			for _, cell := range row.TableCells {
				for _, inner := range cell.Content {
					appendElementText(sb, inner)
				}
			}
		}
	case el.TableOfContents != nil:
		for _, inner := range el.TableOfContents.Content {
			appendElementText(sb, inner)
		}
	}
}

// parse1BasedIndex parses a 1-based string index ("1", "42") and returns
// the Docs API 0-based UTF-16 index it corresponds to. Preserves the
// 1-based contract in error messages so callers aren't confused.
func parse1BasedIndex(s string) (int64, error) {
	var n int64
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0, fmt.Errorf(
			"must be 'end' or a positive 1-based integer index (got %q)", s)
	}
	if n < 1 {
		return 0, fmt.Errorf(
			"must be 'end' or a positive 1-based integer index (got %d)", n)
	}
	return n - 1, nil
}

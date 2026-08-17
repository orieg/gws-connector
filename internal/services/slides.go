package services

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	slides "google.golang.org/api/slides/v1"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// SlidesService implements Google Slides MCP tools.
type SlidesService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewSlidesService creates a new slides service.
func NewSlidesService(router *accounts.Router, clients *auth.ClientFactory) *SlidesService {
	return &SlidesService{router: router, clients: clients}
}

func (s *SlidesService) resolveAndGetService(ctx context.Context, args map[string]any) (*slides.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := s.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := s.clients.SlidesService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// Get reads a presentation: slide count plus a per-slide plain-text summary.
// The raw slides tree is included in the JSON payload for callers that need
// the full structure.
func (s *SlidesService) Get(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := s.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	presentationID, _ := req.GetArguments()["presentation_id"].(string)
	if presentationID == "" {
		return ErrorResult(fmt.Errorf("presentation_id is required")), nil
	}

	pres, err := svc.Presentations.Get(presentationID).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Slides", err, "reading presentation on %s: %w", acct.Label, err)), nil
	}

	slideSummaries := make([]map[string]any, 0, len(pres.Slides))
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf(
		"Read presentation %q on %s (%s).\n  ID: %s\n  Slides: %d\n\n",
		pres.Title, acct.Label, acct.Email, pres.PresentationId, len(pres.Slides)))
	sb.WriteString(fmt.Sprintf(
		"<untrusted-document-content account=%q source=\"slides/%s\">\n",
		acct.Label, presentationID))
	for i, slide := range pres.Slides {
		text := extractSlideText(slide)
		sb.WriteString(fmt.Sprintf("--- Slide %d/%d (id: %s) ---\n", i+1, len(pres.Slides), slide.ObjectId))
		if text != "" {
			sb.WriteString(text)
			sb.WriteString("\n")
		} else {
			sb.WriteString("[No text content]\n")
		}
		slideSummaries = append(slideSummaries, map[string]any{
			"index":     i,
			"object_id": slide.ObjectId,
			"text":      text,
		})
	}
	sb.WriteString("</untrusted-document-content>")

	payload := map[string]any{
		"presentation_id": pres.PresentationId,
		"title":           pres.Title,
		"revision_id":     pres.RevisionId,
		"slide_count":     len(pres.Slides),
		"slides":          slideSummaries,
		"raw_slides":      pres.Slides, // full structural tree for callers that want it
	}
	return TextAndJSONResult(sb.String(), payload), nil
}

// Create creates a new, empty presentation with the given title and returns
// its ID and shareable URL. Additive — it never touches existing content.
func (s *SlidesService) Create(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := s.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	title, _ := req.GetArguments()["title"].(string)
	if title == "" {
		return ErrorResult(fmt.Errorf("title is required")), nil
	}

	pres, err := svc.Presentations.Create(&slides.Presentation{Title: title}).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Slides", err, "creating presentation on %s: %w", acct.Label, err)), nil
	}

	url := fmt.Sprintf("https://docs.google.com/presentation/d/%s/edit", pres.PresentationId)
	summary := fmt.Sprintf(
		"Created presentation %q on %s (%s).\n  ID: %s\n  URL: %s",
		pres.Title, acct.Label, acct.Email, pres.PresentationId, url)

	payload := map[string]any{
		"presentation_id": pres.PresentationId,
		"title":           pres.Title,
		"url":             url,
	}
	return TextAndJSONResult(summary, payload), nil
}

// BatchUpdate applies a raw list of Slides API requests to an existing
// presentation. requests is a JSON array of Slides API Request objects
// (e.g. createSlide, insertText, deleteObject). This is a powerful,
// content-modifying operation — the caller is responsible for constructing
// valid requests per the Slides API reference.
func (s *SlidesService) BatchUpdate(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := s.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	presentationID, _ := req.GetArguments()["presentation_id"].(string)
	if presentationID == "" {
		return ErrorResult(fmt.Errorf("presentation_id is required")), nil
	}

	rawRequests, ok := req.GetArguments()["requests"]
	if !ok || rawRequests == nil {
		return ErrorResult(fmt.Errorf("requests is required (a JSON array of Slides API request objects)")), nil
	}

	// The tool contract accepts a JSON array of Slides API Request objects.
	// Re-marshal the decoded arguments and unmarshal into typed requests so
	// callers can pass the exact Slides API request shapes.
	reqBytes, err := json.Marshal(rawRequests)
	if err != nil {
		return ErrorResult(fmt.Errorf("encoding requests: %w", err)), nil
	}
	var requests []*slides.Request
	if err := json.Unmarshal(reqBytes, &requests); err != nil {
		return ErrorResult(fmt.Errorf("requests must be a JSON array of Slides API request objects: %w", err)), nil
	}
	if len(requests) == 0 {
		return ErrorResult(fmt.Errorf("requests must contain at least one request")), nil
	}

	resp, err := svc.Presentations.BatchUpdate(presentationID, &slides.BatchUpdatePresentationRequest{
		Requests: requests,
	}).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Slides", err, "updating presentation on %s: %w", acct.Label, err)), nil
	}

	summary := fmt.Sprintf(
		"Applied %d request(s) to presentation %s on %s (%s).",
		len(requests), presentationID, acct.Label, acct.Email)

	payload := map[string]any{
		"presentation_id":  resp.PresentationId,
		"requests_applied": len(requests),
		"replies":          resp.Replies,
	}
	return TextAndJSONResult(summary, payload), nil
}

// --- helpers ---

// extractSlideText walks a slide's page elements and returns a flat plain-text
// rendering (text boxes/shapes and table cells). Non-text elements (images,
// lines, charts) are skipped. Good enough for agent reads.
func extractSlideText(slide *slides.Page) string {
	if slide == nil {
		return ""
	}
	var lines []string
	for _, el := range slide.PageElements {
		if t := extractElementText(el); t != "" {
			lines = append(lines, t)
		}
	}
	return strings.Join(lines, "\n")
}

func extractElementText(el *slides.PageElement) string {
	if el == nil {
		return ""
	}
	var sb strings.Builder
	if el.Shape != nil && el.Shape.Text != nil {
		appendTextContent(&sb, el.Shape.Text)
	}
	if el.Table != nil {
		for _, row := range el.Table.TableRows {
			for _, cell := range row.TableCells {
				if cell.Text != nil {
					appendTextContent(&sb, cell.Text)
				}
			}
		}
	}
	if el.ElementGroup != nil {
		for _, child := range el.ElementGroup.Children {
			if t := extractElementText(child); t != "" {
				sb.WriteString(t)
				sb.WriteString("\n")
			}
		}
	}
	return strings.TrimRight(sb.String(), "\n")
}

func appendTextContent(sb *strings.Builder, tc *slides.TextContent) {
	if tc == nil {
		return
	}
	for _, te := range tc.TextElements {
		if te.TextRun != nil {
			sb.WriteString(te.TextRun.Content)
		}
	}
}

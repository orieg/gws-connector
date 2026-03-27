package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"google.golang.org/api/calendar/v3"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// CalendarService implements Calendar-related MCP tools.
type CalendarService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewCalendarService creates a new calendar service.
func NewCalendarService(router *accounts.Router, clients *auth.ClientFactory) *CalendarService {
	return &CalendarService{router: router, clients: clients}
}

func (c *CalendarService) resolveAndGetService(ctx context.Context, args map[string]any) (*calendar.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := c.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := c.clients.CalendarService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// ListEvents lists calendar events in a time range.
func (c *CalendarService) ListEvents(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := c.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	timeMin, _ := req.GetArguments()["timeMin"].(string)
	timeMax, _ := req.GetArguments()["timeMax"].(string)
	calendarId, _ := req.GetArguments()["calendarId"].(string)
	query, _ := req.GetArguments()["q"].(string)
	maxResults := int64(50)
	if mr, ok := req.GetArguments()["maxResults"].(float64); ok && mr > 0 {
		maxResults = int64(mr)
	}
	if maxResults > 250 {
		maxResults = 250
	}

	if calendarId == "" {
		calendarId = "primary"
	}

	call := svc.Events.List(calendarId).
		TimeMin(timeMin).
		TimeMax(timeMax).
		MaxResults(maxResults).
		SingleEvents(true).
		OrderBy("startTime")

	if query != "" {
		call = call.Q(query)
	}

	resp, err := call.Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("listing events on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Items) == 0 {
		return TextResult(fmt.Sprintf("No events found on %s (%s) for the given range.", acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Events on %s (%s) — %d found:\n\n", acct.Label, acct.Email, len(resp.Items)))

	for i, event := range resp.Items {
		start := event.Start.DateTime
		if start == "" {
			start = event.Start.Date + " (all day)"
		}
		end := event.End.DateTime
		if end == "" {
			end = event.End.Date
		}

		sb.WriteString(fmt.Sprintf("%d. **%s**\n", i+1, event.Summary))
		sb.WriteString(fmt.Sprintf("   Start: %s\n   End: %s\n", start, end))
		if event.Location != "" {
			sb.WriteString(fmt.Sprintf("   Location: %s\n", event.Location))
		}
		if event.Description != "" {
			desc := event.Description
			if len(desc) > 200 {
				desc = desc[:200] + "..."
			}
			sb.WriteString(fmt.Sprintf("   Description: %s\n", desc))
		}
		sb.WriteString(fmt.Sprintf("   ID: %s\n\n", event.Id))
	}

	return TextResult(sb.String()), nil
}

// GetEvent gets a single event's details.
func (c *CalendarService) GetEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := c.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	eventId, _ := req.GetArguments()["eventId"].(string)
	calendarId, _ := req.GetArguments()["calendarId"].(string)
	if eventId == "" {
		return ErrorResult(fmt.Errorf("eventId is required")), nil
	}
	if calendarId == "" {
		calendarId = "primary"
	}

	event, err := svc.Events.Get(calendarId, eventId).Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("getting event on %s: %w", acct.Label, err)), nil
	}

	return TextResult(formatEvent(event, acct)), nil
}

// CreateEvent creates a new calendar event.
func (c *CalendarService) CreateEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := c.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	summary, _ := req.GetArguments()["summary"].(string)
	start, _ := req.GetArguments()["start"].(string)
	end, _ := req.GetArguments()["end"].(string)
	description, _ := req.GetArguments()["description"].(string)
	location, _ := req.GetArguments()["location"].(string)
	calendarId, _ := req.GetArguments()["calendarId"].(string)

	if calendarId == "" {
		calendarId = "primary"
	}

	event := &calendar.Event{
		Summary:     summary,
		Description: description,
		Location:    location,
		Start: &calendar.EventDateTime{
			DateTime: start,
		},
		End: &calendar.EventDateTime{
			DateTime: end,
		},
	}

	created, err := svc.Events.Insert(calendarId, event).Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("creating event on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Event created on %s (%s):\n  Title: %s\n  Start: %s\n  End: %s\n  ID: %s\n  Link: %s",
		acct.Label, acct.Email, created.Summary,
		created.Start.DateTime, created.End.DateTime,
		created.Id, created.HtmlLink,
	)), nil
}

// ListCalendars lists all calendars for the account.
func (c *CalendarService) ListCalendars(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := c.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.CalendarList.List().Do()
	if err != nil {
		return ErrorResult(fmt.Errorf("listing calendars on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Calendars on %s (%s):\n\n", acct.Label, acct.Email))
	for i, cal := range resp.Items {
		primary := ""
		if cal.Primary {
			primary = " [PRIMARY]"
		}
		sb.WriteString(fmt.Sprintf("%d. %s%s\n   ID: %s\n   Access: %s\n\n",
			i+1, cal.Summary, primary, cal.Id, cal.AccessRole))
	}

	return TextResult(sb.String()), nil
}

func formatEvent(event *calendar.Event, acct *accounts.Account) string {
	var sb strings.Builder

	start := event.Start.DateTime
	if start == "" {
		start = event.Start.Date + " (all day)"
	}
	end := event.End.DateTime
	if end == "" {
		end = event.End.Date
	}

	sb.WriteString(fmt.Sprintf("Account: %s (%s)\n", acct.Label, acct.Email))
	sb.WriteString(fmt.Sprintf("Title: %s\n", event.Summary))
	sb.WriteString(fmt.Sprintf("Start: %s\nEnd: %s\n", start, end))
	if event.Location != "" {
		sb.WriteString(fmt.Sprintf("Location: %s\n", event.Location))
	}
	if event.Description != "" {
		sb.WriteString(fmt.Sprintf("Description: %s\n", event.Description))
	}
	sb.WriteString(fmt.Sprintf("Status: %s\n", event.Status))
	sb.WriteString(fmt.Sprintf("ID: %s\n", event.Id))
	if event.HtmlLink != "" {
		sb.WriteString(fmt.Sprintf("Link: %s\n", event.HtmlLink))
	}

	if len(event.Attendees) > 0 {
		sb.WriteString(fmt.Sprintf("\nAttendees (%d):\n", len(event.Attendees)))
		for _, a := range event.Attendees {
			name := a.DisplayName
			if name == "" {
				name = a.Email
			}
			sb.WriteString(fmt.Sprintf("  - %s (%s)\n", name, a.ResponseStatus))
		}
	}

	return sb.String()
}

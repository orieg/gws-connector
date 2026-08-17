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
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "listing events on %s: %w", acct.Label, err)), nil
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
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "getting event on %s: %w", acct.Label, err)), nil
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
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "creating event on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Event created on %s (%s):\n  Title: %s\n  Start: %s\n  End: %s\n  ID: %s\n  Link: %s",
		acct.Label, acct.Email, created.Summary,
		created.Start.DateTime, created.End.DateTime,
		created.Id, created.HtmlLink,
	)), nil
}

// UpdateEvent updates an existing calendar event using patch semantics.
// Only the fields provided in the request are changed; all other fields on
// the event are left untouched.
func (c *CalendarService) UpdateEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	eventId, _ := args["eventId"].(string)
	if eventId == "" {
		return ErrorResult(fmt.Errorf("eventId is required")), nil
	}

	svc, acct, err := c.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	calendarId, _ := args["calendarId"].(string)
	if calendarId == "" {
		calendarId = "primary"
	}

	// Build a sparse patch: only set fields that were actually provided so
	// unspecified fields remain untouched by the Events.Patch call.
	patch := &calendar.Event{}
	if summary, ok := args["summary"].(string); ok {
		patch.Summary = summary
	}
	if description, ok := args["description"].(string); ok {
		patch.Description = description
	}
	if location, ok := args["location"].(string); ok {
		patch.Location = location
	}
	if start, ok := args["start"].(string); ok && start != "" {
		patch.Start = &calendar.EventDateTime{DateTime: start}
	}
	if end, ok := args["end"].(string); ok && end != "" {
		patch.End = &calendar.EventDateTime{DateTime: end}
	}

	updated, err := svc.Events.Patch(calendarId, eventId, patch).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "updating event on %s: %w", acct.Label, err)), nil
	}

	start := updated.Start.DateTime
	if start == "" {
		start = updated.Start.Date + " (all day)"
	}
	end := updated.End.DateTime
	if end == "" {
		end = updated.End.Date
	}

	return TextResult(fmt.Sprintf(
		"Event updated on %s (%s):\n  Title: %s\n  Start: %s\n  End: %s\n  ID: %s\n  Link: %s",
		acct.Label, acct.Email, updated.Summary,
		start, end, updated.Id, updated.HtmlLink,
	)), nil
}

// DeleteEvent deletes (cancels) a calendar event.
func (c *CalendarService) DeleteEvent(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	eventId, _ := args["eventId"].(string)
	if eventId == "" {
		return ErrorResult(fmt.Errorf("eventId is required")), nil
	}

	svc, acct, err := c.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	calendarId, _ := args["calendarId"].(string)
	if calendarId == "" {
		calendarId = "primary"
	}

	if err := svc.Events.Delete(calendarId, eventId).Do(); err != nil {
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "deleting event on %s: %w", acct.Label, err)), nil
	}

	return TextResult(fmt.Sprintf(
		"Event deleted on %s (%s):\n  Calendar: %s\n  ID: %s",
		acct.Label, acct.Email, calendarId, eventId,
	)), nil
}

// FreeBusy queries free/busy information for one or more calendars in a range.
func (c *CalendarService) FreeBusy(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	timeMin, _ := args["timeMin"].(string)
	timeMax, _ := args["timeMax"].(string)
	if timeMin == "" || timeMax == "" {
		return ErrorResult(fmt.Errorf("timeMin and timeMax are required (RFC3339)")), nil
	}

	svc, acct, err := c.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	var calendarIds []string
	if raw, ok := args["calendarIds"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok && s != "" {
				calendarIds = append(calendarIds, s)
			}
		}
	}
	if len(calendarIds) == 0 {
		calendarIds = []string{"primary"}
	}

	reqBody := &calendar.FreeBusyRequest{
		TimeMin: timeMin,
		TimeMax: timeMax,
	}
	for _, id := range calendarIds {
		reqBody.Items = append(reqBody.Items, &calendar.FreeBusyRequestItem{Id: id})
	}

	resp, err := svc.Freebusy.Query(reqBody).Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "querying free/busy on %s: %w", acct.Label, err)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Free/busy on %s (%s) from %s to %s:\n\n", acct.Label, acct.Email, timeMin, timeMax))
	for _, id := range calendarIds {
		cal, ok := resp.Calendars[id]
		sb.WriteString(fmt.Sprintf("Calendar: %s\n", id))
		if !ok {
			sb.WriteString("  (no data returned)\n\n")
			continue
		}
		if len(cal.Errors) > 0 {
			for _, e := range cal.Errors {
				sb.WriteString(fmt.Sprintf("  Error: %s\n", e.Reason))
			}
		}
		if len(cal.Busy) == 0 {
			sb.WriteString("  Free for the entire range.\n\n")
			continue
		}
		sb.WriteString(fmt.Sprintf("  Busy (%d):\n", len(cal.Busy)))
		for _, b := range cal.Busy {
			sb.WriteString(fmt.Sprintf("    %s → %s\n", b.Start, b.End))
		}
		sb.WriteString("\n")
	}

	return TextResult(sb.String()), nil
}

// ListCalendars lists all calendars for the account.
func (c *CalendarService) ListCalendars(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	svc, acct, err := c.resolveAndGetService(ctx, req.GetArguments())
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.CalendarList.List().Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Calendar", err, "listing calendars on %s: %w", acct.Label, err)), nil
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

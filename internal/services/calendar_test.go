package services

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func calReq(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// UpdateEvent must reject a missing eventId before touching any account or
// API service, so a nil-dependency service is enough to exercise validation.
func TestUpdateEvent_MissingEventId(t *testing.T) {
	c := NewCalendarService(nil, nil)
	res, err := c.UpdateEvent(context.Background(), calReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing eventId")
	}
	if !strings.Contains(firstText(res), "eventId is required") {
		t.Errorf("expected 'eventId is required', got: %s", firstText(res))
	}
}

func TestDeleteEvent_MissingEventId(t *testing.T) {
	c := NewCalendarService(nil, nil)
	res, err := c.DeleteEvent(context.Background(), calReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing eventId")
	}
	if !strings.Contains(firstText(res), "eventId is required") {
		t.Errorf("expected 'eventId is required', got: %s", firstText(res))
	}
}

func TestFreeBusy_MissingRange(t *testing.T) {
	c := NewCalendarService(nil, nil)
	res, err := c.FreeBusy(context.Background(), calReq(map[string]any{"timeMin": "2026-01-01T00:00:00Z"}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing timeMax")
	}
	if !strings.Contains(firstText(res), "timeMin and timeMax are required") {
		t.Errorf("expected range-required error, got: %s", firstText(res))
	}
}

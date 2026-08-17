package services

import (
	"context"
	"strings"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
	people "google.golang.org/api/people/v1"
)

func contactsReq(args map[string]any) mcp.CallToolRequest {
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// Search must reject a missing/blank query before touching any account or API
// service, so a nil-dependency service is enough to exercise validation.
func TestContactsSearch_MissingQuery(t *testing.T) {
	c := NewContactsService(nil, nil)
	res, err := c.Search(context.Background(), contactsReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing query")
	}
	if !strings.Contains(firstText(res), "query is required") {
		t.Errorf("expected 'query is required', got: %s", firstText(res))
	}
}

func TestContactsSearch_BlankQuery(t *testing.T) {
	c := NewContactsService(nil, nil)
	res, err := c.Search(context.Background(), contactsReq(map[string]any{"query": "   "}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for blank query")
	}
	if !strings.Contains(firstText(res), "query is required") {
		t.Errorf("expected 'query is required', got: %s", firstText(res))
	}
}

func TestContactsDirectorySearch_MissingQuery(t *testing.T) {
	c := NewContactsService(nil, nil)
	res, err := c.DirectorySearch(context.Background(), contactsReq(map[string]any{}))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Fatal("expected error result for missing query")
	}
	if !strings.Contains(firstText(res), "query is required") {
		t.Errorf("expected 'query is required', got: %s", firstText(res))
	}
}

func TestFormatPerson(t *testing.T) {
	p := &people.Person{
		Names:          []*people.Name{{DisplayName: "Ada Lovelace"}},
		EmailAddresses: []*people.EmailAddress{{Value: "ada@example.com"}, {Value: "ada.l@work.com"}},
		PhoneNumbers:   []*people.PhoneNumber{{Value: "+1 555 0100"}},
	}
	out := formatPerson(p)
	for _, want := range []string{"Ada Lovelace", "ada@example.com", "ada.l@work.com", "+1 555 0100", "Emails:", "Phones:"} {
		if !strings.Contains(out, want) {
			t.Errorf("formatPerson output missing %q; got:\n%s", want, out)
		}
	}
}

func TestFormatPerson_NoName(t *testing.T) {
	p := &people.Person{
		EmailAddresses: []*people.EmailAddress{{Value: "anon@example.com"}},
	}
	out := formatPerson(p)
	if !strings.Contains(out, "(no name)") {
		t.Errorf("expected '(no name)' placeholder, got:\n%s", out)
	}
	if strings.Contains(out, "Phones:") {
		t.Errorf("did not expect a Phones line when no phone numbers present, got:\n%s", out)
	}
}

func TestFormatPerson_Nil(t *testing.T) {
	if out := formatPerson(nil); !strings.Contains(out, "no details") {
		t.Errorf("expected nil person to render '(no details)', got: %q", out)
	}
}

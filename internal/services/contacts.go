package services

import (
	"context"
	"fmt"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	people "google.golang.org/api/people/v1"

	"github.com/orieg/gws-connector/internal/accounts"
	"github.com/orieg/gws-connector/internal/auth"
)

// ContactsService implements People-API-backed contact lookup MCP tools.
// All methods are read-only: the connector searches contacts and the org
// directory, it never mutates them.
type ContactsService struct {
	router  *accounts.Router
	clients *auth.ClientFactory
}

// NewContactsService creates a new contacts service.
func NewContactsService(router *accounts.Router, clients *auth.ClientFactory) *ContactsService {
	return &ContactsService{router: router, clients: clients}
}

func (c *ContactsService) resolveAndGetService(ctx context.Context, args map[string]any) (*people.Service, *accounts.Account, error) {
	accountParam, _ := args["account"].(string)
	acct, err := c.router.Resolve(accountParam)
	if err != nil {
		return nil, nil, err
	}
	svc, err := c.clients.PeopleService(ctx, acct.Email)
	if err != nil {
		return nil, nil, err
	}
	return svc, acct, nil
}

// Search searches the account's own contacts (People API SearchContacts).
// Matches against name, nickname, email, and phone. Returns each match's
// display name, email addresses, and phone numbers.
func (c *ContactsService) Search(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return ErrorResult(fmt.Errorf("query is required")), nil
	}

	svc, acct, err := c.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	// SearchContacts caps pageSize at 30.
	maxResults := int64(20)
	if mr, ok := args["maxResults"].(float64); ok && mr > 0 {
		maxResults = int64(mr)
	}
	if maxResults > 30 {
		maxResults = 30
	}

	resp, err := svc.People.SearchContacts().
		Query(query).
		PageSize(maxResults).
		ReadMask("names,emailAddresses,phoneNumbers").
		Do()
	if err != nil {
		return ErrorResult(scopeOrErr(acct, "Contacts", err,
			"searching contacts on %s: %w", acct.Label, err)), nil
	}

	if len(resp.Results) == 0 {
		return TextResult(fmt.Sprintf(
			"No contacts matching %q found on %s (%s).", query, acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Contacts on %s (%s) matching %q — %d found:\n\n",
		acct.Label, acct.Email, query, len(resp.Results)))
	for i, r := range resp.Results {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, formatPerson(r.Person)))
	}

	return TextResult(sb.String()), nil
}

// DirectorySearch searches the Google Workspace organization directory
// (People API SearchDirectoryPeople). Only available for Workspace accounts —
// personal Gmail accounts have no directory and are handled gracefully.
// Returns each match's display name and email addresses.
func (c *ContactsService) DirectorySearch(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	args := req.GetArguments()

	query, _ := args["query"].(string)
	if strings.TrimSpace(query) == "" {
		return ErrorResult(fmt.Errorf("query is required")), nil
	}

	svc, acct, err := c.resolveAndGetService(ctx, args)
	if err != nil {
		return ErrorResult(err), nil
	}

	resp, err := svc.People.SearchDirectoryPeople().
		Query(query).
		ReadMask("names,emailAddresses").
		Sources("DIRECTORY_SOURCE_TYPE_DOMAIN_PROFILE").
		PageSize(30).
		Do()
	if err != nil {
		// A genuine insufficient-scope 403 should still prompt reauth so the
		// user can grant the directory.readonly scope.
		if wrapped := auth.CheckScopeError(err, acct.Label, acct.Email, "Contacts directory"); wrapped != err {
			return ErrorResult(wrapped), nil
		}
		// Any other failure — most commonly a 403/400 on a personal Gmail
		// account, which has no organization directory — is surfaced as a
		// clear message rather than a raw Google API error.
		return TextResult(fmt.Sprintf(
			"Directory search is only available for Google Workspace accounts. "+
				"The account %s (%s) does not have an accessible organization directory "+
				"(personal Gmail accounts do not have one).\n\nUnderlying API error: %v",
			acct.Label, acct.Email, err)), nil
	}

	if len(resp.People) == 0 {
		return TextResult(fmt.Sprintf(
			"No directory people matching %q found on %s (%s).", query, acct.Label, acct.Email)), nil
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Directory results on %s (%s) matching %q — %d found:\n\n",
		acct.Label, acct.Email, query, len(resp.People)))
	for i, p := range resp.People {
		sb.WriteString(fmt.Sprintf("%d. %s", i+1, formatPerson(p)))
	}

	return TextResult(sb.String()), nil
}

// formatPerson renders a People API Person as an indented block with the
// display name, email addresses, and phone numbers. Returns a trailing
// newline so callers can concatenate entries directly.
func formatPerson(p *people.Person) string {
	if p == nil {
		return "(no details)\n"
	}

	name := ""
	if len(p.Names) > 0 {
		name = p.Names[0].DisplayName
	}
	if name == "" {
		name = "(no name)"
	}

	var sb strings.Builder
	sb.WriteString(name + "\n")

	if len(p.EmailAddresses) > 0 {
		emails := make([]string, 0, len(p.EmailAddresses))
		for _, e := range p.EmailAddresses {
			if e.Value != "" {
				emails = append(emails, e.Value)
			}
		}
		if len(emails) > 0 {
			sb.WriteString(fmt.Sprintf("   Emails: %s\n", strings.Join(emails, ", ")))
		}
	}

	if len(p.PhoneNumbers) > 0 {
		phones := make([]string, 0, len(p.PhoneNumbers))
		for _, ph := range p.PhoneNumbers {
			if ph.Value != "" {
				phones = append(phones, ph.Value)
			}
		}
		if len(phones) > 0 {
			sb.WriteString(fmt.Sprintf("   Phones: %s\n", strings.Join(phones, ", ")))
		}
	}

	sb.WriteString("\n")
	return sb.String()
}

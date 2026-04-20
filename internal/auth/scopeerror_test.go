package auth

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

func TestScopeError_ErrorMessage(t *testing.T) {
	e := &ScopeError{
		AccountLabel: "personal",
		AccountEmail: "nico@example.com",
		Operation:    "Sheets",
	}
	msg := e.Error()

	// Must mention the account, operation, and the reauth tool.
	for _, want := range []string{`"personal"`, "Sheets", "gws.accounts.reauth", `"nico@example.com"`} {
		if !strings.Contains(msg, want) {
			t.Errorf("ScopeError.Error() missing %q: %s", want, msg)
		}
	}

	// Must NOT leak likely-sensitive substrings.
	for _, bad := range []string{"Bearer ", "ya29.", "response body", "sheet_id"} {
		if strings.Contains(msg, bad) {
			t.Errorf("ScopeError.Error() leaks %q: %s", bad, msg)
		}
	}
}

func TestCheckScopeError_NilPassesThrough(t *testing.T) {
	if err := CheckScopeError(nil, "l", "e@x", "Sheets"); err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestCheckScopeError_NonScopePassesThrough(t *testing.T) {
	orig := errors.New("spreadsheet not found")
	out := CheckScopeError(orig, "l", "e@x", "Sheets")
	if out != orig {
		t.Errorf("expected passthrough, got %v", out)
	}
}

// Each Google wording seen in the wild must map to a ScopeError when the
// underlying 403 signals insufficient scope.
func TestCheckScopeError_DetectsGoogleWordings(t *testing.T) {
	cases := []struct {
		name string
		err  error
	}{
		{
			name: "ACCESS_TOKEN_SCOPE_INSUFFICIENT in googleapi.Error",
			err: &googleapi.Error{
				Code:    403,
				Message: "Request had insufficient authentication scopes.",
				Body:    `{"error":{"code":403,"message":"Request had insufficient authentication scopes.","status":"PERMISSION_DENIED","details":[{"@type":"type.googleapis.com/google.rpc.ErrorInfo","reason":"ACCESS_TOKEN_SCOPE_INSUFFICIENT"}]}}`,
			},
		},
		{
			name: "PERMISSION_DENIED marker",
			err: &googleapi.Error{
				Code: 403,
				Body: `{"error":{"status":"PERMISSION_DENIED"}}`,
			},
		},
		{
			name: "insufficient scope phrase",
			err: &googleapi.Error{
				Code:    403,
				Message: "insufficient scope for this request",
			},
		},
		{
			name: "wrapped googleapi.Error",
			err: fmt.Errorf("reading range on personal: %w", &googleapi.Error{
				Code: 403, Message: "Request had insufficient authentication scopes.",
			}),
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			out := CheckScopeError(tc.err, "personal", "nico@example.com", "Sheets")
			var se *ScopeError
			if !errors.As(out, &se) {
				t.Fatalf("expected *ScopeError, got %T: %v", out, out)
			}
			if se.AccountLabel != "personal" || se.Operation != "Sheets" {
				t.Errorf("scope error fields wrong: %+v", se)
			}
		})
	}
}

// A 403 that is not about scope (e.g., quota, forbidden resource) must
// remain untouched so we don't confuse the user.
func TestCheckScopeError_Ignores403WithoutScopeMarker(t *testing.T) {
	gerr := &googleapi.Error{
		Code:    403,
		Message: "The caller does not have permission",
		Body:    `{"error":{"code":403,"message":"The caller does not have permission."}}`,
	}
	out := CheckScopeError(gerr, "l", "e@x", "Sheets")
	var se *ScopeError
	if errors.As(out, &se) {
		t.Errorf("plain 403 misclassified as ScopeError")
	}
}

// A non-403 error that happens to contain the marker string must not be
// treated as scope-insufficient; we rely on the HTTP code when it's
// available.
func TestCheckScopeError_Ignores500WithScopeMarkerInBody(t *testing.T) {
	gerr := &googleapi.Error{
		Code:    500,
		Message: "internal — PERMISSION_DENIED somewhere in trace",
	}
	out := CheckScopeError(gerr, "l", "e@x", "Sheets")
	var se *ScopeError
	if errors.As(out, &se) {
		t.Errorf("500 misclassified as ScopeError")
	}
}

// For raw (non-googleapi) errors we are permissive: if the message clearly
// contains one of the scope markers we flag it. This handles pre-wrapped
// errors from other places in the stack.
func TestCheckScopeError_DetectsRawMarker(t *testing.T) {
	out := CheckScopeError(errors.New("boom: ACCESS_TOKEN_SCOPE_INSUFFICIENT"), "l", "e@x", "Sheets")
	var se *ScopeError
	if !errors.As(out, &se) {
		t.Errorf("raw marker not detected: %v", out)
	}
}

func TestValidateScopes_NilTokenPermissive(t *testing.T) {
	if err := ValidateScopes(nil, []string{ScopeSheets}, "l", "e@x", "Sheets"); err != nil {
		t.Errorf("nil token should be permissive, got %v", err)
	}
}

func TestValidateScopes_NoGrantInfoPermissive(t *testing.T) {
	// Token has no Extra("scope") — we cannot know what was granted,
	// so we trust and let the reactive check catch missing scopes later.
	tok := &oauth2.Token{AccessToken: "x"}
	if err := ValidateScopes(tok, []string{ScopeSheets}, "l", "e@x", "Sheets"); err != nil {
		t.Errorf("missing scope field should be permissive, got %v", err)
	}
}

func TestValidateScopes_AllGranted(t *testing.T) {
	tok := tokenWithScopes(
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/spreadsheets",
	)
	err := ValidateScopes(tok, []string{ScopeSheets}, "l", "e@x", "Sheets")
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
}

func TestValidateScopes_Missing(t *testing.T) {
	tok := tokenWithScopes(
		"https://www.googleapis.com/auth/gmail.modify",
	)
	err := ValidateScopes(tok, []string{ScopeSheets}, "personal", "e@x", "Sheets")
	var se *ScopeError
	if !errors.As(err, &se) {
		t.Fatalf("expected *ScopeError, got %T: %v", err, err)
	}
	if se.Operation != "Sheets" {
		t.Errorf("operation field wrong: %+v", se)
	}
}

func TestValidateScopes_MultipleRequiredOneMissing(t *testing.T) {
	tok := tokenWithScopes(ScopeSheets)
	err := ValidateScopes(tok, []string{ScopeSheets, ScopeDocs}, "l", "e@x", "Docs")
	var se *ScopeError
	if !errors.As(err, &se) {
		t.Errorf("expected ScopeError, got %v", err)
	}
}

// tokenWithScopes returns an oauth2.Token carrying the given scopes in the
// standard `scope` extra field.
func tokenWithScopes(scopes ...string) *oauth2.Token {
	tok := &oauth2.Token{AccessToken: "x"}
	return tok.WithExtra(map[string]any{"scope": strings.Join(scopes, " ")})
}

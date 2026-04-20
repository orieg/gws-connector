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

// PERMISSION_DENIED is a generic Google status returned for any 403 —
// including resource-level IAM issues that reauth will not fix. Ensure
// it is NOT treated as a scope error on its own; only the more specific
// markers above should trigger a scope prompt.
func TestCheckScopeError_IgnoresBarePermissionDenied(t *testing.T) {
	gerr := &googleapi.Error{
		Code: 403,
		Body: `{"error":{"code":403,"status":"PERMISSION_DENIED","message":"The caller does not have permission to access resource."}}`,
	}
	out := CheckScopeError(gerr, "l", "e@x", "Sheets")
	var se *ScopeError
	if errors.As(out, &se) {
		t.Errorf("bare PERMISSION_DENIED (no scope marker) must not be a ScopeError — would lead to false reauth prompts for IAM issues")
	}
}

// A non-403 error that happens to contain a scope marker string in its
// body must not be treated as scope-insufficient; we rely on the HTTP
// code when googleapi.Error tells us one.
func TestCheckScopeError_Ignores500WithScopeMarkerInBody(t *testing.T) {
	gerr := &googleapi.Error{
		Code:    500,
		Message: "internal — ACCESS_TOKEN_SCOPE_INSUFFICIENT somewhere in trace",
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

// Direct tests of the low-level helpers to cover defensive branches not
// reached through the public API surface.

func TestIsScopeInsufficient_NilErr(t *testing.T) {
	if isScopeInsufficient(nil) {
		t.Error("nil err must not be a scope failure")
	}
}

func TestGrantedScopes_NilToken(t *testing.T) {
	if got := grantedScopes(nil); got != nil {
		t.Errorf("expected nil for nil token, got %v", got)
	}
}

func TestGrantedScopes_EmptyString(t *testing.T) {
	tok := (&oauth2.Token{AccessToken: "x"}).WithExtra(map[string]any{"scope": ""})
	if got := grantedScopes(tok); got != nil {
		t.Errorf("expected nil for empty scope string, got %v", got)
	}
}

func TestGrantedScopes_NonStringScopeField(t *testing.T) {
	// Some servers misbehave and return a number or list for "scope" — we
	// should treat that as "unknown" rather than panic.
	tok := (&oauth2.Token{AccessToken: "x"}).WithExtra(map[string]any{"scope": 42})
	if got := grantedScopes(tok); got != nil {
		t.Errorf("expected nil for non-string scope field, got %v", got)
	}
}

func TestGrantedScopes_SpaceSeparated(t *testing.T) {
	tok := (&oauth2.Token{AccessToken: "x"}).WithExtra(map[string]any{
		"scope": "a b  c",
	})
	got := grantedScopes(tok)
	if len(got) != 3 || got[0] != "a" || got[2] != "c" {
		t.Errorf("unexpected parse: %v", got)
	}
}

func TestContainsScope(t *testing.T) {
	got := []string{"a", "b", "c"}
	if !containsScope(got, "b") {
		t.Error("expected true for present scope")
	}
	if containsScope(got, "d") {
		t.Error("expected false for missing scope")
	}
	if containsScope(nil, "a") {
		t.Error("expected false for nil set")
	}
}

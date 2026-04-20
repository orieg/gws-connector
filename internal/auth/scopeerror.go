package auth

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/oauth2"
	"google.golang.org/api/googleapi"
)

// ScopeError indicates the current access token does not carry the OAuth
// scopes required for the attempted operation. Handlers should return this
// so agents see a consistent, actionable reauth prompt instead of a raw
// Google API error.
//
// Logging contract: ScopeError.Error() returns only the account label,
// the operation noun, and the suggested remediation. It must not include
// the raw Google API response body, any portion of the access token, or
// file/resource IDs the user did not explicitly reference in the call.
type ScopeError struct {
	// AccountLabel is the user-facing account label ("personal", "work").
	AccountLabel string
	// AccountEmail is the email of the authorizing account. Included
	// so agents can pass it to gws.accounts.reauth.
	AccountEmail string
	// Operation is a short noun describing what was attempted
	// ("Sheets", "Gmail", "Docs").
	Operation string
}

func (e *ScopeError) Error() string {
	return fmt.Sprintf(
		"account %q lacks the scopes required for this %s operation. "+
			"Call gws.accounts.reauth with account=%q to refresh — the user "+
			"must approve the Google consent screen in their browser.",
		e.AccountLabel, e.Operation, e.AccountEmail)
}

// scopeErrorMarkers are substrings Google uses in 403 response bodies /
// error messages to indicate insufficient OAuth scope. Stored lowercase
// and matched against a lowercased message, so we don't pay for
// strings.ToLower on every marker for every error.
//
// PERMISSION_DENIED is deliberately NOT on this list: Google returns it
// for any 403 (including resource-level IAM issues the user has no scope
// problem with), so matching on it would surface false reauth prompts.
// The other markers only appear on genuine scope-insufficient responses.
var scopeErrorMarkers = []string{
	"access_token_scope_insufficient",
	"insufficient authentication scopes",
	"insufficient scope",
}

// isScopeInsufficient reports whether the error message matches one of the
// known "insufficient scope" markers. Works on any error whose rendering
// contains the marker — including googleapi.Error bodies and wrapped
// variants — and is deliberately broad because Google's wording shifts.
func isScopeInsufficient(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	// If it's a googleapi.Error, also check Code — 403 is the canonical
	// signal. Other 403 reasons (e.g., quota exceeded) must not be
	// surfaced as scope errors.
	var gerr *googleapi.Error
	if errors.As(err, &gerr) {
		if gerr.Code != 403 {
			return false
		}
		msg = gerr.Message + "\n" + gerr.Body
	}
	lower := strings.ToLower(msg)
	for _, marker := range scopeErrorMarkers {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// CheckScopeError wraps err in a *ScopeError when the underlying failure
// is a Google insufficient-scope 403; otherwise returns err unchanged.
//
// operation is a short noun used in the remediation message ("Sheets",
// "Gmail", "Calendar").
func CheckScopeError(err error, acctLabel, acctEmail, operation string) error {
	if err == nil {
		return nil
	}
	if isScopeInsufficient(err) {
		return &ScopeError{
			AccountLabel: acctLabel,
			AccountEmail: acctEmail,
			Operation:    operation,
		}
	}
	return err
}

// ValidateScopes parses the scopes granted to token and verifies that every
// scope in required is present. Returns a *ScopeError when any required
// scope is missing.
//
// The OAuth2 server returns granted scopes via the `scope` response field,
// which oauth2 stores in Token.Extra("scope"). Absence of that field is not
// an error — older tokens may pre-date the change — so this function is
// permissive (returns nil) when the grant set cannot be determined. The
// reactive CheckScopeError still catches any real scope-insufficient
// response from Google.
func ValidateScopes(token *oauth2.Token, required []string, acctLabel, acctEmail, operation string) error {
	if token == nil {
		return nil
	}
	granted := grantedScopes(token)
	if len(granted) == 0 {
		return nil
	}
	for _, need := range required {
		if !containsScope(granted, need) {
			return &ScopeError{
				AccountLabel: acctLabel,
				AccountEmail: acctEmail,
				Operation:    operation,
			}
		}
	}
	return nil
}

// grantedScopes extracts the set of scopes granted in token. Returns an
// empty slice when the information is unavailable.
func grantedScopes(token *oauth2.Token) []string {
	if token == nil {
		return nil
	}
	raw := token.Extra("scope")
	s, ok := raw.(string)
	if !ok || s == "" {
		return nil
	}
	return strings.Fields(s)
}

func containsScope(granted []string, want string) bool {
	for _, g := range granted {
		if g == want {
			return true
		}
	}
	return false
}

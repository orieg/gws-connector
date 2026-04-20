package auth

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	docs "google.golang.org/api/docs/v1"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
	sheets "google.golang.org/api/sheets/v4"
)

// AccountCredentials provides per-account OAuth client ID lookup.
type AccountCredentials interface {
	// GetClientID returns the per-account OAuth client ID for the given email.
	// Returns empty string if no per-account client ID is set.
	GetClientID(email string) string
}

// ClientFactory creates authenticated Google API service clients for accounts.
type ClientFactory struct {
	tokenStore       *TokenStore
	globalClientID   string
	globalClientSec  string
	accountCreds     AccountCredentials
}

// NewClientFactory creates a new factory. accountCreds can be nil if no
// per-account credentials are needed.
func NewClientFactory(tokenStore *TokenStore, clientID, clientSecret string, accountCreds AccountCredentials) *ClientFactory {
	return &ClientFactory{
		tokenStore:      tokenStore,
		globalClientID:  clientID,
		globalClientSec: clientSecret,
		accountCreds:    accountCreds,
	}
}

// CredentialsForAccount returns the OAuth client ID and secret to use for
// the given account. Per-account credentials (keychain) take priority over global.
func (f *ClientFactory) CredentialsForAccount(email string) (string, string) {
	// Try per-account client ID from registry
	clientID := ""
	if f.accountCreds != nil {
		clientID = f.accountCreds.GetClientID(email)
	}

	// Try per-account client secret from keychain
	clientSecret, _ := f.tokenStore.LoadClientSecret(email)

	if clientID != "" && clientSecret != "" {
		return clientID, clientSecret
	}

	return f.globalClientID, f.globalClientSec
}

// httpClient returns an authenticated HTTP client for the given account.
func (f *ClientFactory) httpClient(ctx context.Context, email string) (*http.Client, error) {
	token, err := f.tokenStore.Load(email)
	if err != nil {
		return nil, fmt.Errorf("loading token for %s: %w — try running /gws:add-account to re-authenticate", email, err)
	}

	clientID, clientSecret := f.CredentialsForAccount(email)
	ts := TokenSourceForAccount(ctx, clientID, clientSecret, token)

	// Get a fresh token (auto-refreshes if expired)
	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token for %s: %w — try running /gws:add-account to re-authenticate", email, err)
	}

	// Persist refreshed token if it changed
	if newToken.AccessToken != token.AccessToken {
		if err := f.tokenStore.Save(email, newToken); err != nil {
			log.Printf("gws-connector: failed to persist refreshed token for %s: %v", email, err)
		}
	}

	return oauth2.NewClient(ctx, ts), nil
}

// GmailService returns an authenticated Gmail service for the account.
func (f *ClientFactory) GmailService(ctx context.Context, email string) (*gmail.Service, error) {
	client, err := f.httpClient(ctx, email)
	if err != nil {
		return nil, err
	}
	return gmail.NewService(ctx, option.WithHTTPClient(client))
}

// CalendarService returns an authenticated Calendar service for the account.
func (f *ClientFactory) CalendarService(ctx context.Context, email string) (*calendar.Service, error) {
	client, err := f.httpClient(ctx, email)
	if err != nil {
		return nil, err
	}
	return calendar.NewService(ctx, option.WithHTTPClient(client))
}

// DriveService returns an authenticated Drive service for the account.
func (f *ClientFactory) DriveService(ctx context.Context, email string) (*drive.Service, error) {
	client, err := f.httpClient(ctx, email)
	if err != nil {
		return nil, err
	}
	return drive.NewService(ctx, option.WithHTTPClient(client))
}

// SheetsService returns an authenticated Sheets service for the account.
//
// If the account's stored token does not carry the Sheets scope — common for
// users upgrading from releases that pre-date Sheets support — this returns
// a *ScopeError immediately, without hitting the Google API. Callers
// surface the error directly; the message tells the agent which reauth
// tool to call.
func (f *ClientFactory) SheetsService(ctx context.Context, email string) (*sheets.Service, error) {
	if err := f.ensureScope(email, ScopeSheets, "Sheets"); err != nil {
		return nil, err
	}
	client, err := f.httpClient(ctx, email)
	if err != nil {
		return nil, err
	}
	return sheets.NewService(ctx, option.WithHTTPClient(client))
}

// DocsService returns an authenticated Docs service for the account.
// Same proactive-scope behavior as SheetsService.
func (f *ClientFactory) DocsService(ctx context.Context, email string) (*docs.Service, error) {
	if err := f.ensureScope(email, ScopeDocs, "Docs"); err != nil {
		return nil, err
	}
	client, err := f.httpClient(ctx, email)
	if err != nil {
		return nil, err
	}
	return docs.NewService(ctx, option.WithHTTPClient(client))
}

// ensureScope loads the account's token and runs ValidateScopes against it.
// Used by Sheets/Docs factory methods so a stale token is caught before any
// API call is attempted. Label is derived from email when we can't look it
// up through AccountCredentials (the interface only exposes GetClientID).
func (f *ClientFactory) ensureScope(email, scope, operation string) error {
	tok, err := f.tokenStore.Load(email)
	if err != nil {
		// Missing-token failures are already well-surfaced by httpClient
		// downstream; let that path handle them so we don't double-wrap.
		return nil
	}
	return ValidateScopes(tok, []string{scope}, email, email, operation)
}

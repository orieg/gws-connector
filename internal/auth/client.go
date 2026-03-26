package auth

import (
	"context"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/drive/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"
)

// ClientFactory creates authenticated Google API service clients for accounts.
type ClientFactory struct {
	tokenStore   *TokenStore
	clientID     string
	clientSecret string
}

// NewClientFactory creates a new factory.
func NewClientFactory(tokenStore *TokenStore, clientID, clientSecret string) *ClientFactory {
	return &ClientFactory{
		tokenStore:   tokenStore,
		clientID:     clientID,
		clientSecret: clientSecret,
	}
}

// httpClient returns an authenticated HTTP client for the given account.
func (f *ClientFactory) httpClient(ctx context.Context, email string) (*http.Client, error) {
	token, err := f.tokenStore.Load(email)
	if err != nil {
		return nil, fmt.Errorf("loading token for %s: %w — try running /gws:add-account to re-authenticate", email, err)
	}

	ts := TokenSourceForAccount(ctx, f.clientID, f.clientSecret, token)

	// Get a fresh token (auto-refreshes if expired)
	newToken, err := ts.Token()
	if err != nil {
		return nil, fmt.Errorf("refreshing token for %s: %w — try running /gws:add-account to re-authenticate", email, err)
	}

	// Persist refreshed token if it changed
	if newToken.AccessToken != token.AccessToken {
		f.tokenStore.Save(email, newToken)
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

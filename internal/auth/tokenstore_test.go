package auth

import (
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// newFileTokenStore creates a token store that always uses file storage
// (avoids keychain interaction in CI).
func newFileTokenStore(t *testing.T) *TokenStore {
	t.Helper()
	return &TokenStore{
		stateDir:       t.TempDir(),
		useFileStorage: true,
	}
}

func TestSaveAndLoadToken(t *testing.T) {
	ts := newFileTokenStore(t)

	token := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	err := ts.Save("alice@example.com", token)
	if err != nil {
		t.Fatalf("Save() error: %v", err)
	}

	loaded, err := ts.Load("alice@example.com")
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if loaded.AccessToken != "access-123" {
		t.Errorf("expected access-123, got %s", loaded.AccessToken)
	}
	if loaded.RefreshToken != "refresh-456" {
		t.Errorf("expected refresh-456, got %s", loaded.RefreshToken)
	}
	if loaded.TokenType != "Bearer" {
		t.Errorf("expected Bearer, got %s", loaded.TokenType)
	}
}

func TestLoadNonexistentToken(t *testing.T) {
	ts := newFileTokenStore(t)

	_, err := ts.Load("nobody@example.com")
	if err == nil {
		t.Fatal("expected error loading nonexistent token")
	}
}

func TestDeleteToken(t *testing.T) {
	ts := newFileTokenStore(t)

	token := &oauth2.Token{
		AccessToken:  "access-123",
		RefreshToken: "refresh-456",
	}
	ts.Save("alice@example.com", token)

	err := ts.Delete("alice@example.com")
	if err != nil {
		t.Fatalf("Delete() error: %v", err)
	}

	_, err = ts.Load("alice@example.com")
	if err == nil {
		t.Error("expected error after deleting token")
	}
}

func TestMultipleAccountTokens(t *testing.T) {
	ts := newFileTokenStore(t)

	token1 := &oauth2.Token{AccessToken: "alice-token"}
	token2 := &oauth2.Token{AccessToken: "bob-token"}

	ts.Save("alice@example.com", token1)
	ts.Save("bob@work.com", token2)

	loaded1, _ := ts.Load("alice@example.com")
	loaded2, _ := ts.Load("bob@work.com")

	if loaded1.AccessToken != "alice-token" {
		t.Errorf("expected alice-token, got %s", loaded1.AccessToken)
	}
	if loaded2.AccessToken != "bob-token" {
		t.Errorf("expected bob-token, got %s", loaded2.AccessToken)
	}
}

func TestTokenFilePathSanitization(t *testing.T) {
	ts := newFileTokenStore(t)

	path := ts.tokenFilePath("user+tag@example.com")
	// @ and + should be replaced with _
	if path == "" {
		t.Fatal("expected non-empty path")
	}
	// Should not contain @ (replaced with _)
	for _, c := range path[len(ts.tokenDir()):] {
		if c == '@' || c == '+' {
			t.Errorf("path should not contain special chars: %s", path)
			break
		}
	}
}

func TestOverwriteToken(t *testing.T) {
	ts := newFileTokenStore(t)

	token1 := &oauth2.Token{AccessToken: "old-token"}
	token2 := &oauth2.Token{AccessToken: "new-token"}

	ts.Save("alice@example.com", token1)
	ts.Save("alice@example.com", token2)

	loaded, _ := ts.Load("alice@example.com")
	if loaded.AccessToken != "new-token" {
		t.Errorf("expected new-token, got %s", loaded.AccessToken)
	}
}

func TestDeleteNonexistentTokenNoError(t *testing.T) {
	ts := newFileTokenStore(t)

	err := ts.Delete("nobody@example.com")
	if err != nil {
		t.Errorf("Delete() should not error for nonexistent token: %v", err)
	}
}

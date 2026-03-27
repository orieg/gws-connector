package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/zalando/go-keyring"
	"golang.org/x/oauth2"
)

const keychainService = "claude-gws-connector"

// TokenStore manages OAuth tokens for multiple accounts.
// Primary storage: OS keychain. Fallback: file on disk (restricted permissions).
type TokenStore struct {
	stateDir       string
	useFileStorage bool // fallback when keychain unavailable
}

// NewTokenStore creates a token store. It attempts keychain access
// and falls back to file storage if unavailable.
func NewTokenStore(stateDir string) *TokenStore {
	ts := &TokenStore{stateDir: stateDir}

	// Test keychain availability with a probe
	testKey := "__gws_keychain_test__"
	err := keyring.Set(keychainService, testKey, "test")
	if err != nil {
		ts.useFileStorage = true
	} else {
		_ = keyring.Delete(keychainService, testKey) // best-effort cleanup
	}

	return ts
}

// Save stores a token for the given account email.
func (ts *TokenStore) Save(email string, token *oauth2.Token) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("marshaling token: %w", err)
	}

	if ts.useFileStorage {
		return ts.saveToFile(email, data)
	}

	if err := keyring.Set(keychainService, email, string(data)); err != nil {
		// Fallback to file if keychain write fails
		return ts.saveToFile(email, data)
	}
	return nil
}

// Load retrieves a token for the given account email.
func (ts *TokenStore) Load(email string) (*oauth2.Token, error) {
	if ts.useFileStorage {
		return ts.loadFromFile(email)
	}

	secret, err := keyring.Get(keychainService, email)
	if err != nil {
		// Try file fallback
		return ts.loadFromFile(email)
	}

	var token oauth2.Token
	if err := json.Unmarshal([]byte(secret), &token); err != nil {
		return nil, fmt.Errorf("parsing token from keychain: %w", err)
	}
	return &token, nil
}

// Delete removes a token for the given account email.
func (ts *TokenStore) Delete(email string) error {
	if !ts.useFileStorage {
		keyring.Delete(keychainService, email)
	}
	// Also clean up any file storage
	path := ts.tokenFilePath(email)
	if _, err := os.Stat(path); err == nil {
		os.Remove(path)
	}
	return nil
}

// File-based fallback storage

func (ts *TokenStore) tokenDir() string {
	return filepath.Join(ts.stateDir, "tokens")
}

func (ts *TokenStore) tokenFilePath(email string) string {
	// Use a safe filename derived from email
	safe := ""
	for _, c := range email {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			safe += string(c)
		} else {
			safe += "_"
		}
	}
	return filepath.Join(ts.tokenDir(), safe+".json")
}

func (ts *TokenStore) saveToFile(email string, data []byte) error {
	dir := ts.tokenDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating token dir: %w", err)
	}
	path := ts.tokenFilePath(email)
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing token file: %w", err)
	}
	return nil
}

func (ts *TokenStore) loadFromFile(email string) (*oauth2.Token, error) {
	path := ts.tokenFilePath(email)
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading token file for %s: %w", email, err)
	}
	var token oauth2.Token
	if err := json.Unmarshal(data, &token); err != nil {
		return nil, fmt.Errorf("parsing token file: %w", err)
	}
	return &token, nil
}

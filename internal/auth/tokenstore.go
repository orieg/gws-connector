package auth

import (
	"encoding/json"
	"fmt"
	"log"
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
		if delErr := keyring.Delete(keychainService, testKey); delErr != nil {
			log.Printf("warning: keychain probe cleanup failed: %v (falling back to file storage)", delErr)
			ts.useFileStorage = true
		}
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

// SaveClientSecret stores a per-account OAuth client secret in the keychain.
func (ts *TokenStore) SaveClientSecret(email, secret string) error {
	key := email + ":client_secret"
	if ts.useFileStorage {
		return ts.saveSecretToFile(email, secret)
	}
	if err := keyring.Set(keychainService, key, secret); err != nil {
		return ts.saveSecretToFile(email, secret)
	}
	return nil
}

// LoadClientSecret retrieves a per-account OAuth client secret.
func (ts *TokenStore) LoadClientSecret(email string) (string, error) {
	key := email + ":client_secret"
	if ts.useFileStorage {
		return ts.loadSecretFromFile(email)
	}
	secret, err := keyring.Get(keychainService, key)
	if err != nil {
		return ts.loadSecretFromFile(email)
	}
	return secret, nil
}

// DeleteClientSecret removes a per-account OAuth client secret.
func (ts *TokenStore) DeleteClientSecret(email string) error {
	key := email + ":client_secret"
	if !ts.useFileStorage {
		if err := keyring.Delete(keychainService, key); err != nil {
			log.Printf("gws-connector: failed to delete client secret from keychain for %s: %v", email, err)
		}
	}
	path := ts.secretFilePath(email)
	if _, err := os.Stat(path); err == nil {
		if err := os.Remove(path); err != nil {
			return fmt.Errorf("removing client secret file for %s: %w", email, err)
		}
	}
	return nil
}

func (ts *TokenStore) secretFilePath(email string) string {
	safe := sanitizeEmail(email)
	return filepath.Join(ts.tokenDir(), safe+"_client_secret.txt")
}

func (ts *TokenStore) saveSecretToFile(email, secret string) error {
	dir := ts.tokenDir()
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("creating token dir: %w", err)
	}
	path := ts.secretFilePath(email)
	if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
		return fmt.Errorf("writing client secret file: %w", err)
	}
	return nil
}

func (ts *TokenStore) loadSecretFromFile(email string) (string, error) {
	path := ts.secretFilePath(email)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading client secret for %s: %w", email, err)
	}
	return string(data), nil
}

// File-based fallback storage

func (ts *TokenStore) tokenDir() string {
	return filepath.Join(ts.stateDir, "tokens")
}

func sanitizeEmail(email string) string {
	safe := ""
	for _, c := range email {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '.' || c == '-' || c == '_' {
			safe += string(c)
		} else {
			safe += "_"
		}
	}
	return safe
}

func (ts *TokenStore) tokenFilePath(email string) string {
	return filepath.Join(ts.tokenDir(), sanitizeEmail(email)+".json")
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

package accounts

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var (
	ErrAccountNotFound = errors.New("account not found")
	ErrAccountExists   = errors.New("account already exists")
	ErrNoAccounts      = errors.New("no accounts configured")
	ErrLabelInUse      = errors.New("label already in use")
)

// Store manages the accounts.json registry file.
type Store struct {
	stateDir string
}

// NewStore creates a new account store at the given state directory.
func NewStore(stateDir string) *Store {
	return &Store{stateDir: stateDir}
}

func (s *Store) registryPath() string {
	return filepath.Join(s.stateDir, "accounts.json")
}

// Load reads the registry from disk. Returns empty registry if file doesn't exist.
func (s *Store) Load() (*Registry, error) {
	data, err := os.ReadFile(s.registryPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Registry{
				Accounts:     []Account{},
				RoutingRules: RoutingRules{Domains: map[string]string{}},
			}, nil
		}
		return nil, fmt.Errorf("reading accounts.json: %w", err)
	}
	var reg Registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return nil, fmt.Errorf("parsing accounts.json: %w", err)
	}
	if reg.RoutingRules.Domains == nil {
		reg.RoutingRules.Domains = map[string]string{}
	}
	return &reg, nil
}

// Save writes the registry to disk, creating the directory if needed.
func (s *Store) Save(reg *Registry) error {
	if err := os.MkdirAll(s.stateDir, 0700); err != nil {
		return fmt.Errorf("creating state dir: %w", err)
	}
	data, err := json.MarshalIndent(reg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling accounts.json: %w", err)
	}
	if err := os.WriteFile(s.registryPath(), data, 0600); err != nil {
		return fmt.Errorf("writing accounts.json: %w", err)
	}
	return nil
}

// Add registers a new account. clientID/clientSecret are optional per-account
// OAuth credentials that override the global ones. Pass empty strings to use global.
func (s *Store) Add(email, label, displayName, clientID, clientSecret string) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}

	for _, a := range reg.Accounts {
		if strings.EqualFold(a.Email, email) {
			return ErrAccountExists
		}
		if strings.EqualFold(a.Label, label) {
			return ErrLabelInUse
		}
	}

	isDefault := len(reg.Accounts) == 0
	reg.Accounts = append(reg.Accounts, Account{
		Email:        email,
		Label:        label,
		DisplayName:  displayName,
		AddedAt:      time.Now().UTC().Format(time.RFC3339),
		Services:     []string{"mail", "calendar", "drive"},
		Default:      isDefault,
		ClientID:     clientID,
		ClientSecret: clientSecret,
	})

	// Add domain routing rule (only if no rule exists for this domain yet)
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		if _, exists := reg.RoutingRules.Domains[parts[1]]; !exists {
			reg.RoutingRules.Domains[parts[1]] = email
		}
	}

	return s.Save(reg)
}

// Remove deletes an account by email or label.
func (s *Store) Remove(identifier string) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}

	idx := -1
	for i, a := range reg.Accounts {
		if a.Email == identifier || a.Label == identifier {
			idx = i
			break
		}
	}
	if idx == -1 {
		return ErrAccountNotFound
	}

	removed := reg.Accounts[idx]
	reg.Accounts = append(reg.Accounts[:idx], reg.Accounts[idx+1:]...)

	// Remove domain routing rule
	for domain, email := range reg.RoutingRules.Domains {
		if email == removed.Email {
			delete(reg.RoutingRules.Domains, domain)
		}
	}

	// If removed was default, promote the first remaining account
	if removed.Default && len(reg.Accounts) > 0 {
		reg.Accounts[0].Default = true
	}

	return s.Save(reg)
}

// SetDefault sets the default account by email or label.
func (s *Store) SetDefault(identifier string) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}

	found := false
	for i := range reg.Accounts {
		if reg.Accounts[i].Email == identifier || reg.Accounts[i].Label == identifier {
			reg.Accounts[i].Default = true
			found = true
		} else {
			reg.Accounts[i].Default = false
		}
	}
	if !found {
		return ErrAccountNotFound
	}

	return s.Save(reg)
}

// GetCredentials returns per-account OAuth credentials. Returns empty strings
// if the account has no custom credentials (caller should fall back to global).
func (s *Store) GetCredentials(email string) (clientID, clientSecret string) {
	reg, err := s.Load()
	if err != nil {
		return "", ""
	}
	for _, a := range reg.Accounts {
		if a.Email == email {
			return a.ClientID, a.ClientSecret
		}
	}
	return "", ""
}

// GetDefault returns the default account.
func (s *Store) GetDefault() (*Account, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	for _, a := range reg.Accounts {
		if a.Default {
			return &a, nil
		}
	}
	if len(reg.Accounts) > 0 {
		return &reg.Accounts[0], nil
	}
	return nil, ErrNoAccounts
}

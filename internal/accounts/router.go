package accounts

import (
	"fmt"
	"strings"
)

// Router resolves which account to use for a given request.
type Router struct {
	store *Store
}

// NewRouter creates a new account router.
func NewRouter(store *Store) *Router {
	return &Router{store: store}
}

// Resolve finds the account to use based on the optional account parameter.
// Priority: explicit match (email or label) > default > error.
func (r *Router) Resolve(accountParam string) (*Account, error) {
	reg, err := r.store.Load()
	if err != nil {
		return nil, err
	}

	if len(reg.Accounts) == 0 {
		return nil, fmt.Errorf("no accounts configured — run /gws:add-account to connect a Google account")
	}

	// If no account specified, use default
	if accountParam == "" {
		for _, a := range reg.Accounts {
			if a.Default {
				return &a, nil
			}
		}
		return &reg.Accounts[0], nil
	}

	// Try exact match by email or label
	for _, a := range reg.Accounts {
		if strings.EqualFold(a.Email, accountParam) || strings.EqualFold(a.Label, accountParam) {
			return &a, nil
		}
	}

	// Build helpful error listing available accounts
	labels := make([]string, len(reg.Accounts))
	for i, a := range reg.Accounts {
		labels[i] = fmt.Sprintf("%s (%s)", a.Label, a.Email)
	}
	return nil, fmt.Errorf("account %q not found. Available accounts: %s", accountParam, strings.Join(labels, ", "))
}

// ResolveByDomain returns the account associated with an email domain,
// using the routing rules in accounts.json.
func (r *Router) ResolveByDomain(domain string) (*Account, error) {
	reg, err := r.store.Load()
	if err != nil {
		return nil, err
	}

	email, ok := reg.RoutingRules.Domains[strings.ToLower(domain)]
	if !ok {
		return nil, fmt.Errorf("no account configured for domain %q", domain)
	}

	for _, a := range reg.Accounts {
		if a.Email == email {
			return &a, nil
		}
	}

	return nil, fmt.Errorf("routing rule points to %q but account not found", email)
}

// ListAccounts returns all connected accounts.
func (r *Router) ListAccounts() ([]Account, error) {
	reg, err := r.store.Load()
	if err != nil {
		return nil, err
	}
	return reg.Accounts, nil
}

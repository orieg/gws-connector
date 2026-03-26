package accounts

import "time"

// Account represents a connected Google Workspace account.
type Account struct {
	Email        string   `json:"email"`
	Label        string   `json:"label"`
	DisplayName  string   `json:"displayName"`
	AddedAt      string   `json:"addedAt"`
	Services     []string `json:"services"`
	Default      bool     `json:"default"`
	ClientID     string   `json:"clientId,omitempty"`     // per-account OAuth client ID (overrides global)
	ClientSecret string   `json:"clientSecret,omitempty"` // per-account OAuth client secret (overrides global)
}

// RoutingRules defines automatic account selection rules.
type RoutingRules struct {
	Domains map[string]string `json:"domains"` // domain -> email
}

// Registry is the top-level accounts.json structure.
type Registry struct {
	Accounts     []Account    `json:"accounts"`
	RoutingRules RoutingRules `json:"routingRules"`
}

// TokenData holds OAuth tokens for a single account.
type TokenData struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	Expiry       time.Time `json:"expiry"`
	Scopes       []string  `json:"scopes"`
}

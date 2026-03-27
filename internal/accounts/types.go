package accounts

// Account represents a connected Google Workspace account.
type Account struct {
	Email        string   `json:"email"`
	Label        string   `json:"label"`
	DisplayName  string   `json:"displayName"`
	AddedAt      string   `json:"addedAt"`
	Services     []string `json:"services"`
	Default      bool     `json:"default"`
	ClientID     string   `json:"clientId,omitempty"` // per-account OAuth client ID (overrides global)

	// Deprecated: client secrets are now stored in the OS keychain.
	// This field is retained for migration from older versions.
	ClientSecret string `json:"clientSecret,omitempty"`
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

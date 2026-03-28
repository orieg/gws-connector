package auth

import (
	"testing"
)

// mockAccountCreds implements AccountCredentials for testing.
type mockAccountCreds struct {
	clientIDs map[string]string // email -> clientID
}

func (m *mockAccountCreds) GetClientID(email string) string {
	if id, ok := m.clientIDs[email]; ok {
		return id
	}
	return ""
}

func TestCredentialsForAccountUsesPerAccount(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		clientIDs: map[string]string{
			"bob@corp.com": "corp-id",
		},
	}
	// Store client secret in token store (keychain/file)
	ts.SaveClientSecret("bob@corp.com", "corp-secret")

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cid, csec := factory.CredentialsForAccount("bob@corp.com")
	if cid != "corp-id" || csec != "corp-secret" {
		t.Errorf("expected per-account creds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountFallsBackToGlobal(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		clientIDs: map[string]string{},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cid, csec := factory.CredentialsForAccount("alice@example.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountNilAccountCreds(t *testing.T) {
	ts := newFileTokenStore(t)
	factory := NewClientFactory(ts, "global-id", "global-secret", nil)

	cid, csec := factory.CredentialsForAccount("anyone@example.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds with nil accountCreds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountPartialPerAccountFallsBack(t *testing.T) {
	ts := newFileTokenStore(t)
	// Per-account has clientID but no client secret in keychain
	creds := &mockAccountCreds{
		clientIDs: map[string]string{
			"partial@corp.com": "corp-id",
		},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	// Should fall back to global since both must be non-empty
	cid, csec := factory.CredentialsForAccount("partial@corp.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds for partial per-account, got %s/%s", cid, csec)
	}
}

func TestCredentialsMultipleAccounts(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		clientIDs: map[string]string{
			"alice@corpA.com": "corpA-id",
			"bob@corpB.com":   "corpB-id",
		},
	}
	ts.SaveClientSecret("alice@corpA.com", "corpA-secret")
	ts.SaveClientSecret("bob@corpB.com", "corpB-secret")

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cidA, csecA := factory.CredentialsForAccount("alice@corpA.com")
	if cidA != "corpA-id" || csecA != "corpA-secret" {
		t.Errorf("expected corpA creds, got %s/%s", cidA, csecA)
	}

	cidB, csecB := factory.CredentialsForAccount("bob@corpB.com")
	if cidB != "corpB-id" || csecB != "corpB-secret" {
		t.Errorf("expected corpB creds, got %s/%s", cidB, csecB)
	}

	// Unknown account gets global
	cidU, csecU := factory.CredentialsForAccount("unknown@other.com")
	if cidU != "global-id" || csecU != "global-secret" {
		t.Errorf("expected global creds for unknown, got %s/%s", cidU, csecU)
	}
}

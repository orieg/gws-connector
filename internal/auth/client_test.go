package auth

import (
	"testing"
)

// mockAccountCreds implements AccountCredentials for testing.
type mockAccountCreds struct {
	creds map[string][2]string // email -> {clientID, clientSecret}
}

func (m *mockAccountCreds) GetCredentials(email string) (string, string) {
	if c, ok := m.creds[email]; ok {
		return c[0], c[1]
	}
	return "", ""
}

func TestCredentialsForAccountUsesPerAccount(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		creds: map[string][2]string{
			"bob@corp.com": {"corp-id", "corp-secret"},
		},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cid, csec := factory.credentialsForAccount("bob@corp.com")
	if cid != "corp-id" || csec != "corp-secret" {
		t.Errorf("expected per-account creds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountFallsBackToGlobal(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		creds: map[string][2]string{},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cid, csec := factory.credentialsForAccount("alice@example.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountNilAccountCreds(t *testing.T) {
	ts := newFileTokenStore(t)
	factory := NewClientFactory(ts, "global-id", "global-secret", nil)

	cid, csec := factory.credentialsForAccount("anyone@example.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds with nil accountCreds, got %s/%s", cid, csec)
	}
}

func TestCredentialsForAccountPartialPerAccountFallsBack(t *testing.T) {
	ts := newFileTokenStore(t)
	// Per-account has clientID but empty clientSecret
	creds := &mockAccountCreds{
		creds: map[string][2]string{
			"partial@corp.com": {"corp-id", ""},
		},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	// Should fall back to global since both must be non-empty
	cid, csec := factory.credentialsForAccount("partial@corp.com")
	if cid != "global-id" || csec != "global-secret" {
		t.Errorf("expected global creds for partial per-account, got %s/%s", cid, csec)
	}
}

func TestCredentialsMultipleAccounts(t *testing.T) {
	ts := newFileTokenStore(t)
	creds := &mockAccountCreds{
		creds: map[string][2]string{
			"alice@corpA.com": {"corpA-id", "corpA-secret"},
			"bob@corpB.com":   {"corpB-id", "corpB-secret"},
		},
	}

	factory := NewClientFactory(ts, "global-id", "global-secret", creds)

	cidA, csecA := factory.credentialsForAccount("alice@corpA.com")
	if cidA != "corpA-id" || csecA != "corpA-secret" {
		t.Errorf("expected corpA creds, got %s/%s", cidA, csecA)
	}

	cidB, csecB := factory.credentialsForAccount("bob@corpB.com")
	if cidB != "corpB-id" || csecB != "corpB-secret" {
		t.Errorf("expected corpB creds, got %s/%s", cidB, csecB)
	}

	// Unknown account gets global
	cidU, csecU := factory.credentialsForAccount("unknown@other.com")
	if cidU != "global-id" || csecU != "global-secret" {
		t.Errorf("expected global creds for unknown, got %s/%s", cidU, csecU)
	}
}

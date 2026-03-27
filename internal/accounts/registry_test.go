package accounts

import (
	"os"
	"path/filepath"
	"testing"
)

func tempStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return NewStore(dir)
}

func TestLoadEmptyReturnsEmptyRegistry(t *testing.T) {
	store := tempStore(t)
	reg, err := store.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(reg.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(reg.Accounts))
	}
	if reg.RoutingRules.Domains == nil {
		t.Error("expected non-nil Domains map")
	}
}

func TestAddFirstAccountIsDefault(t *testing.T) {
	store := tempStore(t)

	err := store.Add("alice@example.com", "personal", "Alice", "", "")
	if err != nil {
		t.Fatalf("Add() error: %v", err)
	}

	reg, _ := store.Load()
	if len(reg.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(reg.Accounts))
	}
	if !reg.Accounts[0].Default {
		t.Error("first account should be default")
	}
	if reg.Accounts[0].Email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", reg.Accounts[0].Email)
	}
	if reg.Accounts[0].Label != "personal" {
		t.Errorf("expected label 'personal', got %s", reg.Accounts[0].Label)
	}
}

func TestAddSecondAccountIsNotDefault(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	reg, _ := store.Load()
	if len(reg.Accounts) != 2 {
		t.Fatalf("expected 2 accounts, got %d", len(reg.Accounts))
	}
	if !reg.Accounts[0].Default {
		t.Error("first account should remain default")
	}
	if reg.Accounts[1].Default {
		t.Error("second account should not be default")
	}
}

func TestAddDuplicateEmailReturnsError(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.Add("alice@example.com", "work", "Alice Work", "", "")
	if err != ErrAccountExists {
		t.Errorf("expected ErrAccountExists, got %v", err)
	}
}

func TestAddDuplicateLabelReturnsError(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.Add("bob@example.com", "personal", "Bob", "", "")
	if err != ErrLabelInUse {
		t.Errorf("expected ErrLabelInUse, got %v", err)
	}
}

func TestAddSetsRoutingRule(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	reg, _ := store.Load()
	email, ok := reg.RoutingRules.Domains["example.com"]
	if !ok {
		t.Fatal("expected routing rule for example.com")
	}
	if email != "alice@example.com" {
		t.Errorf("expected alice@example.com, got %s", email)
	}
}

func TestAddWithPerAccountCredentials(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@corp.com", "work", "Alice", "corp-client-id", "corp-secret")

	reg, _ := store.Load()
	if reg.Accounts[0].ClientID != "corp-client-id" {
		t.Errorf("expected corp-client-id, got %s", reg.Accounts[0].ClientID)
	}
	if reg.Accounts[0].ClientSecret != "corp-secret" {
		t.Errorf("expected corp-secret, got %s", reg.Accounts[0].ClientSecret)
	}
}

func TestAddWithoutPerAccountCredentials(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	reg, _ := store.Load()
	if reg.Accounts[0].ClientID != "" {
		t.Errorf("expected empty clientId, got %s", reg.Accounts[0].ClientID)
	}
}

func TestRemoveByLabel(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	err := store.Remove("work")
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	reg, _ := store.Load()
	if len(reg.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(reg.Accounts))
	}
	if reg.Accounts[0].Email != "alice@example.com" {
		t.Error("wrong account removed")
	}
}

func TestRemoveByEmail(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.Remove("alice@example.com")
	if err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	reg, _ := store.Load()
	if len(reg.Accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(reg.Accounts))
	}
}

func TestRemoveNotFoundReturnsError(t *testing.T) {
	store := tempStore(t)
	err := store.Remove("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestRemoveDefaultPromotesNext(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	store.Remove("personal")

	reg, _ := store.Load()
	if len(reg.Accounts) != 1 {
		t.Fatalf("expected 1 account, got %d", len(reg.Accounts))
	}
	if !reg.Accounts[0].Default {
		t.Error("remaining account should be promoted to default")
	}
}

func TestRemoveCleansUpRoutingRule(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Remove("personal")

	reg, _ := store.Load()
	if _, ok := reg.RoutingRules.Domains["example.com"]; ok {
		t.Error("routing rule should be removed")
	}
}

func TestSetDefault(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	err := store.SetDefault("work")
	if err != nil {
		t.Fatalf("SetDefault() error: %v", err)
	}

	reg, _ := store.Load()
	if reg.Accounts[0].Default {
		t.Error("alice should no longer be default")
	}
	if !reg.Accounts[1].Default {
		t.Error("bob should be default")
	}
}

func TestSetDefaultByEmail(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	store.SetDefault("bob@work.com")

	reg, _ := store.Load()
	if !reg.Accounts[1].Default {
		t.Error("bob should be default")
	}
}

func TestSetDefaultNotFound(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.SetDefault("nonexistent")
	if err != ErrAccountNotFound {
		t.Errorf("expected ErrAccountNotFound, got %v", err)
	}
}

func TestGetDefault(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")

	acct, err := store.GetDefault()
	if err != nil {
		t.Fatalf("GetDefault() error: %v", err)
	}
	if acct.Email != "alice@example.com" {
		t.Errorf("expected alice, got %s", acct.Email)
	}
}

func TestGetDefaultNoAccounts(t *testing.T) {
	store := tempStore(t)
	_, err := store.GetDefault()
	if err != ErrNoAccounts {
		t.Errorf("expected ErrNoAccounts, got %v", err)
	}
}

func TestGetCredentials(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@corp.com", "work", "Bob", "corp-id", "corp-secret")

	// Account without custom creds
	cid, csec := store.GetCredentials("alice@example.com")
	if cid != "" || csec != "" {
		t.Errorf("expected empty credentials for alice, got %s/%s", cid, csec)
	}

	// Account with custom creds
	cid, csec = store.GetCredentials("bob@corp.com")
	if cid != "corp-id" || csec != "corp-secret" {
		t.Errorf("expected corp-id/corp-secret, got %s/%s", cid, csec)
	}

	// Unknown account
	cid, csec = store.GetCredentials("unknown@example.com")
	if cid != "" || csec != "" {
		t.Error("expected empty credentials for unknown account")
	}
}

func TestSaveAndLoadPersistence(t *testing.T) {
	dir := t.TempDir()
	store1 := NewStore(dir)
	store1.Add("alice@example.com", "personal", "Alice", "", "")

	// Create a new store pointing to same dir
	store2 := NewStore(dir)
	reg, err := store2.Load()
	if err != nil {
		t.Fatalf("Load() error: %v", err)
	}
	if len(reg.Accounts) != 1 {
		t.Errorf("expected 1 account from disk, got %d", len(reg.Accounts))
	}
}

func TestFilePermissions(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	path := filepath.Join(store.stateDir, "accounts.json")
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat() error: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0600 {
		t.Errorf("expected file permissions 0600, got %04o", perm)
	}
}

func TestAddDuplicateEmailCaseInsensitive(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.Add("Alice@Example.COM", "other", "Alice 2", "", "")
	if err != ErrAccountExists {
		t.Errorf("expected ErrAccountExists for case-different email, got %v", err)
	}
}

func TestAddDuplicateLabelCaseInsensitive(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	err := store.Add("bob@work.com", "Personal", "Bob", "", "")
	if err != ErrLabelInUse {
		t.Errorf("expected ErrLabelInUse for case-different label, got %v", err)
	}
}

func TestAddSameDomainDoesNotOverwriteRoutingRule(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@example.com", "work", "Bob", "", "")

	reg, _ := store.Load()
	email, ok := reg.RoutingRules.Domains["example.com"]
	if !ok {
		t.Fatal("expected routing rule for example.com")
	}
	if email != "alice@example.com" {
		t.Errorf("routing rule should still point to alice, got %s", email)
	}
}

func TestAddDomainRoutingNormalizesCase(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@Example.COM", "personal", "Alice", "", "")

	reg, _ := store.Load()
	if _, ok := reg.RoutingRules.Domains["example.com"]; !ok {
		t.Error("expected routing rule stored with lowercase domain key")
	}
	if _, ok := reg.RoutingRules.Domains["Example.COM"]; ok {
		t.Error("domain key should be lowercase, not original case")
	}
}

func TestServicesField(t *testing.T) {
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")

	reg, _ := store.Load()
	services := reg.Accounts[0].Services
	expected := []string{"mail", "calendar", "drive"}
	if len(services) != len(expected) {
		t.Fatalf("expected %d services, got %d", len(expected), len(services))
	}
	for i, s := range expected {
		if services[i] != s {
			t.Errorf("service[%d]: expected %s, got %s", i, s, services[i])
		}
	}
}

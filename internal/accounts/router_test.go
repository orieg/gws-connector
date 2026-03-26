package accounts

import (
	"strings"
	"testing"
)

func setupRouter(t *testing.T) (*Router, *Store) {
	t.Helper()
	store := tempStore(t)
	store.Add("alice@example.com", "personal", "Alice", "", "")
	store.Add("bob@work.com", "work", "Bob", "", "")
	store.SetDefault("personal")
	return NewRouter(store), store
}

func TestResolveEmptyReturnsDefault(t *testing.T) {
	router, _ := setupRouter(t)

	acct, err := router.Resolve("")
	if err != nil {
		t.Fatalf("Resolve('') error: %v", err)
	}
	if acct.Email != "alice@example.com" {
		t.Errorf("expected default (alice), got %s", acct.Email)
	}
}

func TestResolveByLabel(t *testing.T) {
	router, _ := setupRouter(t)

	acct, err := router.Resolve("work")
	if err != nil {
		t.Fatalf("Resolve('work') error: %v", err)
	}
	if acct.Email != "bob@work.com" {
		t.Errorf("expected bob@work.com, got %s", acct.Email)
	}
}

func TestResolveByEmail(t *testing.T) {
	router, _ := setupRouter(t)

	acct, err := router.Resolve("bob@work.com")
	if err != nil {
		t.Fatalf("Resolve('bob@work.com') error: %v", err)
	}
	if acct.Label != "work" {
		t.Errorf("expected label 'work', got %s", acct.Label)
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	router, _ := setupRouter(t)

	acct, err := router.Resolve("WORK")
	if err != nil {
		t.Fatalf("Resolve('WORK') error: %v", err)
	}
	if acct.Email != "bob@work.com" {
		t.Errorf("expected bob@work.com, got %s", acct.Email)
	}

	acct, err = router.Resolve("Bob@Work.com")
	if err != nil {
		t.Fatalf("Resolve('Bob@Work.com') error: %v", err)
	}
	if acct.Label != "work" {
		t.Errorf("expected label 'work', got %s", acct.Label)
	}
}

func TestResolveNotFoundShowsAvailable(t *testing.T) {
	router, _ := setupRouter(t)

	_, err := router.Resolve("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent account")
	}
	errMsg := err.Error()
	if !strings.Contains(errMsg, "not found") {
		t.Errorf("error should mention 'not found': %s", errMsg)
	}
	if !strings.Contains(errMsg, "personal") || !strings.Contains(errMsg, "work") {
		t.Errorf("error should list available accounts: %s", errMsg)
	}
}

func TestResolveNoAccountsConfigured(t *testing.T) {
	store := tempStore(t)
	router := NewRouter(store)

	_, err := router.Resolve("")
	if err == nil {
		t.Fatal("expected error when no accounts configured")
	}
	if !strings.Contains(err.Error(), "no accounts configured") {
		t.Errorf("expected 'no accounts configured' message, got: %s", err.Error())
	}
}

func TestResolveByDomain(t *testing.T) {
	router, _ := setupRouter(t)

	acct, err := router.ResolveByDomain("work.com")
	if err != nil {
		t.Fatalf("ResolveByDomain('work.com') error: %v", err)
	}
	if acct.Email != "bob@work.com" {
		t.Errorf("expected bob@work.com, got %s", acct.Email)
	}
}

func TestResolveByDomainNotFound(t *testing.T) {
	router, _ := setupRouter(t)

	_, err := router.ResolveByDomain("unknown.com")
	if err == nil {
		t.Fatal("expected error for unknown domain")
	}
}

func TestListAccounts(t *testing.T) {
	router, _ := setupRouter(t)

	accts, err := router.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error: %v", err)
	}
	if len(accts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accts))
	}
}

func TestListAccountsEmpty(t *testing.T) {
	store := tempStore(t)
	router := NewRouter(store)

	accts, err := router.ListAccounts()
	if err != nil {
		t.Fatalf("ListAccounts() error: %v", err)
	}
	if len(accts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accts))
	}
}

func TestResolveAfterDefaultChange(t *testing.T) {
	router, store := setupRouter(t)

	store.SetDefault("work")

	acct, err := router.Resolve("")
	if err != nil {
		t.Fatalf("Resolve('') error: %v", err)
	}
	if acct.Email != "bob@work.com" {
		t.Errorf("expected new default (bob), got %s", acct.Email)
	}
}

package auth

import "testing"

// TestScopesContainsSheetsAndDocs guards against a future refactor that
// accidentally drops the Sheets or Docs scope — which would silently break
// users who upgrade because tokens would be minted without the scope and
// every Sheets/Docs call would 403.
func TestScopesContainsSheetsAndDocs(t *testing.T) {
	have := map[string]bool{}
	for _, s := range Scopes {
		have[s] = true
	}
	required := []string{
		"https://www.googleapis.com/auth/gmail.modify",
		"https://www.googleapis.com/auth/calendar",
		"https://www.googleapis.com/auth/drive",
		ScopeSheets,
		ScopeDocs,
		"https://www.googleapis.com/auth/userinfo.email",
		"https://www.googleapis.com/auth/userinfo.profile",
	}
	for _, r := range required {
		if !have[r] {
			t.Errorf("Scopes is missing required scope %q", r)
		}
	}
}

func TestScopeSheetsAndDocsConstantsMatchEntry(t *testing.T) {
	if ScopeSheets != "https://www.googleapis.com/auth/spreadsheets" {
		t.Errorf("ScopeSheets constant drifted: %s", ScopeSheets)
	}
	if ScopeDocs != "https://www.googleapis.com/auth/documents" {
		t.Errorf("ScopeDocs constant drifted: %s", ScopeDocs)
	}
}

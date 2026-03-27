package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// Scopes requested for all accounts.
var Scopes = []string{
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// UserInfo holds the authenticated user's profile.
type UserInfo struct {
	Email       string
	DisplayName string
}

// OAuthFlow runs an interactive OAuth2 authorization for a new account.
// It starts a local HTTP server, opens the browser, and waits for the callback.
func OAuthFlow(ctx context.Context, clientID, clientSecret string) (*oauth2.Token, *UserInfo, error) {
	// Bind a free port — keep the listener open to prevent TOCTOU races.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, nil, fmt.Errorf("finding free port: %w", err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	redirectURL := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       Scopes,
		Endpoint:     google.Endpoint,
		RedirectURL:  redirectURL,
	}

	// Generate state for CSRF protection
	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		listener.Close()
		return nil, nil, fmt.Errorf("generating state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	// Channel to receive the auth code
	codeCh := make(chan string, 1)
	errCh := make(chan error, 1)

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("state") != state {
			errCh <- fmt.Errorf("invalid state parameter — possible CSRF attack")
			http.Error(w, "Invalid state", http.StatusBadRequest)
			return
		}
		if errMsg := r.URL.Query().Get("error"); errMsg != "" {
			errCh <- fmt.Errorf("OAuth error: %s — %s", errMsg, r.URL.Query().Get("error_description"))
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprintf(w, "<html><body><h2>Authorization failed</h2><p>%s</p><p>You can close this window.</p></body></html>",
				strings.ReplaceAll(errMsg, "<", "&lt;")) // XSS-safe
			return
		}
		code := r.URL.Query().Get("code")
		if code == "" {
			errCh <- fmt.Errorf("no authorization code received")
			http.Error(w, "No code", http.StatusBadRequest)
			return
		}
		codeCh <- code
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, "<html><body><h2>Authorization successful!</h2><p>You can close this window and return to your editor.</p></body></html>")
	})

	server := &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// Serve on the already-bound listener (no TOCTOU gap)
	go func() {
		if err := server.Serve(listener); err != http.ErrServerClosed {
			errCh <- fmt.Errorf("callback server: %w", err)
		}
	}()
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		server.Shutdown(shutdownCtx)
	}()

	// Open browser
	authURL := conf.AuthCodeURL(state,
		oauth2.AccessTypeOffline,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	if err := openBrowser(authURL); err != nil {
		return nil, nil, fmt.Errorf("opening browser: %w — please open this URL manually:\n%s", err, authURL)
	}

	// Wait for callback (with timeout)
	select {
	case code := <-codeCh:
		token, err := conf.Exchange(ctx, code)
		if err != nil {
			return nil, nil, fmt.Errorf("exchanging code for token: %w", err)
		}

		// Fetch user info
		info, err := fetchUserInfo(ctx, conf, token)
		if err != nil {
			return nil, nil, fmt.Errorf("fetching user info: %w", err)
		}

		return token, info, nil

	case err := <-errCh:
		return nil, nil, err

	case <-time.After(5 * time.Minute):
		return nil, nil, fmt.Errorf("authorization timed out after 5 minutes")

	case <-ctx.Done():
		return nil, nil, ctx.Err()
	}
}

func fetchUserInfo(ctx context.Context, conf *oauth2.Config, token *oauth2.Token) (*UserInfo, error) {
	svc, err := googleoauth2.NewService(ctx, option.WithTokenSource(conf.TokenSource(ctx, token)))
	if err != nil {
		return nil, err
	}
	info, err := svc.Userinfo.V2.Me.Get().Do()
	if err != nil {
		return nil, err
	}
	name := info.Name
	if name == "" {
		name = info.Email
	}
	return &UserInfo{
		Email:       info.Email,
		DisplayName: name,
	}, nil
}

func openBrowser(url string) error {
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", url).Start()
	case "linux":
		return exec.Command("xdg-open", url).Start()
	case "windows":
		return exec.Command("cmd", "/c", "start", url).Start()
	default:
		return fmt.Errorf("unsupported OS %q for opening browser — open this URL manually", runtime.GOOS)
	}
}

// BuildOAuth2Config creates an oauth2.Config for the given credentials.
func BuildOAuth2Config(clientID, clientSecret string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes:       Scopes,
		Endpoint:     google.Endpoint,
	}
}

// TokenSourceForAccount creates a reusable token source that auto-refreshes.
func TokenSourceForAccount(ctx context.Context, clientID, clientSecret string, token *oauth2.Token) oauth2.TokenSource {
	conf := BuildOAuth2Config(clientID, clientSecret)
	return conf.TokenSource(ctx, token)
}

// ExtractDomain returns the domain part of an email address.
func ExtractDomain(email string) string {
	parts := strings.SplitN(email, "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

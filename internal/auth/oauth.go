package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"html"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	googleoauth2 "google.golang.org/api/oauth2/v2"
	"google.golang.org/api/option"
)

// Scopes requested for all accounts.
//
// Adding or removing a scope is user-visible: existing users must re-auth to
// pick up new scopes, and the consent screen shows the full set. Update
// README.md and skills/configure/SKILL.md in lockstep so the rationale table
// stays accurate.
var Scopes = []string{
	"https://www.googleapis.com/auth/gmail.modify",
	"https://www.googleapis.com/auth/calendar",
	"https://www.googleapis.com/auth/drive",
	"https://www.googleapis.com/auth/spreadsheets",
	"https://www.googleapis.com/auth/documents",
	"https://www.googleapis.com/auth/userinfo.email",
	"https://www.googleapis.com/auth/userinfo.profile",
}

// ScopeSheets is the Google Sheets API full-access scope.
const ScopeSheets = "https://www.googleapis.com/auth/spreadsheets"

// ScopeDocs is the Google Docs API full-access scope.
const ScopeDocs = "https://www.googleapis.com/auth/documents"

// UserInfo holds the authenticated user's profile.
type UserInfo struct {
	Email       string
	DisplayName string
}

// openBrowserFn is overridable for tests. Guarded by openBrowserMu so parallel
// tests (t.Parallel, concurrent packages) that swap it don't race with the
// production read in StartOAuthFlow.
var (
	openBrowserMu sync.RWMutex
	openBrowserFn = openBrowser
)

func getOpenBrowserFn() func(string) error {
	openBrowserMu.RLock()
	defer openBrowserMu.RUnlock()
	return openBrowserFn
}

// SetOpenBrowserForTest overrides the browser-launching function and returns
// a restore func. Tests only.
func SetOpenBrowserForTest(fn func(string) error) func() {
	openBrowserMu.Lock()
	prev := openBrowserFn
	openBrowserFn = fn
	openBrowserMu.Unlock()
	return func() {
		openBrowserMu.Lock()
		openBrowserFn = prev
		openBrowserMu.Unlock()
	}
}

// flowTimeout is how long the background goroutine waits for the user to
// complete the browser consent before failing the flow.
const flowTimeout = 10 * time.Minute

// PendingFlow is a non-blocking OAuth authorization in progress.
//
// The caller starts the flow with StartOAuthFlow (which binds a local
// callback server, opens the browser, and returns immediately), then polls
// Wait with a short timeout until the flow completes. This avoids holding
// an MCP tool call open while the user interacts with their browser.
type PendingFlow struct {
	ID      string
	AuthURL string

	conf     *oauth2.Config
	listener net.Listener
	server   *http.Server
	state    string

	codeCh chan string
	errCh  chan error

	closeOnce sync.Once
	closed    chan struct{}

	mu     sync.Mutex
	done   bool
	doneCh chan struct{}
	token  *oauth2.Token
	info   *UserInfo
	err    error
}

// StartOAuthFlow binds a local callback server, opens the browser, and
// returns a PendingFlow the caller can poll via Wait. It does NOT block
// on the user's browser interaction.
func StartOAuthFlow(clientID, clientSecret string) (*PendingFlow, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, fmt.Errorf("finding free port: %w", err)
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

	stateBytes := make([]byte, 16)
	if _, err := rand.Read(stateBytes); err != nil {
		listener.Close()
		return nil, fmt.Errorf("generating state: %w", err)
	}
	state := hex.EncodeToString(stateBytes)

	idBytes := make([]byte, 8)
	if _, err := rand.Read(idBytes); err != nil {
		listener.Close()
		return nil, fmt.Errorf("generating id: %w", err)
	}

	p := &PendingFlow{
		ID:       hex.EncodeToString(idBytes),
		AuthURL:  conf.AuthCodeURL(state, oauth2.AccessTypeOffline, oauth2.SetAuthURLParam("prompt", "consent")),
		conf:     conf,
		listener: listener,
		state:    state,
		codeCh:   make(chan string, 1),
		errCh:    make(chan error, 1),
		closed:   make(chan struct{}),
		doneCh:   make(chan struct{}),
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", p.handleCallback)
	p.server = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	go func() {
		if err := p.server.Serve(listener); err != http.ErrServerClosed {
			select {
			case p.errCh <- fmt.Errorf("callback server: %w", err):
			default:
			}
		}
	}()

	if err := getOpenBrowserFn()(p.AuthURL); err != nil {
		// Non-fatal — the caller gets AuthURL to share with the user.
		fmt.Fprintf(os.Stderr, "gws-connector: could not open browser automatically: %v\n", err)
	}

	go p.watch()

	return p, nil
}

func (p *PendingFlow) handleCallback(w http.ResponseWriter, r *http.Request) {
	if r.URL.Query().Get("state") != p.state {
		select {
		case p.errCh <- fmt.Errorf("invalid state parameter — possible CSRF attack"):
		default:
		}
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}
	if errMsg := r.URL.Query().Get("error"); errMsg != "" {
		select {
		case p.errCh <- fmt.Errorf("OAuth error: %s — %s", errMsg, r.URL.Query().Get("error_description")):
		default:
		}
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, oauthPageHTML("Authorization Failed", fmt.Sprintf("Error: %s", errMsg), true))
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		select {
		case p.errCh <- fmt.Errorf("no authorization code received"):
		default:
		}
		http.Error(w, "No code", http.StatusBadRequest)
		return
	}
	select {
	case p.codeCh <- code:
	default:
	}
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, oauthPageHTML("Authorization Successful",
		"Your Google account has been connected. You can close this window and return to your editor.", false))
}

// watch runs in its own goroutine until a code, error, timeout, or Close.
func (p *PendingFlow) watch() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go func() {
		select {
		case <-p.closed:
			cancel()
		case <-ctx.Done():
		}
	}()

	select {
	case code := <-p.codeCh:
		tok, err := p.conf.Exchange(ctx, code)
		if err != nil {
			p.finalize(nil, nil, fmt.Errorf("exchanging code for token: %w", err))
			return
		}
		info, err := fetchUserInfo(ctx, p.conf, tok)
		if err != nil {
			p.finalize(nil, nil, fmt.Errorf("fetching user info: %w", err))
			return
		}
		p.finalize(tok, info, nil)
	case err := <-p.errCh:
		p.finalize(nil, nil, err)
	case <-time.After(flowTimeout):
		p.finalize(nil, nil, fmt.Errorf("authorization timed out after %s", flowTimeout))
	case <-p.closed:
		p.finalize(nil, nil, fmt.Errorf("authorization cancelled"))
	}
}

func (p *PendingFlow) finalize(tok *oauth2.Token, info *UserInfo, err error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.done {
		return
	}
	p.done = true
	p.token = tok
	p.info = info
	p.err = err
	close(p.doneCh)
}

// Wait blocks for at most timeout waiting for the flow to finish.
//
// If the flow completed (successfully or with an error), returns done=true
// with the token/info/err. If the timeout expired first, returns done=false
// with no error — the caller should call Wait again later.
func (p *PendingFlow) Wait(ctx context.Context, timeout time.Duration) (done bool, token *oauth2.Token, info *UserInfo, err error) {
	timer := time.NewTimer(timeout)
	defer timer.Stop()
	select {
	case <-p.doneCh:
		p.mu.Lock()
		defer p.mu.Unlock()
		return true, p.token, p.info, p.err
	case <-timer.C:
		return false, nil, nil, nil
	case <-ctx.Done():
		return false, nil, nil, ctx.Err()
	}
}

// Close shuts down the callback server and cancels the flow. Safe to call
// multiple times.
func (p *PendingFlow) Close() {
	p.closeOnce.Do(func() {
		close(p.closed)
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = p.server.Shutdown(shutdownCtx)
	})
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
	if os.Getenv("GWS_NO_BROWSER") == "1" {
		return nil
	}
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

// oauthPageHTML returns a styled HTML page for the OAuth callback result.
func oauthPageHTML(title, message string, isError bool) string {
	color := "#1a73e8"
	icon := "&#10003;" // checkmark
	if isError {
		color = "#d93025"
		icon = "&#10007;" // X mark
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>GWS Connector — %s</title>
<style>
  * { margin: 0; padding: 0; box-sizing: border-box; }
  body {
    font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
    display: flex; align-items: center; justify-content: center;
    min-height: 100vh; background: #f8f9fa; color: #202124;
  }
  .card {
    background: #fff; border-radius: 12px; padding: 48px;
    box-shadow: 0 1px 3px rgba(0,0,0,.12), 0 1px 2px rgba(0,0,0,.06);
    text-align: center; max-width: 420px; width: 90%%;
  }
  .icon {
    width: 64px; height: 64px; border-radius: 50%%;
    background: %s; color: #fff; font-size: 32px;
    display: inline-flex; align-items: center; justify-content: center;
    margin-bottom: 24px;
  }
  h1 { font-size: 22px; font-weight: 600; margin-bottom: 12px; }
  p  { font-size: 15px; line-height: 1.5; color: #5f6368; }
  .hint { margin-top: 24px; font-size: 13px; color: #9aa0a6; }
</style>
</head>
<body>
<div class="card">
  <div class="icon">%s</div>
  <h1>%s</h1>
  <p>%s</p>
  <p class="hint">GWS Connector for Claude Code</p>
</div>
</body>
</html>`, html.EscapeString(title), color, icon, html.EscapeString(title), html.EscapeString(message))
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

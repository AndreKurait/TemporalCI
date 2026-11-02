package main

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

type Session struct {
	Token       string    `json:"-"`
	GitHubUser  string    `json:"githubUser"`
	GitHubLogin string    `json:"githubLogin"`
	AvatarURL   string    `json:"avatarUrl"`
	ExpiresAt   time.Time `json:"expiresAt"`
}

type contextKey string

const userContextKey contextKey = "user"

var (
	sessions = struct {
		sync.RWMutex
		m map[string]*Session
	}{m: make(map[string]*Session)}
	publicRead      = os.Getenv("PUBLIC_READ") == "true"
	ghClientID      = os.Getenv("GITHUB_CLIENT_ID")
	ghClientSecret  = os.Getenv("GITHUB_CLIENT_SECRET")
	oauthHTTPClient = &http.Client{Timeout: 10 * time.Second}
	oauthStates     = struct {
		sync.Mutex
		m map[string]time.Time
	}{m: make(map[string]time.Time)}
)

func generateSessionID() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic("cryptographic randomness unavailable")
	}
	return hex.EncodeToString(b)
}

func setSessionCookie(w http.ResponseWriter, sid string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    sid,
		Path:     "/",
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   86400 * 7,
	})
}

func secureCookies() bool {
	// Opt out only for development over loopback HTTP.
	return os.Getenv("SESSION_COOKIE_SECURE") != "false"
}

func allowedUser(login string) bool {
	if login == "" {
		return false
	}
	for _, candidate := range strings.Split(os.Getenv("GITHUB_ALLOWED_USERS"), ",") {
		if strings.EqualFold(strings.TrimSpace(candidate), login) {
			return true
		}
	}
	return false
}

func getSessionFromRequest(r *http.Request) *Session {
	cookie, err := r.Cookie("session")
	if err != nil {
		return nil
	}
	sessions.RLock()
	defer sessions.RUnlock()
	s := sessions.m[cookie.Value]
	if s != nil && (time.Now().After(s.ExpiresAt) || !allowedUser(s.GitHubLogin)) {
		return nil
	}
	return s
}

// handleAuthGitHub redirects to GitHub OAuth authorize URL.
func handleAuthGitHub(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if ghClientID == "" || ghClientSecret == "" || strings.TrimSpace(os.Getenv("GITHUB_ALLOWED_USERS")) == "" {
		http.Error(w, "GitHub OAuth not configured", http.StatusServiceUnavailable)
		return
	}
	now := time.Now()
	state := generateSessionID()
	oauthStates.Lock()
	for nonce, expiry := range oauthStates.m {
		if now.After(expiry) {
			delete(oauthStates.m, nonce)
		}
	}
	if len(oauthStates.m) >= 1024 {
		oauthStates.Unlock()
		http.Error(w, "too many login attempts", http.StatusTooManyRequests)
		return
	}
	oauthStates.m[state] = now.Add(10 * time.Minute)
	oauthStates.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "oauth_state", Value: state, Path: "/auth/github",
		HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode, MaxAge: 600,
	})
	query := url.Values{"client_id": {ghClientID}, "scope": {"read:user"}, "state": {state}}
	http.Redirect(w, r, "https://github.com/login/oauth/authorize?"+query.Encode(), http.StatusTemporaryRedirect)
}

// handleAuthCallback exchanges the OAuth code for a token and creates a session.
func handleAuthCallback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	state := r.URL.Query().Get("state")
	cookie, cookieErr := r.Cookie("oauth_state")
	if cookieErr != nil || state == "" || subtle.ConstantTimeCompare([]byte(state), []byte(cookie.Value)) != 1 {
		http.Error(w, "invalid OAuth state", http.StatusUnauthorized)
		return
	}
	oauthStates.Lock()
	expiry, exists := oauthStates.m[state]
	delete(oauthStates.m, state)
	oauthStates.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: "oauth_state", Path: "/auth/github", MaxAge: -1,
		HttpOnly: true, Secure: secureCookies(), SameSite: http.SameSiteLaxMode,
	})
	if !exists || time.Now().After(expiry) {
		http.Error(w, "expired OAuth state", http.StatusUnauthorized)
		return
	}

	code := r.URL.Query().Get("code")
	if code == "" {
		http.Error(w, "missing code", http.StatusBadRequest)
		return
	}

	token, err := exchangeCode(code)
	if err != nil {
		slog.Error("oauth exchange failed", "error", err)
		http.Error(w, "authentication failed", http.StatusUnauthorized)
		return
	}
	user, err := fetchGitHubUser(token)
	if err != nil {
		slog.Error("failed to fetch github user", "error", err)
		http.Error(w, "failed to fetch user", http.StatusInternalServerError)
		return
	}
	if !allowedUser(user.Login) {
		http.Error(w, "GitHub user is not allowed", http.StatusForbidden)
		return
	}

	sid := generateSessionID()
	sess := &Session{
		Token:       token,
		GitHubLogin: user.Login,
		GitHubUser:  user.Name,
		AvatarURL:   user.AvatarURL,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	sessions.Lock()
	sessions.m[sid] = sess
	sessions.Unlock()

	setSessionCookie(w, sid)
	http.Redirect(w, r, "/dashboard", http.StatusTemporaryRedirect)
}

// handleAuthMe returns the current user info.
func handleAuthMe(w http.ResponseWriter, r *http.Request) {
	sess := getSessionFromRequest(r)
	if sess == nil {
		http.Error(w, "not authenticated", http.StatusUnauthorized)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sess)
}

// handleAuthLogout clears the session.
func handleAuthLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	cookie, err := r.Cookie("session")
	if err == nil {
		sessions.Lock()
		delete(sessions.m, cookie.Value)
		sessions.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name:     "session",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   secureCookies(),
		SameSite: http.SameSiteLaxMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

// authMiddleware requires a valid session.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sess := getSessionFromRequest(r)
		if sess == nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		ctx := context.WithValue(r.Context(), userContextKey, sess)
		next(w, r.WithContext(ctx))
	}
}

// optionalAuth sets user in context if session exists, but doesn't block.
func optionalAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if sess := getSessionFromRequest(r); sess != nil {
			ctx := context.WithValue(r.Context(), userContextKey, sess)
			r = r.WithContext(ctx)
		}
		next(w, r)
	}
}

// readAuth requires auth unless PUBLIC_READ is true.
func readAuth(next http.HandlerFunc) http.HandlerFunc {
	if publicRead {
		return optionalAuth(next)
	}
	return authMiddleware(next)
}

type ghUser struct {
	Login     string `json:"login"`
	Name      string `json:"name"`
	AvatarURL string `json:"avatar_url"`
}

func exchangeCode(code string) (string, error) {
	payload := fmt.Sprintf(`{"client_id":%q,"client_secret":%q,"code":%q}`, ghClientID, ghClientSecret, code)
	req, _ := http.NewRequest("POST", "https://github.com/login/oauth/access_token", strings.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("OAuth endpoint returned status %d", resp.StatusCode)
	}

	var result struct {
		AccessToken string `json:"access_token"`
		Error       string `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("oauth error: %s", result.Error)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("OAuth response did not include an access token")
	}
	return result.AccessToken, nil
}

func fetchGitHubUser(token string) (*ghUser, error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/user", nil)
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := oauthHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub user endpoint returned status %d", resp.StatusCode)
	}

	var u ghUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, err
	}
	if u.Login == "" {
		return nil, fmt.Errorf("GitHub response did not include a user login")
	}
	return &u, nil
}

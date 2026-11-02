package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

func TestManagementRoutesRejectAnonymousRequests(t *testing.T) {
	previous := publicRead
	publicRead = true // Public build reads must never expose management routes.
	t.Cleanup(func() { publicRead = previous })
	handler := newHTTPHandler()
	for _, tc := range []struct{ method, path string }{
		{"GET", "/api/repos"},
		{"POST", "/api/repos"},
		{"GET", "/api/repos/owner/repo"},
		{"DELETE", "/api/repos/owner/repo"},
		{"POST", "/api/trigger/owner/repo"},
		{"GET", "/api/locks"},
		{"DELETE", "/api/locks/resource"},
		{"POST", "/api/lock-pools"},
		{"GET", "/api/artifacts/owner/repo/run"},
		{"POST", "/api/replay/run"},
		{"POST", "/webhook/custom/owner/repo"},
		{"GET", "/dashboard"},
		{"POST", "/api/ci/notifications/read"},
	} {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			handler.ServeHTTP(response, httptest.NewRequest(tc.method, tc.path, nil))
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401; body: %s", response.Code, response.Body.String())
			}
		})
	}
}

func TestWebhookRequiresConfiguredSecretAndValidSignature(t *testing.T) {
	previous := webhookSecret
	t.Cleanup(func() { webhookSecret = previous })
	body := `{"zen":"test ping"}`
	for _, tc := range []struct {
		name, secret, signature string
		want                    int
	}{
		{"unconfigured", "", "", http.StatusServiceUnavailable},
		{"unsigned", "test-secret", "", http.StatusUnauthorized},
		{"wrong signature", "test-secret", "sha256=invalid", http.StatusUnauthorized},
		{"signed ping", "test-secret", signedBody(body, "test-secret"), http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			webhookSecret = tc.secret
			req := httptest.NewRequest(http.MethodPost, "/webhook", strings.NewReader(body))
			req.Header.Set("X-GitHub-Event", "ping")
			req.Header.Set("X-Hub-Signature-256", tc.signature)
			response := httptest.NewRecorder()
			newHTTPHandler().ServeHTTP(response, req)
			if response.Code != tc.want {
				t.Fatalf("got %d, want %d: %s", response.Code, tc.want, response.Body.String())
			}
		})
	}
}

func signedBody(body, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(body))
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

type oauthTransport func(*http.Request) (*http.Response, error)

func (f oauthTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func configureOAuthTest(t *testing.T) {
	t.Helper()
	clientID, clientSecret, client := ghClientID, ghClientSecret, oauthHTTPClient
	ghClientID, ghClientSecret = "test-client", "test-client-secret"
	t.Setenv("GITHUB_ALLOWED_USERS", "operator")
	t.Setenv("SESSION_COOKIE_SECURE", "true")
	t.Cleanup(func() {
		ghClientID, ghClientSecret, oauthHTTPClient = clientID, clientSecret, client
		oauthStates.Lock()
		oauthStates.m = make(map[string]time.Time)
		oauthStates.Unlock()
	})
}

func startOAuthTest(t *testing.T) (*http.Cookie, string) {
	t.Helper()
	response := httptest.NewRecorder()
	handleAuthGitHub(response, httptest.NewRequest(http.MethodGet, "https://ci.example/auth/github", nil))
	if response.Code != http.StatusTemporaryRedirect {
		t.Fatalf("OAuth start returned %d: %s", response.Code, response.Body.String())
	}
	location, err := url.Parse(response.Header().Get("Location"))
	if err != nil {
		t.Fatal(err)
	}
	cookies := response.Result().Cookies()
	if len(cookies) != 1 || !cookies[0].Secure || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteLaxMode {
		t.Fatalf("OAuth state cookie is not secured: %v", cookies)
	}
	state := location.Query().Get("state")
	if len(state) != 64 || state != cookies[0].Value {
		t.Fatal("authorization URL and browser cookie must carry the same random state")
	}
	return cookies[0], state
}

func TestOAuthStateCheckedBeforeTokenExchange(t *testing.T) {
	configureOAuthTest(t)
	calls := 0
	oauthHTTPClient = &http.Client{Transport: oauthTransport(func(*http.Request) (*http.Response, error) {
		calls++
		t.Fatal("invalid state must not contact the token endpoint")
		return nil, nil
	})}
	for _, tc := range []struct{ name, cookie, query string }{
		{"missing", "", ""},
		{"missing cookie", "", "nonce"},
		{"mismatch", "nonce", "other"},
		{"unissued", "nonce", "nonce"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=code&state="+tc.query, nil)
			if tc.cookie != "" {
				req.AddCookie(&http.Cookie{Name: "oauth_state", Value: tc.cookie})
			}
			response := httptest.NewRecorder()
			handleAuthCallback(response, req)
			if response.Code != http.StatusUnauthorized {
				t.Fatalf("got %d, want 401", response.Code)
			}
		})
	}
	if calls != 0 {
		t.Fatal("unexpected token exchange")
	}
}

func TestOAuthAllowlistAndSingleUseState(t *testing.T) {
	configureOAuthTest(t)
	for _, login := range []string{"outsider", "operator"} {
		t.Run(login, func(t *testing.T) {
			oauthHTTPClient = &http.Client{Transport: oauthTransport(func(req *http.Request) (*http.Response, error) {
				body := `{"access_token":"test-access-token"}`
				if req.URL.Path == "/user" {
					body = `{"login":"` + login + `","name":"Test User"}`
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
			})}
			cookie, state := startOAuthTest(t)
			callback := func() *httptest.ResponseRecorder {
				req := httptest.NewRequest(http.MethodGet, "/auth/github/callback?code=code&state="+state, nil)
				req.AddCookie(cookie)
				response := httptest.NewRecorder()
				handleAuthCallback(response, req)
				return response
			}
			response := callback()
			want := http.StatusForbidden
			if login == "operator" {
				want = http.StatusTemporaryRedirect
			}
			if response.Code != want {
				t.Fatalf("got %d, want %d: %s", response.Code, want, response.Body.String())
			}
			for _, sessionCookie := range response.Result().Cookies() {
				if sessionCookie.Name == "session" {
					if login != "operator" || !sessionCookie.Secure || !sessionCookie.HttpOnly {
						t.Fatal("only allowed users may receive a secure session")
					}
					sessions.Lock()
					delete(sessions.m, sessionCookie.Value)
					sessions.Unlock()
				}
			}
			if replay := callback(); replay.Code != http.StatusUnauthorized {
				t.Fatalf("reused OAuth state returned %d", replay.Code)
			}
		})
	}
}

func TestExistingSessionRevokedWhenOperatorRemoved(t *testing.T) {
	t.Setenv("GITHUB_ALLOWED_USERS", "operator")
	sid := "revoked-operator-session"
	sessions.Lock()
	sessions.m[sid] = &Session{GitHubLogin: "operator", ExpiresAt: time.Now().Add(time.Hour)}
	sessions.Unlock()
	t.Cleanup(func() {
		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	})
	req := httptest.NewRequest(http.MethodGet, "/api/repos", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sid})
	if getSessionFromRequest(req) == nil {
		t.Fatal("allowed operator should have a session")
	}
	t.Setenv("GITHUB_ALLOWED_USERS", "")
	if getSessionFromRequest(req) != nil {
		t.Fatal("empty allowlist must revoke access")
	}
}

func TestBadgeEscapesMarkup(t *testing.T) {
	if svg := badgeSVG("build", `<script>alert("x")</script>`, "#999"); strings.Contains(svg, "<script>") {
		t.Fatal("badge rendered unescaped markup")
	}
}

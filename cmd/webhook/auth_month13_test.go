package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// --- Month 13: GitHub OAuth, session management, readAuth middleware ---

func TestReadAuth_PublicReadMode(t *testing.T) {
	// Save and restore
	origPublicRead := publicRead
	defer func() { publicRead = origPublicRead }()

	publicRead = true
	called := false
	handler := readAuth(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	// No cookie — should still pass with publicRead=true
	req := httptest.NewRequest("GET", "/api/ci/builds", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("handler should be called with publicRead=true and no cookie")
	}
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

func TestReadAuth_PrivateMode(t *testing.T) {
	origPublicRead := publicRead
	defer func() { publicRead = origPublicRead }()

	publicRead = false
	handler := readAuth(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// No cookie — should be rejected
	req := httptest.NewRequest("GET", "/api/ci/builds", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestSessionCookie_SetAndRead(t *testing.T) {
	sid := generateSessionID()
	sessions.Lock()
	sessions.m[sid] = &Session{
		GitHubLogin: "testuser",
		AvatarURL:   "https://avatars.example.com/u/1",
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	}
	sessions.Unlock()
	defer func() {
		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	}()

	// Simulate setting cookie
	w := httptest.NewRecorder()
	setSessionCookie(w, sid)

	cookies := w.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("expected session cookie")
	}
	if cookies[0].Name != "session" {
		t.Errorf("cookie name = %q", cookies[0].Name)
	}
	if !cookies[0].HttpOnly {
		t.Error("cookie should be HttpOnly")
	}

	// Read it back
	req := httptest.NewRequest("GET", "/", nil)
	req.AddCookie(cookies[0])
	sess := getSessionFromRequest(req)
	if sess == nil || sess.GitHubLogin != "testuser" {
		t.Errorf("session = %v", sess)
	}
}

func TestAuthMe_Unauthenticated(t *testing.T) {
	req := httptest.NewRequest("GET", "/auth/me", nil)
	w := httptest.NewRecorder()
	handleAuthMe(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Errorf("got %d, want 401", w.Code)
	}
}

func TestAuthLogout_ClearsSession(t *testing.T) {
	sid := "logout-test"
	sessions.Lock()
	sessions.m[sid] = &Session{GitHubLogin: "user", ExpiresAt: time.Now().Add(time.Hour)}
	sessions.Unlock()

	req := httptest.NewRequest("POST", "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: sid})
	w := httptest.NewRecorder()
	handleAuthLogout(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("got %d, want 204", w.Code)
	}

	sessions.RLock()
	_, exists := sessions.m[sid]
	sessions.RUnlock()
	if exists {
		t.Error("session should be deleted after logout")
	}
}

func TestAuthLogout_WrongMethod(t *testing.T) {
	req := httptest.NewRequest("GET", "/auth/logout", nil)
	w := httptest.NewRecorder()
	handleAuthLogout(w, req)
	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("got %d, want 405", w.Code)
	}
}

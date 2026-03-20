package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSessionLifecycle(t *testing.T) {
	t.Run("create_and_get", func(t *testing.T) {
		sid := generateSessionID()
		sessions.Lock()
		sessions.m[sid] = &Session{
			GitHubLogin: "testuser",
			ExpiresAt:   time.Now().Add(time.Hour),
		}
		sessions.Unlock()

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: sid})
		sess := getSessionFromRequest(req)
		if sess == nil || sess.GitHubLogin != "testuser" {
			t.Errorf("expected testuser, got %v", sess)
		}

		// cleanup
		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	})

	t.Run("expired", func(t *testing.T) {
		sid := "expired-session"
		sessions.Lock()
		sessions.m[sid] = &Session{
			GitHubLogin: "old",
			ExpiresAt:   time.Now().Add(-time.Hour),
		}
		sessions.Unlock()

		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: sid})
		if getSessionFromRequest(req) != nil {
			t.Error("expected nil for expired session")
		}

		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	})

	t.Run("missing_cookie", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		if getSessionFromRequest(req) != nil {
			t.Error("expected nil for missing cookie")
		}
	})
}

func TestGenerateSessionID(t *testing.T) {
	t.Run("length", func(t *testing.T) {
		id := generateSessionID()
		if len(id) != 64 { // 32 bytes = 64 hex chars
			t.Errorf("len = %d, want 64", len(id))
		}
	})

	t.Run("unique", func(t *testing.T) {
		id1 := generateSessionID()
		id2 := generateSessionID()
		if id1 == id2 {
			t.Error("two calls returned the same ID")
		}
	})
}

func TestAuthMiddlewareFunc(t *testing.T) {
	sid := "auth-test-session"
	sessions.Lock()
	sessions.m[sid] = &Session{
		GitHubLogin: "authuser",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	sessions.Unlock()
	defer func() {
		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	}()

	handler := authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	t.Run("no_cookie_401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", w.Code)
		}
	})

	t.Run("invalid_session_401", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: "bogus"})
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusUnauthorized {
			t.Errorf("got %d, want 401", w.Code)
		}
	})

	t.Run("valid_session_200", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: sid})
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("got %d, want 200", w.Code)
		}
	})
}

func TestOptionalAuthFunc(t *testing.T) {
	sid := "opt-auth-session"
	sessions.Lock()
	sessions.m[sid] = &Session{
		GitHubLogin: "optuser",
		ExpiresAt:   time.Now().Add(time.Hour),
	}
	sessions.Unlock()
	defer func() {
		sessions.Lock()
		delete(sessions.m, sid)
		sessions.Unlock()
	}()

	t.Run("no_cookie_passes", func(t *testing.T) {
		var userInCtx interface{}
		handler := optionalAuth(func(w http.ResponseWriter, r *http.Request) {
			userInCtx = r.Context().Value(userContextKey)
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest("GET", "/", nil)
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("got %d, want 200", w.Code)
		}
		if userInCtx != nil {
			t.Errorf("expected no user, got %v", userInCtx)
		}
	})

	t.Run("valid_session_sets_user", func(t *testing.T) {
		var userInCtx interface{}
		handler := optionalAuth(func(w http.ResponseWriter, r *http.Request) {
			userInCtx = r.Context().Value(userContextKey)
			w.WriteHeader(http.StatusOK)
		})
		req := httptest.NewRequest("GET", "/", nil)
		req.AddCookie(&http.Cookie{Name: "session", Value: sid})
		w := httptest.NewRecorder()
		handler(w, req)
		if w.Code != http.StatusOK {
			t.Errorf("got %d, want 200", w.Code)
		}
		sess, ok := userInCtx.(*Session)
		if !ok || sess.GitHubLogin != "optuser" {
			t.Errorf("expected optuser in context, got %v", userInCtx)
		}
	})
}

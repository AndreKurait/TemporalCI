package middleware

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestRateLimiter_Allow(t *testing.T) {
	rl := NewRateLimiter(3, time.Minute)

	// First 3 should pass
	for i := 0; i < 3; i++ {
		if !rl.Allow("1.2.3.4") {
			t.Errorf("request %d should be allowed", i+1)
		}
	}
	// 4th should be blocked
	if rl.Allow("1.2.3.4") {
		t.Error("4th request should be rate limited")
	}
}

func TestRateLimiter_DifferentKeys(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)

	if !rl.Allow("1.1.1.1") {
		t.Error("first IP should be allowed")
	}
	if !rl.Allow("2.2.2.2") {
		t.Error("second IP should be allowed (different key)")
	}
}

func TestRateLimit_Handler(t *testing.T) {
	rl := NewRateLimiter(1, time.Minute)
	handler := RateLimit(rl, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// First request OK
	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "1.2.3.4:1234"
	w := httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("first request: got %d, want 200", w.Code)
	}

	// Second request rate limited
	w = httptest.NewRecorder()
	handler(w, req)
	if w.Code != http.StatusTooManyRequests {
		t.Errorf("second request: got %d, want 429", w.Code)
	}
}

func TestAuditLog_PassesThrough(t *testing.T) {
	called := false
	handler := AuditLog(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})

	req := httptest.NewRequest("POST", "/webhook", nil)
	w := httptest.NewRecorder()
	handler(w, req)

	if !called {
		t.Error("inner handler should be called")
	}
	if w.Code != http.StatusOK {
		t.Errorf("got %d, want 200", w.Code)
	}
}

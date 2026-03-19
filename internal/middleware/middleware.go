package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter implements a simple token-bucket rate limiter per IP.
type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    int           // tokens per interval
	interval time.Duration
}

type bucket struct {
	tokens int
	last   time.Time
}

// NewRateLimiter creates a rate limiter allowing `rate` requests per `interval`.
func NewRateLimiter(rate int, interval time.Duration) *RateLimiter {
	return &RateLimiter{
		buckets:  make(map[string]*bucket),
		rate:     rate,
		interval: interval,
	}
}

// Allow checks if a request from the given key is allowed.
func (rl *RateLimiter) Allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	b, ok := rl.buckets[key]
	if !ok {
		rl.buckets[key] = &bucket{tokens: rl.rate - 1, last: time.Now()}
		return true
	}

	elapsed := time.Since(b.last)
	refill := int(elapsed / rl.interval) * rl.rate
	b.tokens += refill
	if b.tokens > rl.rate {
		b.tokens = rl.rate
	}
	b.last = time.Now()

	if b.tokens <= 0 {
		return false
	}
	b.tokens--
	return true
}

// RateLimit wraps an http.Handler with rate limiting.
func RateLimit(limiter *RateLimiter, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.RemoteAddr
		if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
			key = fwd
		}
		if !limiter.Allow(key) {
			http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
			return
		}
		next(w, r)
	}
}

// AuditLog logs HTTP requests for audit purposes.
func AuditLog(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slog.Info("audit",
			"method", r.Method,
			"path", r.URL.Path,
			"remote", r.RemoteAddr,
			"userAgent", r.UserAgent(),
		)
		next(w, r)
	}
}

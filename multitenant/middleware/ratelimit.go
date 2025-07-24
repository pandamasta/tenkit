package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimiter limits requests per IP within a time window.
type RateLimiter struct {
	visits    map[string]int
	limit     int
	window    time.Duration
	lastReset time.Time
	mu        sync.Mutex
}

// NewRateLimiter creates a new rate limiter with specified limit and window.
func NewRateLimiter(limit int, window time.Duration) *RateLimiter {
	return &RateLimiter{
		visits:    make(map[string]int),
		limit:     limit,
		window:    window,
		lastReset: time.Now(),
	}
}

// RateLimit applies rate limiting to the handler.
func RateLimit(next http.Handler) http.Handler {
	rl := NewRateLimiter(10, time.Minute)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rl.mu.Lock()
		defer rl.mu.Unlock()

		// Step 1: Reset visits if window has passed
		if time.Since(rl.lastReset) > rl.window {
			slog.Debug("[RATELIMIT] Resetting visits")
			rl.visits = make(map[string]int)
			rl.lastReset = time.Now()
		}

		// Step 2: Check rate limit for client IP
		ip := r.RemoteAddr
		rl.visits[ip]++
		if rl.visits[ip] > rl.limit {
			slog.Warn("[RATELIMIT] Rate limit exceeded", "ip", ip, "count", rl.visits[ip])
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		// Step 3: Proceed to next handler
		slog.Debug("[RATELIMIT] Allowing request", "ip", ip, "count", rl.visits[ip])
		next.ServeHTTP(w, r)
	})
}

package middleware

import (
	"log/slog"
	"net/http"
	"sync"
	"time"
)

// RateLimit applies rate limiting to the handler (10 requests per minute).
func RateLimit(next http.Handler) http.Handler {
	visits := make(map[string]int)
	var mu sync.Mutex
	lastReset := time.Now()
	limit := 10
	window := time.Minute

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()

		// Step 1: Reset visits if window has passed
		if time.Since(lastReset) > window {
			slog.Debug("[RATELIMIT] Resetting visits")
			visits = make(map[string]int)
			lastReset = time.Now()
		}

		// Step 2: Check rate limit for client IP
		ip := r.RemoteAddr
		visits[ip]++
		if visits[ip] > limit {
			slog.Warn("[RATELIMIT] Rate limit exceeded", "ip", ip, "count", visits[ip])
			http.Error(w, "Too many requests", http.StatusTooManyRequests)
			return
		}

		// Step 3: Proceed to next handler
		slog.Debug("[RATELIMIT] Allowing request", "ip", ip, "count", visits[ip])
		next.ServeHTTP(w, r)
	})
}

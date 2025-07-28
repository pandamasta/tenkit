package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
)

// CSRFMiddleware adds CSRF protection to POST requests.
func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("[CSRF] Processing request", "method", r.Method, "path", r.URL.Path, "host", r.Host)

		// Step 1: Try to get existing CSRF cookie
		var token string
		cookie, err := r.Cookie("csrf_token")
		if err != nil || cookie.Value == "" {
			// No valid cookie, generate one
			token, err = generateCSRFToken()
			if err != nil {
				slog.Error("[CSRF] Token generation failed", "error", err)
				http.Error(w, "Internal error", http.StatusInternalServerError)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     "csrf_token",
				Value:    token,
				HttpOnly: false, // Must be false to allow form access
				Secure:   false, // Set to true in production with HTTPS
				SameSite: http.SameSiteLaxMode,
				Path:     "/",
			})
			slog.Debug("[CSRF] CSRF token created and set", "token", token, "path", r.URL.Path)
		} else {
			token = cookie.Value
			slog.Debug("[CSRF] Reusing existing CSRF token", "token", token, "path", r.URL.Path)
		}

		// Step 2: Store in context for handler
		ctx := context.WithValue(r.Context(), CsrfKey, token)
		r = r.WithContext(ctx)

		// Step 3: Validate CSRF token for POST, PUT, DELETE requests
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			slog.Debug("[CSRF] Validating CSRF for POST/PUT/DELETE", "path", r.URL.Path)
			if err := r.ParseForm(); err != nil {
				slog.Error("[CSRF] Failed to parse form", "error", err, "path", r.URL.Path)
				http.Error(w, "Invalid form submission", http.StatusBadRequest)
				return
			}
			formToken := r.FormValue("csrf_token")
			if formToken == "" {
				slog.Warn("[CSRF] Missing CSRF token in form", "path", r.URL.Path)
				http.Error(w, "CSRF token missing in form", http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(formToken)) != 1 {
				slog.Warn("[CSRF] Invalid CSRF token", "form_token", formToken, "expected_token", token, "path", r.URL.Path)
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
			slog.Debug("[CSRF] Valid CSRF token", "token", formToken, "path", r.URL.Path)
		}

		// Step 4: Proceed to next handler
		slog.Debug("[CSRF] Proceeding to next handler", "path", r.URL.Path)
		next.ServeHTTP(w, r)
	})
}

func generateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

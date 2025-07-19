package middleware

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"log/slog"
	"net/http"
)

func CSRFMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try to get existing CSRF cookie
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
				HttpOnly: true,
				Secure:   false,
				SameSite: http.SameSiteStrictMode,
			})
			slog.Info("[CSRF] CSRF token created and set", "path", r.URL.Path)
		} else {
			token = cookie.Value
			slog.Debug("[CSRF] Reusing existing CSRF token", "path", r.URL.Path)
		}

		// Store in context for GET handler
		ctx := context.WithValue(r.Context(), CsrfKey, token)
		r = r.WithContext(ctx)

		// If it's a modifying request, verify token
		if r.Method == http.MethodPost || r.Method == http.MethodPut || r.Method == http.MethodDelete {
			formToken := r.FormValue("csrf_token")
			if formToken == "" {
				slog.Warn("[CSRF] Missing CSRF token in form", "path", r.URL.Path)
				http.Error(w, "CSRF token missing in form", http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(token), []byte(formToken)) != 1 {
				slog.Warn("[CSRF] Invalid CSRF token", "path", r.URL.Path)
				http.Error(w, "Invalid CSRF token", http.StatusForbidden)
				return
			}
			slog.Info("[CSRF] Valid CSRF token", "path", r.URL.Path)
		}

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

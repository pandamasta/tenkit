package middleware

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
)

func Logger(cfg *multitenant.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Load user from session token if present
		if cookie, err := r.Cookie(cfg.SessionCookie.Name); err == nil {
			if user, err := models.GetSession(cookie.Value); err == nil && user != nil {
				r = r.WithContext(context.WithValue(r.Context(), userKey, user))
			}
		}

		// Tenant
		t := FromContext(r.Context())
		tenantInfo := "main"
		if t != nil {
			tenantInfo = t.Subdomain
		}

		next.ServeHTTP(w, r)
		log.Printf("[HTTP][%s] %s %s from %s - %v",
			tenantInfo, r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

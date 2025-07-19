package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
)

func SessionMiddleware(cfg *multitenant.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context() // Start with current ctx to propagate outer values like CSRF
		cookie, err := r.Cookie(cfg.SessionCookie.Name)
		if err == nil && cookie.Value != "" {
			slog.Info("[SESSION] Found cookie", "value", cookie.Value)
			user, err := models.GetSession(cookie.Value)
			if err == nil && user != nil {
				// Optional: Add tenant check for security (if not already in GetSession)
				t := FromContext(r.Context()) // Assuming FromContext from tenant.go
				if t != nil && user.TenantID != t.ID {
					slog.Warn("[SESSION] Mismatch tenant for user", "user_id", user.ID, "expected_tenant_id", t.ID, "got_tenant_id", user.TenantID)
					http.SetCookie(w, &http.Cookie{Name: cfg.SessionCookie.Name, MaxAge: -1}) // Clear invalid cookie
					next.ServeHTTP(w, r)
					return
				}
				slog.Info("[SESSION] Resolved userID", "user_id", user.ID)
				ctx = context.WithValue(ctx, userIDKey, user.ID)
				ctx = context.WithValue(ctx, userKey, user)
			} else {
				slog.Warn("[SESSION] Invalid/expired session", "err", err)
				http.SetCookie(w, &http.Cookie{Name: cfg.SessionCookie.Name, MaxAge: -1}) // Clear on error
			}
		} else {
			slog.Info("[SESSION] No session cookie in request")
		}
		r = r.WithContext(ctx) // Always attach updated ctx to propagate (e.g., CSRF token)
		next.ServeHTTP(w, r)
	})
}

func CurrentUserID(r *http.Request) int64 {
	if uid, ok := r.Context().Value(userIDKey).(int64); ok {
		return uid
	}
	return 0
}

func CurrentUser(r *http.Request) *models.User {
	if u, ok := r.Context().Value(userKey).(*models.User); ok {
		return u
	}
	return nil
}

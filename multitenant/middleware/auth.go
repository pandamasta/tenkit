package middleware

import (
	"net/http"
)

// RequireAuth ensures the user is logged in
func RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user := CurrentUser(r)
		if user == nil {
			http.Redirect(w, r, "/login?error=auth", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

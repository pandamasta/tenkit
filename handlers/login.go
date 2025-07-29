package handlers

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"

	"golang.org/x/crypto/bcrypt"
)

// LoginHandler handles GET and POST requests for /login.
func LoginHandler(cfg *multitenant.Config, i18n *i18n.I18n, baseTmpl *template.Template) http.HandlerFunc {
	tmpl, err := baseTmpl.Clone()
	if err != nil {
		slog.Error("[LOGIN] Failed to clone base template", "err", err)
		os.Exit(1) // Or panic
	}
	tmpl, err = tmpl.ParseFiles("templates/login.html")
	if err != nil {
		slog.Error("[LOGIN] Failed to parse login template", "err", err)
		os.Exit(1)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Restrict to tenant domains, return 404 if on marketing domain
		if !middleware.IsTenantRequest(r.Context()) {
			slog.Warn("[LOGIN] Login attempted on marketing domain", "host", r.Host)
			http.NotFound(w, r)
			return
		}

		// Step 2: Retrieve tenant from context
		t := middleware.FromContext(r.Context())
		if t == nil {
			slog.Error("[LOGIN] Tenant context missing")
			http.NotFound(w, r)
			return
		}

		// Step 3: Handle GET request to serve the login form
		if r.Method == http.MethodGet {
			data := render.BaseTemplateData(r, i18n, nil)
			if errorKey := r.URL.Query().Get("error"); errorKey != "" {
				data.Extra = map[string]any{
					"Error": i18n.T("login.error."+errorKey, lang),
				}
			}
			slog.Debug("[LOGIN] Rendering login form", "lang", lang, "tenant", t.Subdomain)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Parse form data from POST request
		if err := r.ParseForm(); err != nil {
			slog.Error("[LOGIN] Invalid form", "err", err)
			renderError(w, r, tmpl, i18n, lang, "login.error.InvalidForm")
			return
		}

		// Step 5: Extract submitted values
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		pass := r.FormValue("password")

		// Step 6: Validate required fields
		if email == "" || pass == "" {
			slog.Warn("[LOGIN] Missing required fields", "email", email)
			renderError(w, r, tmpl, i18n, lang, "login.error.MissingFields")
			return
		}

		// Step 7: Look up user by email and tenant
		user, err := models.GetUserByEmailAndTenant(email, t.ID)
		if err != nil {
			slog.Error("[LOGIN] DB error", "email", email, "tenant", t.Subdomain, "err", err)
			renderError(w, r, tmpl, i18n, lang, "login.error.Internal")
			return
		}
		if user == nil {
			slog.Info("[LOGIN] No user found", "email", email, "tenant", t.Subdomain)
			renderError(w, r, tmpl, i18n, lang, "login.error.InvalidCreds")
			return
		}

		// Step 8: Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)); err != nil {
			slog.Info("[LOGIN] Wrong password", "email", email, "tenant", t.Subdomain)
			renderError(w, r, tmpl, i18n, lang, "login.error.InvalidCreds")
			return
		}

		// Step 9: Create session token
		token := models.CreateSession(user.ID, user.TenantID)

		// Step 10: Set session cookie
		cookie := http.Cookie{
			Name:     cfg.SessionCookie.Name,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   cfg.SessionCookie.Secure,
			SameSite: cfg.SessionCookie.SameSite,
			Expires:  time.Now().Add(cfg.TokenExpiry),
		}
		http.SetCookie(w, &cookie)

		// Step 11: Log success and redirect with 302
		slog.Info("[LOGIN] User logged in", "email", email, "tenant", t.Subdomain)
		w.Header().Set("Location", "/dashboard")
		w.WriteHeader(http.StatusFound) // Use 302 Found instead of 303 See Other
	}
}

// LogoutHandler handles GET requests for /logout.
func LogoutHandler(cfg *multitenant.Config, i18n *i18n.I18n) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Clear session cookie
		cookie := http.Cookie{
			Name:     cfg.SessionCookie.Name,
			Value:    "",
			Path:     "/",
			HttpOnly: true,
			Expires:  time.Unix(0, 0),
		}
		http.SetCookie(w, &cookie)

		// Step 2: Redirect to home
		http.Redirect(w, r, "/", http.StatusSeeOther)
	}
}

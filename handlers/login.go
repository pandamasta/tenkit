package handlers

import (
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"

	"golang.org/x/crypto/bcrypt"
)

// InitLoginTemplates parses the templates needed for the login page.
// It includes header, base layout, and login-specific content.
func InitLoginTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/login.html")...)
	if err != nil {
		slog.Error("[LOGIN] Failed to parse login template", "err", err)
		panic(err)
	}
	return tmpl
}

// LoginHandler handles GET and POST requests for /login.
func LoginHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Redirect to default tenant if on marketing domain
		if !middleware.IsTenantRequest(r.Context()) {
			slog.Info("[LOGIN] Redirecting from main domain to tenant login", "host", r.Host)
			http.Redirect(w, r, "http://default-tenant."+cfg.Domain+"/login", http.StatusSeeOther)
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
			// Step 4: Prepare data for template
			data := render.BaseTemplateData(r, i18n, nil)
			// Step 5: Check for error in query params (from redirect)
			if errorKey := r.URL.Query().Get("error"); errorKey != "" {
				data.Extra = map[string]any{
					"Error": i18n.T("login.error."+errorKey, lang),
				}
			}
			slog.Debug("[LOGIN] Rendering login form", "lang", lang)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 6: Parse form data from POST request
		if err := r.ParseForm(); err != nil {
			slog.Error("[LOGIN] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.InvalidForm", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Extract submitted values
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		pass := r.FormValue("password")

		// Step 8: Validate required fields
		if email == "" || pass == "" {
			slog.Warn("[LOGIN] Missing required fields", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.MissingFields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 9: Look up user by email and tenant
		user, err := models.GetUserByEmailAndTenant(email, t.ID)
		if err != nil {
			slog.Error("[LOGIN] DB error", "email", email, "tenant", t.Subdomain, "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.Internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		if user == nil {
			slog.Info("[LOGIN] No user found", "email", email, "tenant", t.Subdomain)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.InvalidCreds", lang),
			})
			w.WriteHeader(http.StatusUnauthorized)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)); err != nil {
			slog.Info("[LOGIN] Wrong password", "email", email, "tenant", t.Subdomain)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.InvalidCreds", lang),
			})
			w.WriteHeader(http.StatusUnauthorized)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 11: Create session token
		token := models.CreateSession(user.ID, user.TenantID)

		// Step 12: Set session cookie
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

		// Step 13: Log success and redirect
		slog.Info("[LOGIN] User logged in", "email", email, "tenant", t.Subdomain)
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
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

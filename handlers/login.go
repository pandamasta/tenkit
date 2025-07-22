package handlers

import (
	"html/template"
	"log/slog"
	"net/http"
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

		// Step 1: Handle GET request to serve the login form
		if r.Method == http.MethodGet {
			// Step 2: Prepare data for template
			data := render.BaseTemplateData(r, i18n, nil)
			// Step 3: Render login form
			slog.Debug("[LOGIN] Rendering login form", "lang", lang)
			// Check for error in query params (from redirect)
			if errorKey := r.URL.Query().Get("error"); errorKey != "" {
				data.Extra = map[string]any{
					"Error": i18n.T("login.error."+errorKey, lang),
				}
			}
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Parse form data from POST request
		if err := r.ParseForm(); err != nil {
			slog.Error("[LOGIN] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.InvalidForm", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 5: Extract submitted values
		email := r.FormValue("email")
		pass := r.FormValue("password")

		// Step 6: Validate required fields
		if email == "" || pass == "" {
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.MissingFields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Retrieve tenant from context
		t := middleware.FromContext(r.Context())
		if t == nil {
			slog.Error("[LOGIN] Tenant context missing", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.TenantNotFound", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 8: Look up user by email and tenant
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

		// Step 9: Verify password
		if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)); err != nil {
			slog.Info("[LOGIN] Wrong password", "email", email, "tenant", t.Subdomain)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("login.error.InvalidCreds", lang),
			})
			w.WriteHeader(http.StatusUnauthorized)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Create session token
		token := models.CreateSession(user.ID, user.TenantID)

		// Step 11: Set session cookie
		cookie := http.Cookie{
			Name:     cfg.SessionCookie.Name,
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			Secure:   false, // Set to true in production
			Expires:  time.Now().Add(cfg.TokenExpiry),
		}
		http.SetCookie(w, &cookie)

		// Step 12: Log success and redirect
		slog.Info("[LOGIN] User logged in", "email", email, "tenant", t.Subdomain)
		http.Redirect(w, r, "/", http.StatusSeeOther)
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

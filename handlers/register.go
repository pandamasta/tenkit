package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

// InitRegisterTemplates parses the templates needed for the register page.
// It includes header, base layout, and register-specific content.
func InitRegisterTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/register.html")...)
	if err != nil {
		slog.Error("[REGISTER] Failed to parse register template", "err", err)
		panic(err)
	}
	return tmpl
}

// RegisterHandler handles GET and POST requests for /register.
func RegisterHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Retrieve tenant from context
		tCtx := middleware.FromContext(r.Context())
		if tCtx == nil {
			slog.Error("[REGISTER] Tenant context missing")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.no_tenant", lang),
			})
			w.WriteHeader(http.StatusForbidden)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Handle GET request to serve the register form
		if r.Method == http.MethodGet {
			data := render.BaseTemplateData(r, i18n, nil)
			slog.Debug("[REGISTER] Rendering register form", "lang", lang, "tenant", tCtx.Subdomain)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 3: Parse the form data for POST requests
		if err := r.ParseForm(); err != nil {
			slog.Error("[REGISTER] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.invalid_form", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Extract and validate form data
		email := r.FormValue("email")
		password := r.FormValue("password")
		if email == "" || password == "" {
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.missing_fields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 5: Start transaction
		tx, err := db.DB.Begin()
		if err != nil {
			slog.Error("[REGISTER] Failed to start transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		defer tx.Rollback() // Rollback if not committed

		// Step 6: Check for existing pending signups
		var exists int
		err = tx.QueryRow(`
			SELECT COUNT(*) 
			FROM pending_user_signups 
			WHERE email = ? AND tenant_id = ?`, email, tCtx.ID).Scan(&exists)
		if err != nil {
			slog.Error("[REGISTER] DB error checking pending signups", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		if exists > 0 {
			slog.Info("[REGISTER] Already registered", "email", email, "tenant", tCtx.Subdomain)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.already_registered", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Hash password with bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("[REGISTER] Password hashing error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 8: Generate token and insert pending signup
		token, err := utils.GenerateUserToken(email, tCtx.ID, time.Now().Add(24*time.Hour))
		if err != nil {
			slog.Error("[REGISTER] Token generation error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		_, err = tx.Exec(`
			INSERT INTO pending_user_signups (email, tenant_id, password_hash, token, expires_at)
			VALUES (?, ?, ?, ?, ?)`, email, tCtx.ID, string(hash), token, time.Now().Add(24*time.Hour))
		if err != nil {
			slog.Error("[REGISTER] Failed to insert pending signup", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 9: Commit transaction
		if err := tx.Commit(); err != nil {
			slog.Error("[REGISTER] Failed to commit transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Generate confirmation link and log
		link := fmt.Sprintf("http://%s.%s/confirm?token=%s", tCtx.Subdomain, cfg.Domain, token)
		slog.Info("[REGISTER] Sent confirm link", "email", email, "link", link)

		// Step 11: Render success message
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Success": i18n.T("register.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

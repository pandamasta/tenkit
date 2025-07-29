package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

// RegisterHandler handles GET and POST requests for /register.
func RegisterHandler(cfg *multitenant.Config, i18n *i18n.I18n, baseTmpl *template.Template) http.HandlerFunc {
	tmpl, err := baseTmpl.Clone()
	if err != nil {
		slog.Error("[REGISTER] Failed to clone base template", "err", err)
		os.Exit(1)
	}
	tmpl, err = tmpl.ParseFiles("templates/login.html")
	if err != nil {
		slog.Error("[REGISTER] Failed to parse login template", "err", err)
		os.Exit(1)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Restrict to tenant domains, return 404 if not tenant
		if !middleware.IsTenantRequest(r.Context()) {
			slog.Warn("[REGISTER] Registration attempted on non-tenant domain", "host", r.Host)
			http.NotFound(w, r)
			return
		}

		// Step 2: Retrieve tenant from context
		tCtx := middleware.FromContext(r.Context())
		if tCtx == nil {
			slog.Error("[REGISTER] Tenant context missing")
			http.NotFound(w, r)
			return
		}

		// Step 3: Handle GET request to serve the register form
		if r.Method == http.MethodGet {
			data := render.BaseTemplateData(r, i18n, nil)
			slog.Debug("[REGISTER] Rendering register form", "lang", lang, "tenant", tCtx.Subdomain)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Parse the form data for POST requests
		if err := r.ParseForm(); err != nil {
			slog.Error("[REGISTER] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.invalid_form", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 5: Extract and validate form data
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		password := r.FormValue("password")
		if email == "" || password == "" {
			slog.Warn("[REGISTER] Missing required fields", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.missing_fields", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 6: Validate email format
		if !emailRegex.MatchString(email) {
			slog.Warn("[REGISTER] Invalid email format", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.invalid_email", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Validate password policy
		if !isValidPassword(password) {
			slog.Warn("[REGISTER] Invalid password format", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.invalid_password", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 8: Start transaction
		tx, err := db.DB.Begin()
		if err != nil {
			slog.Error("[REGISTER] Failed to start transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		defer tx.Rollback() // Rollback if not committed

		// Step 9: Check for existing pending signups
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
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		if exists > 0 {
			slog.Info("[REGISTER] Already registered", "email", email, "tenant", tCtx.Subdomain)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.already_registered", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Hash password with bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("[REGISTER] Password hashing error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 11: Generate token and insert pending signup
		token, err := utils.GenerateUserToken(email, tCtx.ID, time.Now().Add(24*time.Hour))
		if err != nil {
			slog.Error("[REGISTER] Token generation error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
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
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 12: Commit transaction
		if err := tx.Commit(); err != nil {
			slog.Error("[REGISTER] Failed to commit transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("register.error.internal", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 13: Generate confirmation link and log
		link := fmt.Sprintf("http://%s.%s/confirm?token=%s", tCtx.Subdomain, cfg.Domain, token)
		slog.Info("[REGISTER] Sent confirm link", "email", email, "link", link)

		// Step 14: Render success message
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Success": i18n.T("register.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

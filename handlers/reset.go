package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"golang.org/x/crypto/bcrypt"
)

// InitResetTemplates parses the templates needed for the reset password pages.
func InitResetTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/reset.html")...)
	if err != nil {
		slog.Error("[RESET] Failed to parse reset template", "err", err)
		panic(err)
	}
	return tmpl
}

// RequestResetPasswordHandler handles GET and POST requests for password reset requests.
func RequestResetPasswordHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Handle GET request to serve the reset form
		if r.Method == http.MethodGet {
			slog.Debug("[RESET] GET request received")
			data := render.BaseTemplateData(r, i18n, nil)
			slog.Debug("[RESET] Rendering template with base layout using RenderTemplate")
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Parse the form data for POST requests
		if err := r.ParseForm(); err != nil {
			slog.Error("[RESET] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.invalid_form", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 3: Extract and validate form data
		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		if email == "" {
			slog.Warn("[RESET] Missing email", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.missing_fields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Validate email format
		if !emailRegex.MatchString(email) {
			slog.Warn("[RESET] Invalid email format", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.invalid_email", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 5: Retrieve tenant from context
		tenant := middleware.FromContext(r.Context())
		var tenantID int64
		if tenant != nil {
			tenantID = tenant.ID
		}

		// Step 6: Fetch user
		user, err := models.GetUserByEmailAndTenant(email, tenantID)
		if err != nil {
			slog.Error("[RESET] Failed to fetch user", "err", err, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		if user == nil {
			// Avoid leaking user existence
			slog.Info("[RESET] Password reset requested for non-existent user", "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Success": i18n.T("reset.success", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Generate reset token
		token := models.CreateSession(user.ID, tenantID)

		// Step 8: Store reset token
		_, err = db.DB.Exec(`
			INSERT INTO password_resets (user_id, tenant_id, token, expires_at)
			VALUES (?, ?, ?, ?)`,
			user.ID, tenantID, token, time.Now().Add(time.Hour))
		if err != nil {
			slog.Error("[RESET] Failed to store reset token", "err", err, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 9: Generate reset link and log
		var link string
		if tenant != nil {
			link = fmt.Sprintf("http://%s.%s/reset/confirm?token=%s", tenant.Subdomain, cfg.Domain, token)
		} else {
			link = fmt.Sprintf("http://%s/reset/confirm?token=%s", cfg.Domain, token)
		}
		slog.Info("[RESET] Password reset requested", "email", email, "tenant_id", tenantID, "link", link)

		// Step 10: Render success message
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Success": i18n.T("reset.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

// ResetPasswordHandler handles GET and POST requests to reset the password.
func ResetPasswordHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Handle GET request to serve the reset confirm form
		if r.Method == http.MethodGet {
			slog.Debug("[RESET] GET request received for confirm")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Token": r.URL.Query().Get("token"),
			})
			slog.Debug("[RESET] Rendering template with base layout using RenderTemplate")
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Parse the form data for POST requests
		if err := r.ParseForm(); err != nil {
			slog.Error("[RESET] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.invalid_form", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 3: Extract and validate form data
		token := r.FormValue("token")
		password := r.FormValue("password")
		if token == "" || password == "" {
			slog.Warn("[RESET] Missing required fields")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.missing_fields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Validate password policy
		if !isValidPassword(password) {
			slog.Warn("[RESET] Invalid password format")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.invalid_password", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 5: Fetch reset token
		var userID, tenantID int64
		row := db.LogQueryRow(r.Context(), db.DB,
			`SELECT user_id, tenant_id FROM password_resets WHERE token = ? AND expires_at > ?`,
			token, time.Now())
		if err := row.Scan(&userID, &tenantID); err != nil {
			slog.Error("[RESET] Invalid or expired token", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.invalid_token", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 6: Hash password with bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("[RESET] Password hashing error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Update password
		_, err = db.DB.Exec(`
			UPDATE users SET password_hash = ? WHERE id = ? AND tenant_id = ?`,
			hash, userID, tenantID)
		if err != nil {
			slog.Error("[RESET] Failed to update password", "err", err, "user_id", userID)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("reset.error.internal", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 8: Invalidate reset token
		_, err = db.DB.Exec(`DELETE FROM password_resets WHERE token = ?`, token)
		if err != nil {
			slog.Warn("[RESET] Failed to delete reset token", "err", err)
		}

		// Step 9: Log success and redirect
		slog.Info("[RESET] Password reset successful", "user_id", userID, "tenant_id", tenantID)
		http.Redirect(w, r, "/login?message=reset", http.StatusSeeOther)
	}
}

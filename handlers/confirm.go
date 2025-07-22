package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

// InitConfirmTemplates parses the templates needed for the confirm page.
// It includes header, base layout, and confirm-specific content.
func InitConfirmTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/confirm.html")...)
	if err != nil {
		slog.Error("[CONFIRM] Failed to parse confirm template", "err", err)
		panic(err)
	}
	return tmpl
}

// ConfirmHandler handles user confirmation via token.
func ConfirmHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Validate the token
		token := r.URL.Query().Get("token")
		email, tid, ok := utils.ValidateUserToken(token)
		if !ok {
			slog.Info("[CONFIRM] Invalid or expired token")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.invalid_token", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Check for pending signup in DB
		var ph string
		err := db.DB.QueryRow(`
			SELECT password_hash FROM pending_user_signups WHERE token = ? AND tenant_id = ?`, token, tid).Scan(&ph)
		if err != nil {
			slog.Info("[CONFIRM] No signup found for email=%s, tid=%d", "email", email, "tid", tid)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.not_found", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 3: Insert user and membership, delete pending signup
		tx, err := db.DB.Begin()
		if err != nil {
			slog.Error("[CONFIRM] Failed to start transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		defer tx.Rollback() // Rollback if not committed

		res, err := tx.Exec(`
			INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
			VALUES (?, ?, 1, ?, 'member')`, email, ph, tid)
		if err != nil {
			slog.Error("[CONFIRM] Failed to insert user", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		uid, err := res.LastInsertId()
		if err != nil {
			slog.Error("[CONFIRM] Failed to get user ID", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		_, err = tx.Exec(`INSERT INTO memberships (user_id, tenant_id, role, is_active) VALUES (?, ?, 'member', 1)`, uid, tid)
		if err != nil {
			slog.Error("[CONFIRM] Failed to insert membership", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		_, err = tx.Exec(`DELETE FROM pending_user_signups WHERE token = ?`, token)
		if err != nil {
			slog.Error("[CONFIRM] Failed to delete pending signup", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		if err := tx.Commit(); err != nil {
			slog.Error("[CONFIRM] Failed to commit transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("confirm.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Render success message
		slog.Info("[CONFIRM] User confirmed: %s (tenant %d)", "email", email, "tid", tid)
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Message": i18n.T("confirm.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

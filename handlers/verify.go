package handlers

import (
	"database/sql"
	"html/template"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

// VerifyHandler handles tenant verification via token.
func VerifyHandler(cfg *multitenant.Config, i18n *i18n.I18n, baseTmpl *template.Template) http.HandlerFunc {
	tmpl, err := baseTmpl.Clone()
	if err != nil {
		slog.Error("[VERIFY] Failed to clone base template", "err", err)
		os.Exit(1) // Or panic
	}
	tmpl, err = tmpl.ParseFiles("templates/login.html")
	if err != nil {
		slog.Error("[VERIFY] Failed to parse login template", "err", err)
		os.Exit(1)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Validate the token
		token := r.URL.Query().Get("token")
		email, org, ok := utils.ValidateSignupToken(token)
		if !ok {
			slog.Info("[VERIFY] Invalid or expired token")
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("verify.invalid_token", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Normalize email and subdomain
		email = strings.ToLower(strings.TrimSpace(email))
		sub := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(org), " ", ""))
		slog.Info("[VERIFY] Verifying", "email", email, "org", org, "subdomain", sub)

		// Step 3: Get password hash from pending signups
		var ph string
		err := db.DB.QueryRow(`SELECT password_hash FROM pending_tenant_signups WHERE token = ?`, token).Scan(&ph)
		if err == sql.ErrNoRows {
			slog.Info("[VERIFY] Token already used or not found", "org", org, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("verify.link_already_used", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		} else if err != nil {
			slog.Error("[VERIFY] DB error reading signup token", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Start transaction
		tx, err := db.DB.Begin()
		if err != nil {
			slog.Error("[VERIFY] Failed to start transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		defer tx.Rollback() // Rollback if not committed

		// Step 5: Check if tenant already exists
		var tid int64
		err = tx.QueryRow(`SELECT id FROM tenants WHERE LOWER(subdomain) = LOWER(?) OR LOWER(email) = LOWER(?)`, sub, email).Scan(&tid)
		tenantExists := (err != sql.ErrNoRows)
		if err != nil && err != sql.ErrNoRows {
			slog.Error("[VERIFY] Tenant lookup DB error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 6: Check if user already exists for that tenant
		var uid int64
		userExists := false
		if tenantExists {
			err = tx.QueryRow(`SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND tenant_id = ?`, email, tid).Scan(&uid)
			userExists = (err != sql.ErrNoRows)
			if err != nil && err != sql.ErrNoRows {
				slog.Error("[VERIFY] User lookup DB error", "err", err)
				data := render.BaseTemplateData(r, i18n, map[string]any{
					"Message": i18n.T("common.internal_error", lang),
				})
				w.WriteHeader(http.StatusInternalServerError)
				render.RenderTemplate(w, tmpl, "base", data)
				return
			}
		}

		// Step 7: Handle existing tenant/user cases
		if tenantExists && userExists {
			slog.Info("[VERIFY] Tenant and user already exist", "subdomain", sub, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("verify.already_verified", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		if tenantExists && !userExists {
			slog.Info("[VERIFY] Tenant exists but user does not", "subdomain", sub, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.conflict_error", lang),
			})
			w.WriteHeader(http.StatusConflict)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 8: Create new tenant
		res, err := tx.Exec(`
			INSERT INTO tenants (name, slug, subdomain, email, is_active, is_deleted)
			VALUES (?, ?, ?, ?, 1, 0)`, org, sub, sub, email)
		if err != nil {
			slog.Error("[VERIFY] Failed to create tenant", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		tid, err = res.LastInsertId()
		if err != nil {
			slog.Error("[VERIFY] Failed to get tenant ID", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 9: Create user
		res, err = tx.Exec(`
			INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
			VALUES (?, ?, 1, ?, 'owner')`, email, ph, tid)
		if err != nil {
			slog.Error("[VERIFY] Failed to create user", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		uid, err = res.LastInsertId()
		if err != nil {
			slog.Error("[VERIFY] Failed to get user ID", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Create membership and delete pending signup
		_, err = tx.Exec(`INSERT INTO memberships (user_id, tenant_id, role, is_active) VALUES (?, ?, 'owner', 1)`, uid, tid)
		if err != nil {
			slog.Error("[VERIFY] Failed to create membership", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		_, err = tx.Exec(`DELETE FROM pending_tenant_signups WHERE token = ?`, token)
		if err != nil {
			slog.Error("[VERIFY] Failed to delete pending signup", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 11: Commit transaction
		if err := tx.Commit(); err != nil {
			slog.Error("[VERIFY] Failed to commit transaction", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Message": i18n.T("common.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 12: Render success message
		slog.Info("[VERIFY] Tenant and user created successfully", "subdomain", sub, "email", email)
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Message": i18n.T("verify.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

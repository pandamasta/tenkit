package handlers

import (
	"database/sql"
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
)

// ConfirmHandler handles GET requests for /confirm to verify user or tenant signup.
func ConfirmHandler(cfg *multitenant.Config, i18n *i18n.I18n, baseTmpl *template.Template) http.HandlerFunc {
	tmpl, err := baseTmpl.Clone()
	if err != nil {
		slog.Error("[CONFIRM] Failed to clone base template", "err", err)
		os.Exit(1) // Or panic
	}
	tmpl, err = tmpl.ParseFiles("templates/confirm.html")
	if err != nil {
		slog.Error("[CONFIRM] Failed to parse login template", "err", err)
		os.Exit(1)
	}

	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Restrict to tenant domains for user confirmation, allow marketing domain for tenant confirmation
		tenant := middleware.FromContext(r.Context())
		isTenantRequest := middleware.IsTenantRequest(r.Context())

		// Step 2: Extract token from query parameter
		token := r.URL.Query().Get("token")
		if token == "" {
			slog.Warn("[CONFIRM] Missing token")
			renderError(w, r, tmpl, i18n, lang, "confirm.error.missing_token")
			return
		}

		// Step 3: Start transaction
		tx, err := db.DB.Begin()
		if err != nil {
			slog.Error("[CONFIRM] Failed to start transaction", "err", err)
			renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
			return
		}
		defer tx.Rollback()

		// Step 4: Check for user signup in pending_user_signups (for /register)
		var userID, tenantID int64
		var email, passwordHash string
		err = tx.QueryRow(`
			SELECT email, tenant_id, password_hash 
			FROM pending_user_signups 
			WHERE token = ? AND expires_at > ?`,
			token, time.Now()).Scan(&email, &tenantID, &passwordHash)
		if err == nil && isTenantRequest {
			// Step 5: Verify tenant matches
			if tenant == nil || tenant.ID != tenantID {
				slog.Warn("[CONFIRM] Tenant mismatch", "token", token, "tenant_id", tenantID)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.invalid_token")
				return
			}

			// Step 6: Check if user already exists
			var exists int
			err = tx.QueryRow(`SELECT 1 FROM users WHERE email = ?`, email).Scan(&exists)
			if err == nil {
				slog.Warn("[CONFIRM] User already exists", "email", email)
				// Delete pending signup to prevent reuse
				_, _ = tx.Exec(`DELETE FROM pending_user_signups WHERE token = ?`, token)
				tx.Commit()
				renderError(w, r, tmpl, i18n, lang, "confirm.error.already_registered")
				return
			}
			if err != sql.ErrNoRows {
				slog.Error("[CONFIRM] Failed to check user existence", "err", err, "email", email)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 7: Insert user
			result, err := tx.Exec(`
				INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
				VALUES (?, ?, ?, ?, ?)`,
				email, passwordHash, true, tenantID, "member")
			if err != nil {
				slog.Error("[CONFIRM] Failed to insert user", "err", err, "email", email)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 8: Get inserted user ID
			userID, err = result.LastInsertId()
			if err != nil {
				slog.Error("[CONFIRM] Failed to get user ID", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 9: Insert membership
			_, err = tx.Exec(`
				INSERT INTO memberships (user_id, tenant_id, role, is_active)
				VALUES (?, ?, ?, ?)`,
				userID, tenantID, "member", true)
			if err != nil {
				slog.Error("[CONFIRM] Failed to insert membership", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 10: Delete pending signup
			_, err = tx.Exec(`DELETE FROM pending_user_signups WHERE token = ?`, token)
			if err != nil {
				slog.Warn("[CONFIRM] Failed to delete pending user signup", "err", err)
			}

			// Step 11: Commit transaction
			if err := tx.Commit(); err != nil {
				slog.Error("[CONFIRM] Failed to commit transaction", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 12: Log success and redirect
			slog.Info("[CONFIRM] User confirmed", "email", email, "tenant_id", tenantID)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Success": i18n.T("confirm.success.user", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		} else if err != sql.ErrNoRows {
			slog.Error("[CONFIRM] Failed to fetch pending user signup", "err", err)
			renderError(w, r, tmpl, i18n, lang, "confirm.error.invalid_token")
			return
		}

		// Step 13: Check for tenant signup in pending_tenant_signups (for /enroll)
		var orgName string
		err = tx.QueryRow(`
			SELECT email, org_name, password_hash 
			FROM pending_tenant_signups 
			WHERE token = ? AND expires_at > ?`,
			token, time.Now()).Scan(&email, &orgName, &passwordHash)
		if err == nil && !isTenantRequest {
			// Step 14: Check if user or tenant already exists
			var exists int
			err = tx.QueryRow(`SELECT 1 FROM users WHERE email = ?`, email).Scan(&exists)
			if err == nil {
				slog.Warn("[CONFIRM] User already exists for tenant signup", "email", email)
				// Delete pending signup to prevent reuse
				_, _ = tx.Exec(`DELETE FROM pending_tenant_signups WHERE token = ?`, token)
				tx.Commit()
				renderError(w, r, tmpl, i18n, lang, "confirm.error.already_registered")
				return
			}
			if err != sql.ErrNoRows {
				slog.Error("[CONFIRM] Failed to check user existence for tenant", "err", err, "email", email)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 15: Check for duplicate subdomain
			subdomain := strings.ToLower(strings.ReplaceAll(orgName, " ", ""))
			err = tx.QueryRow(`SELECT 1 FROM tenants WHERE subdomain = ?`, subdomain).Scan(&exists)
			if err == nil {
				slog.Warn("[CONFIRM] Subdomain already exists", "subdomain", subdomain)
				// Delete pending signup to prevent reuse
				_, _ = tx.Exec(`DELETE FROM pending_tenant_signups WHERE token = ?`, token)
				tx.Commit()
				renderError(w, r, tmpl, i18n, lang, "confirm.error.subdomain_exists")
				return
			}
			if err != sql.ErrNoRows {
				slog.Error("[CONFIRM] Failed to check subdomain existence", "err", err, "subdomain", subdomain)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 16: Insert tenant
			result, err := tx.Exec(`
				INSERT INTO tenants (name, slug, subdomain, email, is_active, allow_signins)
				VALUES (?, ?, ?, ?, ?, ?)`,
				orgName, subdomain, subdomain, email, true, true)
			if err != nil {
				slog.Error("[CONFIRM] Failed to insert tenant", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 17: Get inserted tenant ID
			tenantID, err = result.LastInsertId()
			if err != nil {
				slog.Error("[CONFIRM] Failed to get tenant ID", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 18: Insert user
			result, err = tx.Exec(`
				INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
				VALUES (?, ?, ?, ?, ?)`,
				email, passwordHash, true, tenantID, "admin")
			if err != nil {
				slog.Error("[CONFIRM] Failed to insert user", "err", err, "email", email)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 19: Get inserted user ID
			userID, err = result.LastInsertId()
			if err != nil {
				slog.Error("[CONFIRM] Failed to get user ID", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 20: Insert membership
			_, err = tx.Exec(`
				INSERT INTO memberships (user_id, tenant_id, role, is_active)
				VALUES (?, ?, ?, ?)`,
				userID, tenantID, "admin", true)
			if err != nil {
				slog.Error("[CONFIRM] Failed to insert membership", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 21: Delete pending signup
			_, err = tx.Exec(`DELETE FROM pending_tenant_signups WHERE token = ?`, token)
			if err != nil {
				slog.Warn("[CONFIRM] Failed to delete pending tenant signup", "err", err)
			}

			// Step 22: Commit transaction
			if err := tx.Commit(); err != nil {
				slog.Error("[CONFIRM] Failed to commit transaction", "err", err)
				renderError(w, r, tmpl, i18n, lang, "confirm.error.internal")
				return
			}

			// Step 23: Log success and redirect
			slog.Info("[CONFIRM] Tenant and user confirmed", "email", email, "tenant_id", tenantID)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Success": i18n.T("confirm.success.tenant", lang),
			})
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 24: Invalid token
		slog.Warn("[CONFIRM] Invalid or expired token", "token", token)
		renderError(w, r, tmpl, i18n, lang, "confirm.error.invalid_token")
	}
}

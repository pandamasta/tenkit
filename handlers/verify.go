package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"strings"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

var verifyTmpl *template.Template

func InitVerifyTemplates(base []string) {
	verifyTmpl = template.Must(template.ParseFiles(append(base, "templates/verify.html")...))
}

func VerifyHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	email, org, ok := utils.ValidateSignupToken(token)
	if !ok {
		log.Printf("[VERIFY] Invalid or expired token")
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "verify.invalid_token",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}

	// Normalize email and subdomain
	email = strings.ToLower(strings.TrimSpace(email))
	sub := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(org), " ", ""))

	log.Printf("[VERIFY] Verifying email: %s, org: %s → subdomain: %s", email, org, sub)

	// Get password hash from pending signups
	var ph string
	err := db.DB.QueryRow(`SELECT password_hash FROM pending_tenant_signups WHERE token = ?`, token).Scan(&ph)
	if err == sql.ErrNoRows {
		log.Printf("[VERIFY] Token already used or not found: %s (%s)", org, email)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "verify.link_already_used",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	} else if err != nil {
		log.Printf("[VERIFY] DB error reading signup token: %v", err)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "common.internal_error",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}

	// Check if tenant already exists
	var tid int64
	err = db.DB.QueryRow(`SELECT id FROM tenants WHERE LOWER(subdomain) = LOWER(?) OR LOWER(email) = LOWER(?)`, sub, email).Scan(&tid)
	tenantExists := (err != sql.ErrNoRows)
	if err != nil && err != sql.ErrNoRows {
		log.Printf("[VERIFY] Tenant lookup DB error: %v", err)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "common.internal_error",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}

	// Check if user already exists for that tenant
	var uid int64
	err = db.DB.QueryRow(`SELECT id FROM users WHERE LOWER(email) = LOWER(?) AND tenant_id = ?`, email, tid).Scan(&uid)
	userExists := (err != sql.ErrNoRows)

	if tenantExists && userExists {
		log.Printf("[VERIFY] Tenant and user already exist: %s (%s)", sub, email)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "verify.already_verified",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}

	if tenantExists && !userExists {
		log.Printf("[VERIFY] Tenant '%s' exists but user '%s' does not", sub, email)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "common.conflict_error",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}

	// Create new tenant
	res, err := db.DB.Exec(`
		INSERT INTO tenants (name, slug, subdomain, email, is_active, is_deleted)
		VALUES (?, ?, ?, ?, 1, 0)`, org, sub, sub, email)
	if err != nil {
		log.Printf("[VERIFY] Failed to create tenant: %v", err)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "common.internal_error",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}
	tid, _ = res.LastInsertId()

	// Create user
	res, err = db.DB.Exec(`
		INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
		VALUES (?, ?, 1, ?, 'owner')`, email, ph, tid)
	if err != nil {
		log.Printf("[VERIFY] Failed to create user: %v", err)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "common.internal_error",
		})
		utils.RenderTemplate(w, verifyTmpl, "base", data)
		return
	}
	uid, _ = res.LastInsertId()

	db.DB.Exec(`INSERT INTO memberships (user_id, tenant_id, role, is_active) VALUES (?, ?, 'owner', 1)`, uid, tid)
	db.DB.Exec(`DELETE FROM pending_tenant_signups WHERE token = ?`, token)

	log.Printf("[VERIFY] Tenant '%s' and user '%s' created successfully!", sub, email)

	data := utils.BaseTemplateData(r, map[string]interface{}{
		"MessageKey": "verify.success",
	})
	utils.RenderTemplate(w, verifyTmpl, "base", data)
}

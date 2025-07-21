package handlers

import (
	"html/template"
	"log"
	"net/http"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

var confirmTmpl *template.Template

func InitConfirmTemplates(base []string) {
	confirmTmpl = template.Must(template.ParseFiles(append(base, "templates/confirm.html")...))
}

func ConfirmHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	email, tid, ok := utils.ValidateUserToken(token)
	if !ok {
		log.Printf("[CONFIRM] Invalid or expired token")
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "confirm.invalid_token",
		})
		utils.RenderTemplate(w, confirmTmpl, "base", data)
		return
	}

	row := db.DB.QueryRow(`
        SELECT password_hash FROM pending_user_signups WHERE token = ? AND tenant_id = ?`, token, tid)
	var ph string
	if row.Scan(&ph) != nil {
		log.Printf("[CONFIRM] No signup found for email=%s, tid=%d", email, tid)
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"MessageKey": "confirm.not_found",
		})
		utils.RenderTemplate(w, confirmTmpl, "base", data)
		return
	}

	tx, _ := db.DB.Begin()
	res, _ := tx.Exec(`
        INSERT INTO users (email, password_hash, is_verified, tenant_id, role)
        VALUES (?, ?, 1, ?, 'member')`, email, ph, tid)
	uid, _ := res.LastInsertId()

	tx.Exec(`INSERT INTO memberships (user_id, tenant_id, role, is_active) VALUES (?, ?, 'member', 1)`, uid, tid)
	tx.Exec(`DELETE FROM pending_user_signups WHERE token = ?`, token)
	tx.Commit()

	log.Printf("[CONFIRM] User confirmed: %s (tenant %d)", email, tid)

	data := utils.BaseTemplateData(r, map[string]interface{}{
		"MessageKey": "confirm.success",
	})
	utils.RenderTemplate(w, confirmTmpl, "base", data)
}

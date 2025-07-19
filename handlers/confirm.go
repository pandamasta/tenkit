package handlers

import (
	"net/http"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

func ConfirmHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	email, tid, ok := utils.ValidateUserToken(token)
	if !ok {
		http.Error(w, "Invalid or expired", 400)
		return
	}
	row := db.DB.QueryRow(`
        SELECT password_hash FROM pending_user_signups WHERE token = ? AND tenant_id = ?`, token, tid)
	var ph string
	if row.Scan(&ph) != nil {
		http.Error(w, "No signup found", 400)
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

	w.Write([]byte("🎉 Email verified! You can now log in."))
}

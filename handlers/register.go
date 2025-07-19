package handlers

import (
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

var registerTmpl *template.Template

func InitRegisterTemplates(base []string) {
	registerTmpl = template.Must(template.ParseFiles(append(base, "templates/register.html")...))
}

func RegisterHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Retrieve tenant from context
	tCtx := middleware.FromContext(r.Context())
	if tCtx == nil {
		http.Error(w, "Register only allowed from tenant domains", http.StatusForbidden)
		return
	}
	// Step 2: Handle GET request to serve the register form
	if r.Method == http.MethodGet {
		// Extract CSRF token from context (set by middleware)
		csrfToken, ok := r.Context().Value(middleware.CsrfKey).(string)
		if !ok {
			slog.Error("[REGISTER] CSRF token not found in context")
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		data := struct {
			CSRFToken string
		}{
			CSRFToken: csrfToken,
		}
		registerTmpl.Execute(w, data) // Render template with CSRF data
		return
	}
	// Step 3: Parse the form data for POST requests
	r.ParseForm()
	e := r.FormValue("email")
	p := r.FormValue("password")
	// Step 4: Basic validation (add more if needed, e.g., email format)
	if e == "" || p == "" {
		http.Error(w, "Email and password required", 400)
		return
	}

	// Step 5: Check for existing pending signups
	var exists int
	db.DB.QueryRow(`SELECT COUNT(*) FROM pending_user_signups WHERE email = ? AND tenant_id = ?`, e, tCtx.ID).Scan(&exists)
	if exists > 0 {
		http.Error(w, "Already registered — check email", 400)
		return
	}

	// Step 6: Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(p), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("[REGISTER] Password hashing error", "err", err)
		http.Error(w, "Internal error", 500)
		return
	}

	// Step 7: Generate token and insert pending signup
	token, _ := utils.GenerateUserToken(e, tCtx.ID, time.Now().Add(24*time.Hour))
	db.DB.Exec(`
        INSERT INTO pending_user_signups (email, tenant_id, password_hash, token, expires_at)
        VALUES (?, ?, ?, ?, ?)`, e, tCtx.ID, string(hash), token, time.Now().Add(24*time.Hour),
	)
	link := fmt.Sprintf("http://%s.%s/confirm?token=%s", tCtx.Subdomain, cfg.Domain, token)
	slog.Info("[REGISTER] Sent confirm link", "email", e, "link", link)
	w.Write([]byte("Check your email for a confirmation link."))
}

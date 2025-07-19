package handlers

import (
	"database/sql"
	"fmt"
	"html/template"
	"log/slog"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

var tmpl *template.Template

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var subdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

func InitEnrollTemplates(base []string) {
	tmpl = template.Must(template.ParseFiles(append(base, "templates/enroll.html")...))
}

func EnrollHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Handle GET request to serve the enroll form
	if r.Method == http.MethodGet {
		// Extract CSRF token from context (set by middleware)
		csrfToken, ok := r.Context().Value(middleware.CsrfKey).(string)
		if !ok {
			slog.Error("[ENROLL] CSRF token not found in context")
			http.Error(w, "Internal error", 500)
			return
		}
		data := struct {
			CSRFToken string
		}{
			CSRFToken: csrfToken,
		}
		tmpl.Execute(w, data) // Render template with CSRF data
		return
	}

	// Step 2: Parse the form data for POST requests
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Invalid form", 400)
		return
	}

	email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
	org := strings.TrimSpace(r.FormValue("org_name"))
	password := r.FormValue("password")

	// Step 3: Validate required fields
	if email == "" || org == "" || password == "" {
		http.Error(w, "All fields are required", 400)
		return
	}

	// Step 4: Validate email format
	if !emailRegex.MatchString(email) {
		http.Error(w, "Invalid email format", 400)
		return
	}

	sub := strings.ToLower(strings.ReplaceAll(org, " ", ""))
	// Step 5: Validate subdomain
	if !subdomainRegex.MatchString(sub) {
		http.Error(w, "Organization name is invalid", 400)
		return
	}

	// Step 6: Check for duplicate email or subdomain in DB
	var exists int
	err := db.DB.QueryRow(`SELECT 1 FROM tenants WHERE email = ? OR subdomain = ?`, email, sub).Scan(&exists)
	if err == sql.ErrNoRows {
		// No duplicate, proceed
	} else if err != nil {
		slog.Error("[ENROLL] DB lookup error", "err", err, "email", email, "sub", sub)
		http.Error(w, "Internal error", 500)
		return
	} else {
		slog.Info("[ENROLL] Attempt to reuse email or subdomain", "org", org, "email", email)
		http.Error(w, "Organization or email already in use", http.StatusConflict)
		return
	}

	// Step 7: Hash password with bcrypt
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		slog.Error("[ENROLL] Password hashing error", "err", err)
		http.Error(w, "Internal error", 500)
		return
	}
	passHash := string(hash)

	expires := time.Now().Add(24 * time.Hour)
	token, err := utils.GenerateSignupToken(email, org, expires)
	if err != nil {
		slog.Error("[ENROLL] Token generation error", "err", err)
		http.Error(w, "Token error", 500)
		return
	}

	// Step 8: Insert pending signup into DB
	_, err = db.DB.Exec(`
		INSERT INTO pending_tenant_signups (email, org_name, password_hash, token, expires_at)
		VALUES (?, ?, ?, ?, ?)`,
		email, org, passHash, token, expires)
	if err != nil {
		slog.Error("[ENROLL] DB insert error", "err", err, "email", email)
		http.Error(w, "DB error", 500)
		return
	}

	// Step 9: Generate verification link and log
	link := fmt.Sprintf("http://%s/verify?token=%s", cfg.Domain, token)
	slog.Info("[ENROLL] Token created", "email", email, "link", link)

	w.Write([]byte("Please check your email (or console during dev) to verify your account."))
}

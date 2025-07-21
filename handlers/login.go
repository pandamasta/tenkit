package handlers

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

var loginTmpl *template.Template // Loaded at startup

// Step 0: Load templates
func InitLoginTemplates(base []string) {
	loginTmpl = template.Must(template.ParseFiles(
		append(base, "templates/login.html")...,
	))
}

// Step 1–7: Handle GET and POST /login
func LoginHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Handle GET request to serve the login form
	if r.Method == http.MethodGet {
		// Extract CSRF token from context (set by middleware)
		csrfToken, ok := r.Context().Value(middleware.CsrfKey).(string)
		if !ok {
			slog.Error("[LOGIN] CSRF token not found in context")
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}

		// Step 2: Prepare data for template
		data := utils.BaseTemplateData(r, map[string]interface{}{
			"CSRFToken": csrfToken,
		})

		// Step 3: Render login form
		utils.RenderTemplate(w, loginTmpl, "base", data)
		return
	}

	// Step 4: Parse form data from POST request
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=InvalidForm", http.StatusSeeOther)
		return
	}

	// Step 5: Extract submitted values
	email := r.FormValue("email")
	pass := r.FormValue("password")

	// Step 6: Validate required fields
	if email == "" || pass == "" {
		http.Redirect(w, r, "/login?error=MissingFields", http.StatusSeeOther)
		return
	}

	// Step 7: Retrieve tenant from context
	t := middleware.FromContext(r.Context())
	if t == nil {
		slog.Error("[LOGIN] Tenant context missing", "email", email)
		http.Error(w, "Tenant not found", http.StatusBadRequest)
		return
	}

	// Step 8: Look up user by email and tenant
	user, err := models.GetUserByEmailAndTenant(email, t.ID)
	if err != nil {
		slog.Error("[LOGIN] DB error", "email", email, "tenant", t.Subdomain, "err", err)
		http.Redirect(w, r, "/login?error=Internal", http.StatusSeeOther)
		return
	}
	if user == nil {
		slog.Info("[LOGIN] No user found", "email", email, "tenant", t.Subdomain)
		http.Redirect(w, r, "/login?error=InvalidCreds", http.StatusSeeOther)
		return
	}

	// Step 9: Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)); err != nil {
		slog.Info("[LOGIN] Wrong password", "email", email, "tenant", t.Subdomain)
		http.Redirect(w, r, "/login?error=InvalidCreds", http.StatusSeeOther)
		return
	}

	// Step 10: Create session token
	token := models.CreateSession(user.ID, user.TenantID)

	// Step 11: Set session cookie
	cookie := http.Cookie{
		Name:     cfg.SessionCookie.Name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production
		Expires:  time.Now().Add(cfg.TokenExpiry),
	}
	http.SetCookie(w, &cookie)

	// Step 12: Log success and redirect
	slog.Info("[LOGIN] User logged in", "email", email, "tenant", t.Subdomain)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GET /logout
func LogoutHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Clear session cookie
	cookie := http.Cookie{
		Name:     cfg.SessionCookie.Name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	}
	http.SetCookie(w, &cookie)

	// Step 2: Redirect to home
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

package handlers

import (
	"html/template"
	"log/slog"
	"net/http"
	"time"

	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"

	"golang.org/x/crypto/bcrypt"
)

var loginTmpl *template.Template // Define the template variable

func InitLoginTemplates(base []string) {
	loginTmpl = template.Must(template.ParseFiles(append(base, "templates/login.html")...))
}

// POST /login
func LoginHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Handle GET request to serve the login form
	if r.Method == http.MethodGet {
		// Extract CSRF token from context (set by middleware)
		csrfToken, ok := r.Context().Value(middleware.CsrfKey).(string) // Use uppercase CsrfKey
		if !ok {
			slog.Error("[LOGIN] CSRF token not found in context")
			http.Error(w, "Internal error", http.StatusInternalServerError)
			return
		}
		data := struct {
			CSRFToken string
		}{
			CSRFToken: csrfToken,
		}
		loginTmpl.Execute(w, data) // Render the template with data
		return
	}

	// Step 2: Parse the form data for POST requests
	if err := r.ParseForm(); err != nil {
		http.Redirect(w, r, "/login?error=InvalidForm", http.StatusSeeOther)
		return
	}

	email := r.FormValue("email")
	pass := r.FormValue("password")

	// Step 3: Validate required fields
	if email == "" || pass == "" {
		http.Redirect(w, r, "/login?error=MissingFields", http.StatusSeeOther)
		return
	}

	// Step 4: Retrieve tenant from context
	t := middleware.FromContext(r.Context())
	if t == nil {
		slog.Error("[LOGIN] Tenant context missing for login attempt", "email", email)
		http.Error(w, "Tenant not found", http.StatusBadRequest)
		return
	}

	// Step 5: Fetch user from DB
	user, err := models.GetUserByEmailAndTenant(email, t.ID)
	if err != nil {
		slog.Error("[LOGIN] DB error during lookup", "email", email, "tenant_subdomain", t.Subdomain, "err", err)
		http.Redirect(w, r, "/login?error=Internal", http.StatusSeeOther)
		return
	}
	if user == nil {
		slog.Info("[LOGIN] No such user", "email", email, "tenant_subdomain", t.Subdomain)
		http.Redirect(w, r, "/login?error=InvalidCreds", http.StatusSeeOther)
		return
	}

	// Step 6: Verify password with bcrypt
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)); err != nil {
		slog.Info("[LOGIN] Wrong password", "email", email, "tenant_subdomain", t.Subdomain)
		http.Redirect(w, r, "/login?error=InvalidCreds", http.StatusSeeOther)
		return
	}

	// Step 7: Create session and set cookie
	token := models.CreateSession(user.ID, user.TenantID)
	cookie := http.Cookie{
		Name:     cfg.SessionCookie.Name,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // TODO: set to true in production with HTTPS
		Expires:  time.Now().Add(cfg.TokenExpiry),
	}
	http.SetCookie(w, &cookie)

	slog.Info("[LOGIN] User logged in", "email", email, "tenant_subdomain", t.Subdomain)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// GET /logout
func LogoutHandler(cfg *multitenant.Config, w http.ResponseWriter, r *http.Request) {
	// Step 1: Clear the session cookie
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

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
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"

	"golang.org/x/crypto/bcrypt"
)

var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)
var subdomainRegex = regexp.MustCompile(`^[a-z0-9]([a-z0-9\-]{0,61}[a-z0-9])?$`)

// InitEnrollTemplates parses the templates needed for the enroll page.
// It includes header, base layout, and enroll-specific content.
func InitEnrollTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/enroll.html")...)
	if err != nil {
		slog.Error("[ENROLL] Failed to parse enroll template", "err", err)
		panic(err)
	}
	return tmpl
}

// EnrollHandler handles GET requests to serve the enroll form and POST requests to process it.
func EnrollHandler(cfg *multitenant.Config, i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		lang := middleware.LangFromContext(r.Context())

		// Step 1: Handle GET request to serve the enroll form
		if r.Method == http.MethodGet {
			slog.Debug("[ENROLL] GET request received")
			data := render.BaseTemplateData(r, i18n, nil)
			slog.Debug("[ENROLL] Rendering template with base layout using RenderTemplate")
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 2: Parse the form data for POST requests
		if err := r.ParseForm(); err != nil {
			slog.Error("[ENROLL] Invalid form", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.invalid_form", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		email := strings.ToLower(strings.TrimSpace(r.FormValue("email")))
		org := strings.TrimSpace(r.FormValue("org_name"))
		password := r.FormValue("password")

		// Step 3: Validate required fields
		if email == "" || org == "" || password == "" {
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.required_fields", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 4: Validate email format
		if !emailRegex.MatchString(email) {
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.invalid_email", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		sub := strings.ToLower(strings.ReplaceAll(org, " ", ""))
		// Step 5: Validate subdomain
		if !subdomainRegex.MatchString(sub) {
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.invalid_org_name", lang),
			})
			w.WriteHeader(http.StatusBadRequest)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 6: Check for duplicate email or subdomain in DB
		var exists int
		err := db.DB.QueryRow(`SELECT 1 FROM tenants WHERE email = ? OR subdomain = ?`, email, sub).Scan(&exists)
		if err == sql.ErrNoRows {
			// No duplicate, proceed
		} else if err != nil {
			slog.Error("[ENROLL] DB lookup error", "err", err, "email", email, "sub", sub)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		} else {
			slog.Info("[ENROLL] Attempt to reuse email or subdomain", "org", org, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.email_or_subdomain_exists", lang),
			})
			w.WriteHeader(http.StatusConflict)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 7: Hash password with bcrypt
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			slog.Error("[ENROLL] Password hashing error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}
		passHash := string(hash)

		expires := time.Now().Add(24 * time.Hour)
		// Step 8: Generate signup token
		token, err := utils.GenerateSignupToken(email, org, expires)
		if err != nil {
			slog.Error("[ENROLL] Token generation error", "err", err)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 9: Insert pending signup into DB
		_, err = db.DB.Exec(`
			INSERT INTO pending_tenant_signups (email, org_name, password_hash, token, expires_at)
			VALUES (?, ?, ?, ?, ?)`,
			email, org, passHash, token, expires)
		if err != nil {
			slog.Error("[ENROLL] DB insert error", "err", err, "email", email)
			data := render.BaseTemplateData(r, i18n, map[string]any{
				"Error": i18n.T("enroll.internal_error", lang),
			})
			w.WriteHeader(http.StatusInternalServerError)
			render.RenderTemplate(w, tmpl, "base", data)
			return
		}

		// Step 10: Generate verification link and log
		link := fmt.Sprintf("http://%s/verify?token=%s", cfg.Domain, token)
		slog.Info("[ENROLL] Token created", "email", email, "link", link)

		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Success": i18n.T("enroll.success", lang),
		})
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

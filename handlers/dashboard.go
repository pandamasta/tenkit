package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

// InitDashboardTemplates parses the template needed for the dashboard page.
func InitDashboardTemplates(base []string) *template.Template {
	tmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	tmpl, err = tmpl.ParseFiles(append(base, "templates/dashboard.html")...)
	if err != nil {
		slog.Error("[DASHBOARD] Failed to parse dashboard template", "err", err)
		panic(err)
	}
	return tmpl
}

// DashboardHandler handles GET requests to display the user/admin dashboard.
func DashboardHandler(i18n *i18n.I18n, tmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Restrict to tenant domains, return 404 if on marketing domain
		if !middleware.IsTenantRequest(r.Context()) {
			slog.Warn("[DASHBOARD] Dashboard attempted on marketing domain", "host", r.Host)
			http.NotFound(w, r)
			return
		}

		// Step 2: Check authenticated user
		user := middleware.CurrentUser(r)
		if user == nil {
			slog.Warn("[DASHBOARD] Unauthenticated access attempt")
			http.Redirect(w, r, "/login?error=auth", http.StatusSeeOther)
			return
		}

		// Step 3: Get tenant information
		tenant := middleware.FromContext(r.Context())
		if tenant == nil {
			slog.Error("[DASHBOARD] Tenant context missing")
			http.NotFound(w, r)
			return
		}

		// Step 4: Fetch role from memberships
		var role string
		err := db.LogQueryRow(r.Context(), db.DB,
			`SELECT role FROM memberships WHERE user_id = ? AND tenant_id = ?`,
			user.ID, tenant.ID).Scan(&role)
		if err != nil {
			slog.Warn("[DASHBOARD] Failed to fetch role, defaulting to member", "err", err)
			role = "member"
		}

		// Step 5: Prepare template data
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Role": role,
		})

		// Step 6: Render dashboard
		slog.Debug("[DASHBOARD] Rendering dashboard", "user_id", user.ID, "tenant_id", tenant.ID)
		render.RenderTemplate(w, tmpl, "base", data)
	}
}

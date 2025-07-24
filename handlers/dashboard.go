package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

// InitDashboardTemplates parses the templates needed for the dashboard page.
// Returns main and tenant templates for rendering.
func InitDashboardTemplates(base []string) (*template.Template, *template.Template) {
	mainTmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	var err error
	mainTmpl, err = mainTmpl.ParseFiles(append(base, "templates/main.html")...)
	if err != nil {
		slog.Error("[DASHBOARD] Failed to parse main template", "err", err)
		panic(err)
	}

	tenantTmpl := template.New("base").Funcs(template.FuncMap{
		"t": func(key string, args ...any) string {
			return key // Placeholder
		},
	})
	tenantTmpl, err = tenantTmpl.ParseFiles(append(base, "templates/tenant.html")...)
	if err != nil {
		slog.Error("[DASHBOARD] Failed to parse tenant template", "err", err)
		panic(err)
	}

	return mainTmpl, tenantTmpl
}

// DashboardHandler handles GET requests to display the user/admin dashboard.
func DashboardHandler(cfg *multitenant.Config, i18n *i18n.I18n, mainTmpl, tenantTmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Check authenticated user
		user := middleware.CurrentUser(r)
		if user == nil {
			slog.Warn("[DASHBOARD] Unauthenticated access attempt")
			http.Redirect(w, r, "/login?error=auth", http.StatusSeeOther)
			return
		}

		// Step 2: Get tenant information
		tenant := middleware.FromContext(r.Context())
		tenantName := "Main Site"
		tenantID := int64(0)
		if tenant != nil {
			tenantName = tenant.Name
			tenantID = tenant.ID
		}

		// Step 3: Fetch role from memberships
		var role string
		row := db.LogQueryRow(r.Context(), db.DB,
			`SELECT role FROM memberships WHERE user_id = ? AND tenant_id = ?`,
			user.ID, tenantID)
		if err := row.Scan(&role); err != nil {
			slog.Warn("[DASHBOARD] Failed to fetch role, defaulting to member", "err", err)
			role = "member"
		}

		// Step 4: Prepare template data
		data := render.BaseTemplateData(r, i18n, map[string]any{
			"Email":      user.Email,
			"Role":       role,
			"TenantID":   tenantID,
			"TenantName": tenantName,
		})

		// Step 5: Render dashboard
		slog.Debug("[DASHBOARD] Rendering dashboard", "user_id", user.ID, "tenant_id", tenantID)
		if tenant != nil {
			slog.Debug("[DASHBOARD] Rendering tenant template", "template", "tenant.html")
			render.RenderTemplate(w, tenantTmpl, "base", data)
		} else {
			slog.Debug("[DASHBOARD] Rendering main template", "template", "main.html")
			render.RenderTemplate(w, mainTmpl, "base", data)
		}
	}
}

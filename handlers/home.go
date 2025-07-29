package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

// HomeHandler handles "/" and dispatches to marketing or tenant home.
// TODO: Split into MarketingHomeHandler and TenantHomeHandler in separate files.
func HomeHandler(i18n *i18n.I18n, baseTmpl *template.Template) http.HandlerFunc {
	// Clone and parse specific templates
	mainTmpl, err := baseTmpl.Clone() // Handle error
	if err != nil {
		slog.Error("[HOME] Failed to clone base for main template", "err", err)
		panic(err)
	}
	mainTmpl, err = mainTmpl.ParseFiles("templates/main.html")
	if err != nil {
		slog.Error("[HOME] Failed to parse main template", "err", err)
		panic(err) // Or handle gracefully
	}

	tenantTmpl, err := baseTmpl.Clone() // Handle error
	if err != nil {
		slog.Error("[HOME] Failed to clone base for tenant template", "err", err)
		panic(err)
	}
	tenantTmpl, err = tenantTmpl.ParseFiles("templates/tenant.html")
	if err != nil {
		slog.Error("[HOME] Failed to parse tenant template", "err", err)
		panic(err) // Or handle gracefully
	}

	return func(w http.ResponseWriter, r *http.Request) {
		if middleware.FromContext(r.Context()) != nil {
			tenantHome(w, r, i18n, tenantTmpl) // Dispatch to tenant
		} else {
			marketingHome(w, r, i18n, mainTmpl) // Dispatch to marketing
		}
	}
}

// marketingHome renders the public landing page (extracted for future split).
func marketingHome(w http.ResponseWriter, r *http.Request, i18n *i18n.I18n, tmpl *template.Template) {
	data := render.BaseTemplateData(r, i18n, nil)
	slog.Debug("[HOME] Rendering marketing home", "lang", data.Lang)
	render.RenderTemplate(w, tmpl, "base", data)
}

// tenantHome renders the tenant-specific home (extracted for future split).
func tenantHome(w http.ResponseWriter, r *http.Request, i18n *i18n.I18n, tmpl *template.Template) {
	data := render.BaseTemplateData(r, i18n, nil)
	slog.Debug("[HOME] Rendering tenant home", "lang", data.Lang, "tenant", data.Tenant.Subdomain)
	render.RenderTemplate(w, tmpl, "base", data)
}

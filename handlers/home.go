package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/internal/render"
)

// InitHomeTemplates parses the templates for the landing page and tenant home page.
// It includes header, base layout, and specific content for each.
func InitHomeTemplates(base []string) (*template.Template, *template.Template) {
	mainTmpl := template.New("base")
	var err error
	mainTmpl, err = mainTmpl.ParseFiles(append(base, "templates/main.html")...)
	if err != nil {
		slog.Error("[HOME] Failed to parse main template", "err", err)
		panic(err)
	}

	tenantTmpl := template.New("base")
	tenantTmpl, err = tenantTmpl.ParseFiles(append(base, "templates/tenant.html")...)
	if err != nil {
		slog.Error("[HOME] Failed to parse tenant template", "err", err)
		panic(err)
	}

	return mainTmpl, tenantTmpl
}

// HomeHandler handles the "/" route.
// Renders the marketing landing page (if no tenant) or tenant home page (if tenant).
func HomeHandler(i18n *i18n.I18n, mainTmpl, tenantTmpl *template.Template) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		data := render.BaseTemplateData(r, i18n, nil)
		slog.Debug("[HOME] Rendering home page", "lang", data.Lang, "tenant", data.Tenant != nil, "user", data.User != nil)

		if data.Tenant != nil {
			slog.Debug("[HOME] Rendering tenant template", "template", "tenant.html")
			render.RenderTemplate(w, tenantTmpl, "base", data)
		} else {
			slog.Debug("[HOME] Rendering main template", "template", "main.html")
			render.RenderTemplate(w, mainTmpl, "base", data)
		}
	}
}

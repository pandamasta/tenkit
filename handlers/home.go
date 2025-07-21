package handlers

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/multitenant/middleware"
	"github.com/pandamasta/tenkit/multitenant/utils"
)

var (
	mainPageTmpl   *template.Template
	tenantPageTmpl *template.Template
)

// InitHomeTemplates parses the templates needed for the home page.
// Each template includes header, base layout, and its specific content.
func InitHomeTemplates() {
	mainPageTmpl = template.Must(template.ParseFiles(
		"templates/header.html",
		"templates/base.html",
		"templates/main.html",
	))

	tenantPageTmpl = template.Must(template.ParseFiles(
		"templates/header.html",
		"templates/base.html",
		"templates/tenant.html",
	))
}

// HomeHandler handles GET requests to "/".
// It detects whether a tenant is present and renders the appropriate view.
func HomeHandler(w http.ResponseWriter, r *http.Request) {
	t := middleware.FromContext(r.Context())
	u := middleware.CurrentUser(r)
	lang := middleware.LangFromContext(r.Context())

	uid := int64(0)
	if u != nil {
		uid = u.ID
	}
	slog.Info("[MAIN-homeHandler] GET / - tenant", "tenant", t, "userID", uid, "lang", lang)

	extra := map[string]interface{}{
		"UserID": uid,
		"User":   u,
	}
	if t != nil {
		extra["Tenant"] = t
	}

	data := utils.BaseTemplateData(r, extra)

	if t != nil {
		utils.RenderTemplate(w, tenantPageTmpl, "base", data)
	} else {
		utils.RenderTemplate(w, mainPageTmpl, "base", data)
	}
}

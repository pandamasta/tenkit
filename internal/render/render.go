package render

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

type TemplateData struct {
	Tenant    *multitenant.Tenant
	User      *models.User
	Lang      string
	CSRFToken string
	T         func(key string, args ...any) string
	Extra     map[string]any
}

func BaseTemplateData(r *http.Request, i18n *i18n.I18n, extra map[string]any) TemplateData {
	ctx := r.Context()
	tenant := middleware.FromContext(ctx)
	user := middleware.CurrentUser(r)
	lang := middleware.LangFromContext(ctx)
	csrf, _ := ctx.Value(middleware.CsrfKey).(string)

	slog.Debug("[RENDER] BaseTemplateData", "lang", lang, "tenant", tenant != nil, "user", user != nil, "csrf", csrf != "")

	return TemplateData{
		Tenant:    tenant,
		User:      user,
		Lang:      lang,
		CSRFToken: csrf,
		T: func(key string, args ...any) string {
			slog.Debug("[RENDER] Translation called", "key", key, "lang", lang, "args", args)
			result := i18n.T(key, lang, args...)
			slog.Debug("[RENDER] Translation result", "key", key, "lang", lang, "result", result)
			return result
		},
		Extra: extra,
	}
}

func RenderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data TemplateData) {
	slog.Debug("[RENDER] Rendering template", "name", name, "lang", data.Lang)
	if err := tmpl.ExecuteTemplate(w, name, data); err != nil {
		slog.Error("[RENDER] Template execution failed", "err", err)
		// Vérifier si l'en-tête a déjà été écrit
		if w.Header().Get("Content-Type") == "" {
			w.Header().Set("Content-Type", "text/plain; charset=utf-8")
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = w.Write([]byte("Internal server error"))
		}
	}
}

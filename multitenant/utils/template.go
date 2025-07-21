package utils

import (
	"html/template"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

// BaseTemplateData returns the base data structure injected into templates,
// including translation helper, selected language, and any additional context (like CSRFToken).
func BaseTemplateData(r *http.Request, extra map[string]interface{}) map[string]interface{} {
	lang := middleware.LangFromContext(r.Context())

	data := map[string]interface{}{
		"Lang": lang,
		"T": func(key string) string {
			return i18n.T(key, lang)
		},
	}

	// Inject extra values (like CSRFToken, PageTemplate, etc.)
	for k, v := range extra {
		data[k] = v
	}

	return data
}

func RenderTemplate(w http.ResponseWriter, tmpl *template.Template, name string, data map[string]interface{}) {
	err := tmpl.ExecuteTemplate(w, name, data)
	if err != nil {
		slog.Error("[TEMPLATE] Failed to render", "template", name, "error", err)
		http.Error(w, "Template rendering error", http.StatusInternalServerError)
		return
	}
	slog.Info("[TEMPLATE] Rendered", "template", name)
}

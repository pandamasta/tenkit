package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pandamasta/tenkit/multitenant"
)

// LangMiddleware extracts the "lang" cookie and injects it into the context.
// If no cookie is found, it falls back to the Accept-Language header or default language.
func LangMiddleware(cfg *multitenant.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := cfg.I18n.DefaultLang

		if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
			lang = cookie.Value
			slog.Info("[LANG] Language from cookie", "lang", lang)
		} else if accept := r.Header.Get("Accept-Language"); accept != "" {
			// Parse only the first language from the header (e.g., "fr-FR,fr;q=0.9")
			lang = strings.Split(accept, ",")[0]
			lang = strings.Split(lang, "-")[0] // Convert "fr-FR" to "fr"
			slog.Info("[LANG] Language from Accept-Language header", "lang", lang)
		} else {
			slog.Info("[LANG] No 'lang' cookie or header found, using default", "lang", lang)
		}

		ctx := context.WithValue(r.Context(), langKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LangFromContext retrieves the current language from context.
func LangFromContext(ctx context.Context) string {
	lang, ok := ctx.Value(langKey).(string)
	if !ok {
		return "en"
	}
	return lang
}

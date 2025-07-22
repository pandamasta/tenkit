package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"strings"

	"github.com/pandamasta/tenkit/multitenant"
)

type LangKeyType string

const LangKey LangKeyType = "lang"

// LangMiddleware extracts the "lang" cookie and injects it into the context.
// If no cookie is found, it falls back to the Accept-Language header or default language.
func LangMiddleware(cfg *multitenant.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := cfg.I18n.DefaultLang

		if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
			lang = cookie.Value
			slog.Info("[LANG] Language from cookie", "lang", lang)
		} else if accept := r.Header.Get("Accept-Language"); accept != "" {
			lang = strings.Split(accept, ",")[0]
			lang = strings.Split(lang, "-")[0]
			slog.Info("[LANG] Language from Accept-Language header", "lang", lang)
		} else {
			slog.Info("[LANG] No 'lang' cookie or header found, using default", "lang", lang)
		}

		slog.Debug("[LANG] Language resolved", "lang", lang)
		ctx := context.WithValue(r.Context(), LangKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LangFromContext retrieves the current language from context.
func LangFromContext(ctx context.Context) string {
	lang, ok := ctx.Value(LangKey).(string)
	if !ok {
		return "en"
	}
	return lang
}

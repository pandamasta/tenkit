package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/multitenant"
)

// LangMiddleware extracts the "lang" cookie and injects it into the context.
func LangMiddleware(cfg *multitenant.Config, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := cfg.I18n.DefaultLang

		if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
			lang = cookie.Value
			slog.Info("[LANG] Language from cookie", "lang", lang)
		} else {
			slog.Info("[LANG] No 'lang' cookie found, using default", "lang", lang)
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

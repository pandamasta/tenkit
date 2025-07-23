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

// I18nProvider defines an interface for accessing translations.
type I18nProvider interface {
	Translations() map[string]map[string]string
}

// LangMiddleware extracts the language from the cookie or Accept-Language header and injects it into the context.
func LangMiddleware(cfg *multitenant.Config, i18n I18nProvider, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lang := cfg.I18n.DefaultLang // Read DEFAULT_LANG from .env via Config
		translations := i18n.Translations()

		// 1. Check the "lang" cookie
		if cookie, err := r.Cookie("lang"); err == nil && cookie.Value != "" {
			if _, ok := translations[cookie.Value]; ok {
				lang = cookie.Value
				slog.Info("[LANG] Language from cookie", "lang", lang)
			}
		} else if accept := r.Header.Get("Accept-Language"); accept != "" {
			// 2. Check the Accept-Language header
			langs := strings.Split(accept, ",")
			for _, l := range langs {
				l = strings.Split(l, ";")[0] // Ignore weights (e.g., q=0.9)
				l = strings.TrimSpace(l)
				if _, ok := translations[l]; ok {
					lang = l
					slog.Info("[LANG] Language from Accept-Language header", "lang", lang)
					break
				}
				// Try the base language (e.g., fr for fr-FR)
				base := strings.Split(l, "-")[0]
				if base != l {
					if _, ok := translations[base]; ok {
						lang = base
						slog.Info("[LANG] Language from Accept-Language base", "lang", lang)
						break
					}
				}
			}
		} else {
			slog.Info("[LANG] No 'lang' cookie or header found, using default", "lang", lang)
		}

		slog.Debug("[LANG] Language resolved", "lang", lang)
		ctx := context.WithValue(r.Context(), LangKey, lang)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// LangFromContext retrieves the current language from the context.
func LangFromContext(ctx context.Context) string {
	lang, ok := ctx.Value(LangKey).(string)
	if !ok {
		slog.Warn("[LANG] No language in context, falling back to default", "lang", "en")
		return "en"
	}
	return lang
}

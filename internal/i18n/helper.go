package i18n

import "github.com/pandamasta/tenkit/multitenant"

// Translate returns the translated value with fallback logic.
func Translate(cfg multitenant.I18nConfig, key, lang string) string {
	if lang == "" {
		lang = cfg.DefaultLang
	}
	return T(key, lang)
}

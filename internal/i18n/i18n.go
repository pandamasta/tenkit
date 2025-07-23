package i18n

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

// I18n manages JSON translations with a robust and thread-safe mechanism.
type I18n struct {
	translations map[string]map[string]string
	defaultLang  string
	debug        bool
	mu           sync.RWMutex
}

// New creates a new I18n instance with the default language.
func New(defaultLang string) (*I18n, error) {
	// Validate the default language code (e.g., en, fr-FR)
	if !isValidLang(defaultLang) {
		return nil, fmt.Errorf("invalid default language: %s (must be like 'en' or 'en-US')", defaultLang)
	}
	return &I18n{
		translations: make(map[string]map[string]string),
		defaultLang:  defaultLang,
	}, nil
}

// isValidLang validates language codes (e.g., en, fr-FR).
func isValidLang(lang string) bool {
	return regexp.MustCompile(`^[a-z]{2}(-[A-Z]{2})?$`).MatchString(lang)
}

// EnableDebug enables debug mode for detailed logging.
func (i *I18n) EnableDebug() {
	i.debug = true
}

// Translations returns a copy of the translations for validation in middlewares.
func (i *I18n) Translations() map[string]map[string]string {
	i.mu.RLock()
	defer i.mu.RUnlock()
	return i.translations
}

// LoadLocales loads JSON translation files from a directory.
func (i *I18n) LoadLocales(dir string) error {
	i.mu.Lock()
	defer i.mu.Unlock()

	// Reset translations to avoid stale data
	i.translations = make(map[string]map[string]string)

	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		slog.Error("[LANG] Failed to list translation files", "dir", dir, "error", err)
		return fmt.Errorf("failed to list translation files: %w", err)
	}
	if len(files) == 0 {
		slog.Warn("[LANG] No translation files found", "dir", dir)
		return fmt.Errorf("no translation files found in %s", dir)
	}

	for _, file := range files {
		lang := strings.TrimSuffix(filepath.Base(file), ".json")
		if !isValidLang(lang) {
			slog.Warn("[LANG] Invalid language code, skipping", "lang", lang, "file", file)
			continue
		}

		slog.Info("[LANG] Loading translation file", "file", file, "lang", lang)
		data, err := os.ReadFile(file)
		if err != nil {
			slog.Error("[LANG] Failed to read translation file", "file", file, "error", err)
			return fmt.Errorf("failed to read translation file %s: %w", file, err)
		}

		var entries map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			slog.Error("[LANG] Invalid JSON format", "file", file, "error", err)
			return fmt.Errorf("invalid JSON format in %s: %w", file, err)
		}
		if len(entries) == 0 {
			slog.Warn("[LANG] Translation file is empty", "file", file)
			continue
		}

		i.translations[lang] = entries
		slog.Info("[LANG] Successfully loaded", "lang", lang, "entries", len(entries))
		if i.debug {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			slog.Debug("[LANG] Loaded keys", "lang", lang, "keys", keys)
		}
	}

	// Validate that the default language has translations
	if _, ok := i.translations[i.defaultLang]; !ok {
		slog.Error("[LANG] Default language has no translations", "lang", i.defaultLang)
		return fmt.Errorf("default language %s has no translations", i.defaultLang)
	}

	if i.debug {
		slog.Debug("[LANG] All translations loaded", "langs", len(i.translations))
	}
	return nil
}

// ReloadLocales reloads JSON translation files without restarting the server.
func (i *I18n) ReloadLocales(dir string) error {
	slog.Info("[LANG] Reloading locales", "dir", dir)
	return i.LoadLocales(dir)
}

// T translates a key into the requested language, with support for arguments.
func (i *I18n) T(key, lang string, args ...any) string {
	i.mu.RLock()
	defer i.mu.RUnlock()

	if i.debug {
		keys := make([]string, 0, len(i.translations[lang]))
		for k := range i.translations[lang] {
			keys = append(keys, k)
		}
		slog.Debug("[LANG] Looking up key", "key", key, "lang", lang, "available_keys", keys)
	}

	val := i.getTranslation(key, lang)
	if val == "" {
		slog.Warn("[LANG] Missing translation", "key", key, "lang", lang)
		val = key // Fallback to the key
	}

	if len(args) > 0 {
		return fmt.Sprintf(val, args...)
	}
	return val
}

// getTranslation retrieves a translation with fallback to base language and default language.
func (i *I18n) getTranslation(key, lang string) string {
	if v, ok := i.translations[lang][key]; ok {
		return v
	}
	if base := strings.Split(lang, "-")[0]; base != lang {
		if v, ok := i.translations[base][key]; ok {
			return v
		}
	}
	if v, ok := i.translations[i.defaultLang][key]; ok {
		return v
	}
	return ""
}

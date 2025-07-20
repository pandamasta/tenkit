package i18n

import (
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	translations      = make(map[string]map[string]string) // lang -> key -> value
	defaultLang       = "en"
	debugTranslations = false
	mu                sync.RWMutex
)

// SetDefaultLang sets the fallback language if no match found
func SetDefaultLang(lang string) {
	mu.Lock()
	defer mu.Unlock()
	defaultLang = lang
}

// EnableDebug enables debug logging for missing translations
func EnableDebug() {
	debugTranslations = true
}

// LoadLocales loads all .json files from the provided directory
func LoadLocales(dir string) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.json"))
	if err != nil {
		return err
	}

	if len(files) == 0 {
		slog.Warn("[LANG] No translation files found", "dir", dir)
	}

	for _, file := range files {
		lang := strings.TrimSuffix(filepath.Base(file), ".json")
		slog.Info("[LANG] Loading translation file", "file", file, "lang", lang)

		data, err := os.ReadFile(file)
		if err != nil {
			slog.Error("[LANG] Failed to read translation file", "file", file, "error", err)
			return err
		}

		var entries map[string]string
		if err := json.Unmarshal(data, &entries); err != nil {
			slog.Error("[LANG] Invalid JSON format", "file", file, "error", err)
			return err
		}

		if len(entries) == 0 {
			slog.Warn("[LANG] Translation file is empty", "file", file)
		}

		mu.Lock()
		translations[lang] = entries
		mu.Unlock()

		slog.Info("[LANG] Successfully loaded", "lang", lang, "entries", len(entries))
	}
	return nil
}

// T translates a key into the requested language, fallback to defaultLang
func T(key, lang string) string {
	mu.RLock()

	defer mu.RUnlock()

	if debugTranslations {
		slog.Debug("[LANG] Looking up key", "key", key, "lang", lang)
	}

	if val, ok := translations[lang][key]; ok {
		return val
	}

	if val, ok := translations[defaultLang][key]; ok {
		if debugTranslations {
			slog.Warn("[LANG] Missing translation in requested lang", "key", key, "lang", lang, "fallback", defaultLang)
		}
		return val
	}

	if debugTranslations {
		slog.Error("[LANG] Missing translation in all langs", "key", key, "lang", lang)
	}
	return key
}

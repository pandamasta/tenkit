package i18n

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type I18n struct {
	translations map[string]map[string]string
	defaultLang  string
	debug        bool
	mu           sync.RWMutex
}

func New(defaultLang string) *I18n {
	return &I18n{
		translations: make(map[string]map[string]string),
		defaultLang:  defaultLang,
	}
}

func (i *I18n) EnableDebug() {
	i.debug = true
}

func (i *I18n) LoadLocales(dir string) error {
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
		i.mu.Lock()
		i.translations[lang] = entries
		i.mu.Unlock()
		slog.Info("[LANG] Successfully loaded", "lang", lang, "entries", len(entries))
		if i.debug {
			keys := make([]string, 0, len(entries))
			for k := range entries {
				keys = append(keys, k)
			}
			slog.Debug("[LANG] Loaded keys", "lang", lang, "keys", keys)
		}
	}
	if i.debug {
		slog.Debug("[LANG] All translations loaded", "translations", i.translations)
	}
	return nil
}

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
		if i.debug {
			slog.Warn("[LANG] Missing translation in all langs", "key", key, "lang", lang)
		}
		val = key // Fallback to key
	}

	if len(args) > 0 {
		return fmt.Sprintf(val, args...)
	}
	return val
}

func (i *I18n) getTranslation(key, lang string) string {
	if val, ok := i.translations[lang][key]; ok {
		return val
	}
	baseLang := strings.Split(lang, "-")[0]
	if baseLang != lang {
		if val, ok := i.translations[baseLang][key]; ok {
			return val
		}
	}
	if val, ok := i.translations[i.defaultLang][key]; ok {
		return val
	}
	return ""
}

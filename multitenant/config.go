package multitenant

import (
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/pandamasta/tenkit/internal/envloader"
)

// Config defines the global configuration structure for a multitenant application.
type Config struct {
	Domain        string        // Root domain (e.g., "example.com")
	SessionCookie CookieConfig  // Session cookie configuration
	CSRF          CSRFConfig    // CSRF protection configuration
	Server        ServerConfig  // HTTP server configuration
	TokenExpiry   time.Duration // Default token/session expiration
	I18n          I18nConfig    // Language and translation config
}

// I18nConfig holds configuration for i18n and translations.
type I18nConfig struct {
	DefaultLang string // e.g. "en", "fr"
	LocalesPath string // Path to folder with JSON translation files
}

// CookieConfig holds session cookie settings.
type CookieConfig struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
	MaxAge   time.Duration
	Domain   string // Added to specify cookie domain
}

// CSRFConfig holds CSRF token configuration for cookie and headers.
type CSRFConfig struct {
	CookieName string
	HeaderName string
	Secure     bool
	SameSite   http.SameSite
	MaxAge     time.Duration
}

// ServerConfig holds the network address configuration.
type ServerConfig struct {
	Addr string // Example: ":8080"
}

// LoadDefaultConfig returns an AppConfig populated with environment variables or default values.
func LoadDefaultConfig() *Config {
	envloader.LoadDotEnv(".env") // log déjà géré

	domain := getEnv("APP_DOMAIN", "localhost:9003")
	isSecure := domain != "localhost" && domain != "localhost:9003"

	defaultLang := getEnv("DEFAULT_LANG", "en")
	localesPath := getEnv("TENKIT_LOCALES", "internal/i18n/locales") // permet override en prod/dev

	return &Config{
		Domain: domain,
		SessionCookie: CookieConfig{
			Name:     getEnv("SESSION_COOKIE", "app_session"),
			Secure:   getEnvBool("SESSION_COOKIE_SECURE", isSecure),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7 * 24 * time.Hour,
			Domain:   "", // Empty for localhost; set to ".xxx.xx" in production
		},
		CSRF: CSRFConfig{
			CookieName: "csrf_token",
			HeaderName: "X-CSRF-Token",
			Secure:     getEnvBool("CSRF_COOKIE_SECURE", isSecure),
			SameSite:   http.SameSiteLaxMode,
			MaxAge:     2 * time.Hour,
		},
		Server: ServerConfig{
			Addr: getEnv("SERVER_ADDR", ":9003"),
		},
		TokenExpiry: 24 * time.Hour,
		I18n: I18nConfig{
			DefaultLang: defaultLang,
			LocalesPath: localesPath,
		},
	}
}

// getEnv returns the environment variable or a fallback default.
func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvBool returns a boolean environment variable or a fallback.
func getEnvBool(key string, fallback bool) bool {
	if v := os.Getenv(key); v != "" {
		b, err := strconv.ParseBool(v)
		if err == nil {
			return b
		}
	}
	return fallback
}

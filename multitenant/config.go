package multitenant

import (
	"net/http"
	"os"
	"strconv"
	"time"
)

// AppConfig defines the global configuration structure for a multitenant application.
type Config struct {
	Domain        string        // Root domain, e.g., "example.com"
	SessionCookie CookieConfig  // Session cookie configuration
	CSRF          CSRFConfig    // CSRF protection configuration
	Server        ServerConfig  // HTTP server configuration
	TokenExpiry   time.Duration // Default token/session expiration
}

// CookieConfig holds session cookie settings.
type CookieConfig struct {
	Name     string
	Secure   bool
	SameSite http.SameSite
	MaxAge   time.Duration
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
	domain := getEnv("APP_DOMAIN", "localhost:9003")
	isSecure := domain != "localhost" && domain != "localhost:9003"

	return &Config{
		Domain: domain,
		SessionCookie: CookieConfig{
			Name:     getEnv("SESSION_COOKIE", "app_session"),
			Secure:   getEnvBool("SESSION_COOKIE_SECURE", isSecure),
			SameSite: http.SameSiteLaxMode,
			MaxAge:   7 * 24 * time.Hour,
		},
		CSRF: CSRFConfig{
			CookieName: "csrf_token",
			HeaderName: "X-CSRF-Token",
			Secure:     getEnvBool("CSRF_COOKIE_SECURE", isSecure),
			SameSite:   http.SameSiteStrictMode,
			MaxAge:     2 * time.Hour,
		},
		Server: ServerConfig{
			Addr: getEnv("SERVER_ADDR", ":9003"),
		},
		TokenExpiry: 24 * time.Hour,
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

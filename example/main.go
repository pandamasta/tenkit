package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/handlers"
	"github.com/pandamasta/tenkit/internal/i18n"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

func main() {
	cfg := multitenant.LoadDefaultConfig()

	// Initialize i18n
	i18nInstance, err := i18n.New(cfg.I18n.DefaultLang)
	if err != nil {
		slog.Error("[LANG] Failed to initialize i18n", "error", err)
		os.Exit(1)
	}
	slog.Info("[LANG] Loading locales", "path", cfg.I18n.LocalesPath)
	if err := i18nInstance.LoadLocales(cfg.I18n.LocalesPath); err != nil {
		slog.Error("[LANG] Error loading translations", "err", err)
		os.Exit(1)
	}

	// Debug setup
	if os.Getenv("TENKIT_DEBUG") == "1" {
		db.EnableDebugLogs()
		i18nInstance.EnableDebug()
		slog.Info("Debug logging ENABLED")
	}

	// Logging setup
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// Load DB
	db.Init()

	// Centralized base template loading (only shared files)
	baseTemplates := []string{"templates/base.html", "templates/header.html"}
	baseTmpl := template.New("base")
	baseTmpl, err = baseTmpl.ParseFiles(baseTemplates...)
	if err != nil {
		slog.Error("[TEMPLATES] Failed to parse base templates", "err", err)
		os.Exit(1)
	}

	// Routes (pass baseTmpl to all handlers)
	mux := setupRoutes(cfg, i18nInstance, baseTmpl)

	// Tenant resolver/fetcher
	resolver := multitenant.SubdomainResolver{Config: cfg}
	fetcher := multitenant.DBFetcher{DB: db.DB}

	// Middleware chain
	handler := middleware.LangMiddleware(cfg, i18nInstance, mux)
	handler = middleware.TenantMiddleware(cfg, resolver, fetcher, handler)
	handler = middleware.SessionMiddleware(cfg, handler)
	handler = middleware.CSRFMiddleware(handler)
	handler = Recover(handler)
	handler = middleware.Logger(cfg, handler)

	slog.Info("Starting HTTP server", "addr", cfg.Server.Addr)
	slog.Debug("Loaded config", "config", cfg)

	if err := http.ListenAndServe(cfg.Server.Addr, handler); err != nil {
		slog.Error("Server exited with error", "error", err)
		os.Exit(1)
	}
}

// setupRoutes extracts route registration for clarity.
func setupRoutes(cfg *multitenant.Config, i18n *i18n.I18n, baseTmpl *template.Template) *http.ServeMux {
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", handlers.HomeHandler(i18n, baseTmpl)) // Pass baseTmpl
	mux.HandleFunc("/lang", langSwitcherHandler)

	mux.Handle("/enroll", middleware.RateLimit(handlers.EnrollHandler(cfg, i18n, baseTmpl)))
	mux.Handle("/verify", handlers.VerifyHandler(cfg, i18n, baseTmpl))
	mux.Handle("/register", middleware.RateLimit(handlers.RegisterHandler(cfg, i18n, baseTmpl)))
	mux.Handle("/confirm", handlers.ConfirmHandler(cfg, i18n, baseTmpl))
	mux.Handle("/login", middleware.RateLimit(handlers.LoginHandler(cfg, i18n, baseTmpl)))
	mux.Handle("/logout", handlers.LogoutHandler(cfg, i18n))
	mux.Handle("/reset", middleware.RateLimit(handlers.RequestResetPasswordHandler(cfg, i18n, baseTmpl)))
	mux.Handle("/reset/confirm", middleware.RateLimit(handlers.ResetPasswordHandler(cfg, i18n, baseTmpl)))
	mux.Handle("/dashboard", middleware.RequireAuth(handlers.DashboardHandler(i18n, baseTmpl)))

	return mux
}

// langSwitcherHandler extracts the /lang func for readability.
func langSwitcherHandler(w http.ResponseWriter, r *http.Request) {
	lang := r.URL.Query().Get("lang")
	if lang != "" {
		http.SetCookie(w, &http.Cookie{Name: "lang", Value: lang, Path: "/"})
	}
	http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
}

// Recover is a middleware for panic recovery.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("[RECOVER] Panic recovered", "err", err, "url", r.URL.Path)
				if w.Header().Get("Content-Type") == "" {
					w.Header().Set("Content-Type", "text/plain")
				}
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

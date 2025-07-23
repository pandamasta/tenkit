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
	"github.com/pandamasta/tenkit/internal/render"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

var (
	mainPageTmpl   *template.Template
	tenantPageTmpl *template.Template
)

func main() {
	cfg := multitenant.LoadDefaultConfig()

	// Initialiser i18n avec validation
	i18n, err := i18n.New(cfg.I18n.DefaultLang)
	if err != nil {
		slog.Error("[LANG] Failed to initialize i18n", "error", err)
		os.Exit(1)
	}
	slog.Info("[LANG] Loading locales", "path", cfg.I18n.LocalesPath)
	if err := i18n.LoadLocales(cfg.I18n.LocalesPath); err != nil {
		slog.Error("[LANG] Error loading translations", "err", err)
		os.Exit(1)
	}

	if os.Getenv("TENKIT_DEBUG") == "1" {
		db.EnableDebugLogs()
		i18n.EnableDebug()
		slog.Info("Debug logging ENABLED")
	}

	// Log config
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if os.Getenv("TENKIT_DEBUG") == "1" {
		db.EnableDebugLogs()
		i18n.EnableDebug()
		slog.Info("Debug logging ENABLED")
	}

	// Load DB
	db.Init()

	// Load templates
	baseTemplates := []string{
		"templates/base.html",
		"templates/header.html",
	}
	mainPageTmpl, tenantPageTmpl = handlers.InitHomeTemplates(baseTemplates)
	enrollTmpl := handlers.InitEnrollTemplates(baseTemplates)
	verifyTmpl := handlers.InitVerifyTemplates(baseTemplates)
	registerTmpl := handlers.InitRegisterTemplates(baseTemplates)
	confirmTmpl := handlers.InitConfirmTemplates(baseTemplates)
	loginTmpl := handlers.InitLoginTemplates(baseTemplates)

	// Routes
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", handlers.HomeHandler(i18n, mainPageTmpl, tenantPageTmpl))

	// Set language via dropdown (persists in cookie)
	mux.HandleFunc("/lang", func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang != "" {
			http.SetCookie(w, &http.Cookie{
				Name:  "lang",
				Value: lang,
				Path:  "/",
			})
		}
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	})

	mux.HandleFunc("/enroll", handlers.EnrollHandler(cfg, i18n, enrollTmpl))
	mux.HandleFunc("/verify", handlers.VerifyHandler(cfg, i18n, verifyTmpl))
	mux.HandleFunc("/register", handlers.RegisterHandler(cfg, i18n, registerTmpl))
	mux.HandleFunc("/confirm", handlers.ConfirmHandler(cfg, i18n, confirmTmpl))
	mux.HandleFunc("/login", handlers.LoginHandler(cfg, i18n, loginTmpl))
	mux.HandleFunc("/logout", handlers.LogoutHandler(cfg, i18n))

	dashboardHandler := func(w http.ResponseWriter, r *http.Request) {
		// Step 1: Prepare template data
		data := render.BaseTemplateData(r, i18n, nil)
		slog.Debug("[DASHBOARD] Rendering dashboard", "lang", data.Lang, "tenant", data.Tenant != nil, "user", data.User != nil)

		// Step 2: Render template
		if data.Tenant != nil {
			slog.Debug("[DASHBOARD] Rendering tenant template", "template", "tenant.html")
			render.RenderTemplate(w, tenantPageTmpl, "base", data)
		} else {
			slog.Debug("[DASHBOARD] Rendering main template", "template", "main.html")
			render.RenderTemplate(w, mainPageTmpl, "base", data)
		}
	}
	mux.Handle("/dashboard", middleware.RequireAuth(http.HandlerFunc(dashboardHandler)))

	resolver := multitenant.SubdomainResolver{Config: cfg}
	fetcher := multitenant.DBFetcher{DB: db.DB}

	// Middleware
	handler := middleware.LangMiddleware(cfg, i18n, mux)
	handler = middleware.TenantMiddleware(cfg, resolver, fetcher, handler)
	handler = middleware.SessionMiddleware(cfg, handler)
	handler = middleware.CSRFMiddleware(handler)
	handler = middleware.Logger(cfg, handler)

	slog.Info("Starting HTTP server", "addr", cfg.Server.Addr)
	slog.Debug("Loaded config", "config", cfg)

	if err := http.ListenAndServe(cfg.Server.Addr, handler); err != nil {
		slog.Error("Server exited with error", "error", err)
	}
}

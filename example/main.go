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
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

var (
	mainPageTmpl   *template.Template
	tenantPageTmpl *template.Template
)

type PageData struct {
	Tenant *multitenant.Tenant
	UserID int64
	User   *models.User
	Lang   string
	T      func(string) string
}

func main() {
	cfg := multitenant.LoadDefaultConfig()

	i18n.SetDefaultLang(cfg.I18n.DefaultLang)
	slog.Info("[LANG] Loading locales", "path", cfg.I18n.LocalesPath)
	if err := i18n.LoadLocales(cfg.I18n.LocalesPath); err != nil {
		slog.Error("Error loading translations", "err", err)
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
	mainPageTmpl = template.Must(template.ParseFiles(append(baseTemplates, "templates/main.html")...))
	tenantPageTmpl = template.Must(template.ParseFiles(append(baseTemplates, "templates/tenant.html")...))

	handlers.InitHomeTemplates()
	handlers.InitEnrollTemplates(baseTemplates)
	handlers.InitRegisterTemplates(baseTemplates)
	handlers.InitLoginTemplates(baseTemplates)
	handlers.InitVerifyTemplates(baseTemplates)
	handlers.InitConfirmTemplates(baseTemplates)

	// Routes
	mux := http.NewServeMux()

	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	mux.HandleFunc("/", handlers.HomeHandler)

	// Set language via dropdown (persists in cookie)
	mux.HandleFunc("/lang", func(w http.ResponseWriter, r *http.Request) {
		lang := r.URL.Query().Get("lang")
		if lang != "" {
			http.SetCookie(w, &http.Cookie{
				Name:  "lang",
				Value: lang,
				Path:  "/",
				// Optionnel : MaxAge, SameSite, etc.
			})
		}
		http.Redirect(w, r, r.Referer(), http.StatusSeeOther)
	})

	mux.HandleFunc("/enroll", func(w http.ResponseWriter, r *http.Request) {
		handlers.EnrollHandler(cfg, w, r)
	})
	mux.HandleFunc("/verify", func(w http.ResponseWriter, r *http.Request) {
		handlers.VerifyHandler(cfg, w, r)
	})
	mux.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		handlers.RegisterHandler(cfg, w, r)
	})
	mux.HandleFunc("/confirm", func(w http.ResponseWriter, r *http.Request) {
		handlers.ConfirmHandler(cfg, w, r)
	})
	mux.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handlers.LoginHandler(cfg, w, r)
	})
	mux.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		handlers.LogoutHandler(cfg, w, r)
	})

	mux.Handle("/dashboard", middleware.RequireAuth(http.HandlerFunc(dashboardHandler)))

	resolver := multitenant.SubdomainResolver{Config: cfg}
	fetcher := multitenant.DBFetcher{DB: db.DB}

	// Middleware
	handler := middleware.LangMiddleware(cfg, mux)
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

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	t := middleware.FromContext(r.Context())
	u := middleware.CurrentUser(r)
	lang := middleware.LangFromContext(r.Context())

	uid := int64(0)
	if u != nil {
		uid = u.ID
	}

	data := PageData{
		Tenant: t,
		UserID: uid,
		User:   u,
		Lang:   lang,
		T: func(key string) string {
			return i18n.T(key, lang)
		},
	}

	if t != nil {
		tenantPageTmpl.Execute(w, data)
	} else {
		mainPageTmpl.Execute(w, data)
	}
}

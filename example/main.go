package main

import (
	"html/template"
	"log/slog"
	"net/http"
	"os"

	_ "github.com/mattn/go-sqlite3"

	"github.com/pandamasta/tenkit/db"
	"github.com/pandamasta/tenkit/handlers"
	"github.com/pandamasta/tenkit/internal/envloader"
	"github.com/pandamasta/tenkit/models"
	"github.com/pandamasta/tenkit/multitenant"
	"github.com/pandamasta/tenkit/multitenant/middleware"
)

var (
	mainPageTmpl   *template.Template
	tenantPageTmpl *template.Template
)

type PageData struct {
	Tenant *multitenant.Tenant // Changed to multitenant.Tenant
	UserID int64
	User   *models.User
}

func main() {

	// Load env

	envloader.LoadDotEnv(".env")

	// Set up slog with text handler (string format)
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	// load config

	cfg := multitenant.LoadDefaultConfig()

	// Envvar

	if os.Getenv("TENKIT_DEBUG") == "1" {
		db.EnableDebugLogs()
		slog.Info(" Debug logging ENABLED")
	}

	// 1 Init DB & Constants
	db.Init()

	// 2 Load templates
	base := []string{
		"templates/base.html",
		"templates/header.html",
	}
	mainPageTmpl = template.Must(template.ParseFiles(append(base, "templates/main.html")...))
	tenantPageTmpl = template.Must(template.ParseFiles(append(base, "templates/tenant.html")...))
	handlers.InitEnrollTemplates(base)
	handlers.InitRegisterTemplates(base)
	handlers.InitLoginTemplates(base)

	// 3 Setup Routes
	mux := http.NewServeMux()

	// Static assets
	fileServer := http.FileServer(http.Dir("static"))
	mux.Handle("/static/", http.StripPrefix("/static/", fileServer))

	// Public + tenant routes
	mux.HandleFunc("/", homeHandler)
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

	// 4 Wrap with Middleware: Tenant + Logger
	handler := middleware.TenantMiddleware(cfg, resolver, fetcher, mux)
	handler = middleware.SessionMiddleware(cfg, handler)
	handler = middleware.CSRFMiddleware(handler)
	handler = middleware.Logger(cfg, handler)

	// 5 Start Server
	slog.Info("Starting HTTP server", "addr", cfg.Server.Addr)
	slog.Debug("Loaded config", "config", cfg)

	if err := http.ListenAndServe(cfg.Server.Addr, handler); err != nil {
		slog.Error("Server exited with error", "error", err)
	}

}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	t := middleware.FromContext(r.Context())
	u := middleware.CurrentUser(r)
	uid := int64(0)
	if u != nil {
		uid = u.ID
	}
	slog.Info("[MAIN-homeHandler] GET / - tenant", "tenant", t, "userID", uid)

	data := PageData{
		Tenant: t,
		UserID: uid,
		User:   u,
	}

	if t != nil {
		tenantPageTmpl.Execute(w, data)
	} else {
		mainPageTmpl.Execute(w, data)
	}
}

func dashboardHandler(w http.ResponseWriter, r *http.Request) {
	t := middleware.FromContext(r.Context())
	user := middleware.CurrentUser(r)

	data := map[string]interface{}{
		"Tenant": t,
		"User":   user,
	}

	if t != nil {
		tenantPageTmpl.Execute(w, data)
	} else {
		mainPageTmpl.Execute(w, data)
	}
}

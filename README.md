# tenkit

**tenkit** is a minimal multitenant middleware toolkit for Go web applications with server-side rendering (SSR).  
It provides reusable logic to bootstrap multi-tenant SaaS platforms in Go, using only the standard library.

## Features

- Multitenant support with subdomain resolution
- Internationalization (i18n) with JSON-based translations
- Server-side rendered templates with CSRF protection
- Secure session management and authentication
- Environment variable loading via `.env`
- SQLite database support (PostgreSQL planned)
- Zero external dependencies (stdlib only)

## Middleware

- **Subdomain-based tenant resolution** (`multitenant/middleware/tenant.go`): Resolves tenants from request subdomains and injects tenant context.
- **Session management** (`multitenant/middleware/session.go`): Secure cookie-based session store with user tracking.
- **CSRF protection** (`multitenant/middleware/csrf.go`): Token-based CSRF prevention for forms and headers.
- **Authentication guard** (`multitenant/middleware/auth.go`): Restricts routes to authenticated users.
- **Language handling** (`multitenant/middleware/lang.go`): Sets language from cookie or `Accept-Language` header.
- **HTTP request logging** (`multitenant/middleware/http_logger.go`): Logs requests using `slog`.

## Current Limitations

- Email sending not implemented
- SQLite only (PostgreSQL support planned)
- Server-side rendering only (API and client-side rendering planned)

## Directory Structure

```

tenkit/
├── go.mod                   # Go module definition
├── main.go                  # Main application entry
├── internal/
│   ├── i18n/               # Internationalization (JSON translations)
│   ├── render/             # Template rendering utilities
│   └── envloader/          # .env file loader
├── handlers/               # HTTP handlers (home, enroll, login, etc.)
├── templates/              # HTML templates (base.html, main.html, etc.)
├── multitenant/
│   ├── middleware/         # Middleware components (tenant, session, etc.)
│   ├── utils/              # Token generation utilities
│   ├── config.go           # Configuration
│   └── interfaces.go       # Resolver and fetcher interfaces
├── models/                 # Data models (tenant, user)
└── db/                     # SQLite database integration
└── example/                # Example application
```


## Example Usage

See the working example in [`example/`](./example/):

```
package main

import (
    "net/http"
    "os"

    "github.com/pandamasta/tenkit/db"
    "github.com/pandamasta/tenkit/handlers"
    "github.com/pandamasta/tenkit/internal/envloader"
    "github.com/pandamasta/tenkit/internal/i18n"
    "github.com/pandamasta/tenkit/multitenant"
    "github.com/pandamasta/tenkit/multitenant/middleware"
    "log/slog"
)

func main() {
    envloader.LoadDotEnv(".env")
    cfg := multitenant.LoadDefaultConfig()

    i18n := i18n.New(cfg.I18n.DefaultLang)
    if err := i18n.LoadLocales(cfg.I18n.LocalesPath); err != nil {
        slog.Error("Error loading translations", "err", err)
    }

    db.Init()

    baseTemplates := []string{"templates/base.html", "templates/header.html"}
    mainTmpl, tenantTmpl := handlers.InitHomeTemplates(baseTemplates)

    mux := http.NewServeMux()
    mux.HandleFunc("/", handlers.HomeHandler(i18n, mainTmpl, tenantTmpl))

    resolver := multitenant.SubdomainResolver{Config: cfg}
    fetcher := multitenant.DBFetcher{DB: db.DB}

    handler := middleware.LangMiddleware(cfg, mux)
    handler = middleware.TenantMiddleware(cfg, resolver, fetcher, handler)
    handler = middleware.SessionMiddleware(cfg, handler)
    handler = middleware.CSRFMiddleware(handler)
    handler = middleware.Logger(cfg, handler)

    slog.Info("Starting HTTP server", "addr", cfg.Server.Addr)
    if err := http.ListenAndServe(cfg.Server.Addr, handler); err != nil {
        slog.Error("Server exited with error", "error", err)
    }
}
```

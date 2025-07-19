# tenkit

**tenkit** is a minimal multitenant middleware toolkit for Go web applications (server-side rendered).  
It provides reusable logic to help you bootstrap multi-tenant SaaS platforms in Go — without external dependencies.

## Features

* Environment variable support via `.env` loader (`internal/envloader`)
* Built-in SQLite support (PostgreSQL planned)
* Server-side rendered templates (SSR)
* Uses only the Go **standard library** (by design, for now)
* Simple structure, minimal dependencies
* Pluggable and composable architecture

###  Middleware

The following middleware components are available and can be composed freely:

* **Subdomain-based tenant resolution**
  Resolves tenants based on request subdomain and injects tenant context.

* **Session management with secure cookies**
  Cookie-based session store, with secure encoding and user tracking.

* **CSRF protection**
  Token-based CSRF prevention via headers and HTML forms.

* **Authentication guard**
  Protect routes and restrict access to authenticated users only.


## Current limitations

- Email sending is not implemented yet
- Only SQLite is supported (PostgreSQL planned)
- Only server-side rendering (SSR) — API & client-side rendering (CSR) will come in future versions

## Directory structure

- `internal/envloader/` – loads variables from `.env` files
- `multitenant/config.go` – default config (domain, session, etc.)
- `multitenant/interfaces.go` – tenant resolver & fetcher interfaces
- `multitenant/errors.go` – internal error definitions
- `multitenant/middleware/`
  - `tenant.go` – tenant resolution via subdomain
  - `session.go` – session handling (cookies)
  - `csrf.go` – CSRF protection
  - `auth.go` – authentication guard
  - `http_logger.go` – HTTP request logging (using `slog`)


## Example usage

A working example app is available in [`example/`](./example/):

```
envloader.LoadDotEnv(".env")
cfg := multitenant.LoadDefaultConfig()

handler := middleware.TenantMiddleware(cfg, resolver, fetcher, mux)
handler = middleware.SessionMiddleware(cfg, handler)
handler = middleware.CSRFMiddleware(handler)
handler = middleware.Logger(cfg, handler)

http.ListenAndServe(cfg.Server.Addr, handler)
```

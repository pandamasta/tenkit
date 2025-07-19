package middleware

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/pandamasta/tenkit/multitenant"
)

func TenantMiddleware(cfg *multitenant.Config, resolver multitenant.TenantResolver, fetcher multitenant.TenantFetcher, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		subdomain, err := resolver.Resolve(r)
		if err != nil {
			slog.Error("[MIDDLEWARE] Resolution error", "err", err)
			http.NotFound(w, r)
			return
		}
		ctx := r.Context()

		if subdomain == "" {
			slog.Info("[MIDDLEWARE] Default domain accessed", "host", r.Host)
			ctx = context.WithValue(ctx, isTenantCtxKey, false)
			r = r.WithContext(ctx) // Ensure updated ctx is attached
			next.ServeHTTP(w, r)
			return
		}

		slog.Info("[MIDDLEWARE] Looking up tenant for subdomain", "subdomain", subdomain, "host", r.Host)

		t, err := fetcher.Fetch(ctx, subdomain)
		if err != nil {
			slog.Error("[TENANT] Fetch error", "subdomain", subdomain, "err", err)
			http.NotFound(w, r)
			return
		}
		if t == nil {
			slog.Error("[TENANT] Unknown or inactive tenant", "subdomain", subdomain)
			http.NotFound(w, r)
			return
		}

		slog.Info("[TENANT] Loaded tenant", "name", t.Name, "subdomain", t.Subdomain)
		ctx = context.WithValue(ctx, TenantKey, t)
		ctx = context.WithValue(ctx, isTenantCtxKey, true)
		r = r.WithContext(ctx) // Ensure updated ctx is attached
		next.ServeHTTP(w, r)
	})
}

func FromContext(ctx context.Context) *multitenant.Tenant {
	if t, ok := ctx.Value(TenantKey).(*multitenant.Tenant); ok {
		return t
	}
	return nil
}

func IsTenantRequest(ctx context.Context) bool {
	v := ctx.Value(isTenantCtxKey)
	ok, isBool := v.(bool)
	return ok && isBool
}

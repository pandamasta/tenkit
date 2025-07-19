package multitenant

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"strings"

	"github.com/pandamasta/tenkit/models"
)

// Tenant is the shared struct for tenant data.
type Tenant struct {
	ID        int64
	Subdomain string
	Name      string
}

// TenantResolver extracts the tenant identifier from the request.
type TenantResolver interface {
	Resolve(r *http.Request) (string, error) // Returns subdomain or empty for main site
}

// SubdomainResolver is the default implementation using host.
type SubdomainResolver struct {
	Config *Config
}

func (s SubdomainResolver) Resolve(r *http.Request) (string, error) {
	host := r.Host
	if i := strings.Index(host, ":"); i != -1 {
		host = host[:i] // Remove port
	}
	// Strict domain check
	if !strings.HasSuffix(host, "."+s.Config.Domain) && host != s.Config.Domain && host != "www."+s.Config.Domain {
		return "", fmt.Errorf("invalid domain: %s", host)
	}
	if host == s.Config.Domain || host == "www."+s.Config.Domain {
		return "", nil
	}
	sub := strings.TrimSuffix(host, "."+s.Config.Domain)
	return strings.ToLower(strings.TrimSuffix(sub, ".")), nil
}

// TenantFetcher loads the tenant from the identifier.
type TenantFetcher interface {
	Fetch(ctx context.Context, identifier string) (*Tenant, error)
}

// DBFetcher is the default DB-based implementation.
type DBFetcher struct {
	DB *sql.DB // Or *gorm.DB if using ORM later
}

func (f DBFetcher) Fetch(ctx context.Context, sub string) (*Tenant, error) {
	// Your existing logic from tenantLoader/models
	t, err := models.GetTenantBySubdomain(ctx, f.DB, sub)
	if err != nil || t == nil {
		return nil, err
	}
	return &Tenant{ID: int64(t.ID), Subdomain: t.Subdomain, Name: t.Name}, nil
}

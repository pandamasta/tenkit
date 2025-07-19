package models

import (
	"context"
	"database/sql"
	"log"
	"time"

	"github.com/pandamasta/tenkit/db"
)

type Tenant struct {
	ID           int
	Name         string
	Slug         string
	Subdomain    string
	CustomDomain sql.NullString
	Email        string
	PrimaryColor sql.NullString
	LogoPath     sql.NullString
	IsActive     bool
	IsDeleted    bool
	AllowSignins bool
	CreatedAt    time.Time
	UpdatedAt    time.Time
	DeletedAt    sql.NullTime
	Timezone     string
	Address      sql.NullString
	Country      sql.NullString
}

func GetTenantBySubdomain(ctx context.Context, conn *sql.DB, subdomain string) (*Tenant, error) {
	log.Printf("[DB] 🔍 Querying tenant: %q", subdomain)

	row := db.LogQueryRow(ctx, conn, `
		SELECT id, name, slug, subdomain, custom_domain, email, primary_color,
		       logo_path, is_active, is_deleted, allow_signins,
		       created_at, updated_at, deleted_at, timezone, address, country
		FROM tenants
		WHERE subdomain = ? AND is_active = 1 AND is_deleted = 0
	`, subdomain)

	var t Tenant
	err := row.Scan(&t.ID, &t.Name, &t.Slug, &t.Subdomain, &t.CustomDomain,
		&t.Email, &t.PrimaryColor, &t.LogoPath, &t.IsActive, &t.IsDeleted,
		&t.AllowSignins, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
		&t.Timezone, &t.Address, &t.Country)

	if err == sql.ErrNoRows {
		log.Printf("[DB] ❌ No tenant matched: %q", subdomain)
		return nil, nil
	}
	if err != nil {
		log.Printf("[DB] ❌ Query failed: %v", err)
	}
	return &t, err
}

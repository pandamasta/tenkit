package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

// Init initializes the database and creates the schema.
func Init() {
	var err error
	DB, err = sql.Open("sqlite3", "./clubapp.db")
	if err != nil {
		log.Fatalf("DB connection error: %v", err)
	}

	schema := `
	CREATE TABLE IF NOT EXISTS tenants (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL UNIQUE,
		slug TEXT NOT NULL UNIQUE,
		subdomain TEXT NOT NULL UNIQUE,
		custom_domain TEXT,
		email TEXT NOT NULL,
		primary_color TEXT,
		logo_path TEXT,
		is_active BOOLEAN NOT NULL DEFAULT 1,
		is_deleted BOOLEAN NOT NULL DEFAULT 0,
		allow_signins BOOLEAN NOT NULL DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		deleted_at DATETIME,
		timezone TEXT DEFAULT 'UTC',
		address TEXT,
		country TEXT
	);

	CREATE TABLE IF NOT EXISTS pending_tenant_signups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		org_name TEXT NOT NULL,
		password_hash TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL
	);

	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL,
		is_verified BOOLEAN NOT NULL DEFAULT 0,
		tenant_id INTEGER,
		role TEXT DEFAULT 'member',
		FOREIGN KEY (tenant_id) REFERENCES tenants(id)
	);

	CREATE TABLE IF NOT EXISTS memberships (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		tenant_id INTEGER NOT NULL,
		role TEXT DEFAULT 'member',
		is_active BOOLEAN NOT NULL DEFAULT 1,
		joined_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (user_id) REFERENCES users(id),
		FOREIGN KEY (tenant_id) REFERENCES tenants(id),
		UNIQUE(user_id, tenant_id)
	);

	CREATE TABLE IF NOT EXISTS pending_user_signups (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		email TEXT NOT NULL,
		tenant_id INTEGER NOT NULL,
		password_hash TEXT NOT NULL,
		token TEXT NOT NULL UNIQUE,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY (tenant_id) REFERENCES tenants(id)
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		tenant_id INTEGER NOT NULL,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id),
		FOREIGN KEY(tenant_id) REFERENCES tenants(id)
	);

	CREATE TABLE IF NOT EXISTS password_resets (
		user_id INTEGER NOT NULL,
		tenant_id INTEGER NOT NULL,
		token TEXT PRIMARY KEY,
		expires_at DATETIME NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id),
		FOREIGN KEY(tenant_id) REFERENCES tenants(id)
	);
	`

	if _, err := DB.Exec(schema); err != nil {
		log.Fatalf("Schema error: %v", err)
	}
}

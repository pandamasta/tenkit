package models

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"log"
	"time"

	"github.com/pandamasta/tenkit/db"
)

type User struct {
	ID           int64
	Email        string
	PasswordHash string
	TenantID     int64
}

func GetUserByEmail(email string) (*User, error) {
	row := db.LogQueryRow(context.Background(), db.DB,
		`SELECT id, email, password_hash, tenant_id FROM users WHERE email = ? AND is_verified = 1`, email)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.TenantID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func GetUserByEmailAndTenant(email string, tenantID int64) (*User, error) {
	row := db.LogQueryRow(context.Background(), db.DB,
		`SELECT id, email, password_hash, tenant_id FROM users 
		 WHERE email = ? AND tenant_id = ? AND is_verified = 1`,
		email, tenantID)
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.TenantID); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func CreateSession(userID, tenantID int64) string {
	b := make([]byte, 16)
	rand.Read(b)
	token := hex.EncodeToString(b)

	_, err := db.DB.Exec(`INSERT INTO sessions (token, user_id, tenant_id, expires_at)
        VALUES (?, ?, ?, ?)`, token, userID, tenantID, time.Now().Add(24*time.Hour))
	if err != nil {
		log.Printf("[SESSION] Error creating session: %v", err)
	}
	return token
}

func GetSession(token string) (*User, error) {
	row := db.LogQueryRow(context.Background(), db.DB,
		`SELECT u.id, u.email, u.password_hash, u.tenant_id
         FROM sessions s
         JOIN users u ON u.id = s.user_id
         WHERE s.token = ? AND s.expires_at > ?`,
		token, time.Now())
	var u User
	if err := row.Scan(&u.ID, &u.Email, &u.PasswordHash, &u.TenantID); err != nil {
		return nil, err
	}
	return &u, nil
}

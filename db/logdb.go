package db

import (
	"context"
	"database/sql"
	"log"
)

// Debug controls whether to print DB logs
var Debug = false

func EnableDebugLogs() {
	Debug = true
}

func DisableDebugLogs() {
	Debug = false
}

func LogExec(ctx context.Context, db *sql.DB, query string, args ...any) (sql.Result, error) {
	if Debug {
		log.Printf("[SQL Exec] %s -- %v", query, args)
	}
	return db.ExecContext(ctx, query, args...)
}

func LogQuery(ctx context.Context, db *sql.DB, query string, args ...any) (*sql.Rows, error) {
	if Debug {
		log.Printf("[SQL Query] %s -- %v", query, args)
	}
	return db.QueryContext(ctx, query, args...)
}

func LogQueryRow(ctx context.Context, db *sql.DB, query string, args ...any) *sql.Row {
	if Debug {
		log.Printf("[SQL QueryRow] \n%s\n         -- %v", query, args)
	}
	return db.QueryRowContext(ctx, query, args...)
}

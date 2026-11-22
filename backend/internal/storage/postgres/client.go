package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

// Config controls the database connection pool.
// The pool is bounded to prevent request bursts from exhausting PostgreSQL.
type Config struct {
	URL                string
	MaxOpenConnections int
	MaxIdleConnections int
	ConnectionLifetime time.Duration
	IdleConnectionTime time.Duration
}

// Open creates a PostgreSQL pool and verifies connectivity before returning it.
func Open(ctx context.Context, cfg Config) (*sql.DB, error) {
	db, err := sql.Open("pgx", cfg.URL)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL connection pool: %w", err)
	}

	db.SetMaxOpenConns(cfg.MaxOpenConnections)
	db.SetMaxIdleConns(cfg.MaxIdleConnections)
	db.SetConnMaxLifetime(cfg.ConnectionLifetime)
	db.SetConnMaxIdleTime(cfg.IdleConnectionTime)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("verify PostgreSQL connectivity: %w", err)
	}

	return db, nil
}

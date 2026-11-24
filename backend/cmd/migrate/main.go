package main

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"

	"monitor-me.internal/backend/internal/config"
	"monitor-me.internal/backend/migrations"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("database migration failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	source, err := iofs.New(migrations.Files, ".")
	if err != nil {
		return fmt.Errorf("create embedded migration source: %w", err)
	}

	databaseURL, err := migrationDatabaseURL(cfg.DatabaseURL)
	if err != nil {
		_ = source.Close()
		return err
	}

	migrator, err := migrate.NewWithSourceInstance("iofs", source, databaseURL)
	if err != nil {
		_ = source.Close()
		return fmt.Errorf("create migration runner: %w", err)
	}
	defer func() {
		sourceErr, databaseErr := migrator.Close()
		if sourceErr != nil || databaseErr != nil {
			logger.Warn("migration runner close returned an error", "source_error", sourceErr, "database_error", databaseErr)
		}
	}()

	// A clean database is expected on first startup. ErrNoChange means that
	// every migration has already been applied and is not a failure.
	if err := migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("apply database migrations: %w", err)
	}

	version, dirty, err := migrator.Version()
	if err != nil {
		return fmt.Errorf("read migration version: %w", err)
	}
	if dirty {
		return fmt.Errorf("database migration version %d is marked dirty", version)
	}

	logger.Info("database migrations are up to date", "version", version)
	return nil
}

func migrationDatabaseURL(raw string) (string, error) {
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parse migration database URL: %w", err)
	}

	if parsed.Scheme != "postgres" && parsed.Scheme != "postgresql" {
		return "", fmt.Errorf("migration database URL must use the postgres scheme")
	}

	parsed.Scheme = "pgx5"
	query := parsed.Query()
	query.Set("x-multi-statement", "true")
	query.Set("x-statement-timeout", "30000")
	parsed.RawQuery = query.Encode()

	return parsed.String(), nil
}

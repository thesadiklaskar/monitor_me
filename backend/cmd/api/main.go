package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	redisclient "github.com/WorkRCS/monitor_me/backend/internal/cache/redis"
	"github.com/WorkRCS/monitor_me/backend/internal/config"
	"github.com/WorkRCS/monitor_me/backend/internal/httpapi"
	"github.com/WorkRCS/monitor_me/backend/internal/storage/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("API server stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	startupContext, cancelStartup := context.WithTimeout(context.Background(), cfg.StartupTimeout)
	defer cancelStartup()

	database, err := postgres.Open(startupContext, postgres.Config{
		URL:                cfg.DatabaseURL,
		MaxOpenConnections: cfg.DBMaxOpenConnections,
		MaxIdleConnections: cfg.DBMaxIdleConnections,
		ConnectionLifetime: cfg.DBConnectionLifetime,
		IdleConnectionTime: 5 * time.Minute,
	})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Warn("close PostgreSQL pool", "error", closeErr)
		}
	}()

	cache, err := redisclient.Open(startupContext, redisclient.Config{
		Address:      cfg.RedisAddress,
		Password:     cfg.RedisPassword,
		PoolSize:     16,
		MinIdleConns: 2,
		DialTimeout:  3 * time.Second,
		ReadTimeout:  2 * time.Second,
		WriteTimeout: 2 * time.Second,
	})
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := cache.Close(); closeErr != nil {
			logger.Warn("close Redis client", "error", closeErr)
		}
	}()

	dependencies := []httpapi.Dependency{
		{Name: "postgres", Check: database.PingContext},
		{Name: "redis", Check: func(ctx context.Context) error { return cache.Ping(ctx).Err() }},
	}

	server := &http.Server{
		Addr:              cfg.APIAddress,
		Handler:           httpapi.NewRouter(cfg.ServiceVersion, dependencies...),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Create the signal context before starting the listener so shutdown
	// signals cannot be missed during server startup.
	shutdownSignals, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serverErrors := make(chan error, 1)

	go func() {
		logger.Info("API server starting", "address", cfg.APIAddress)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()

	select {
	case err := <-serverErrors:
		return err
	case <-shutdownSignals.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownContext); err != nil {
		if closeErr := server.Close(); closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}

	logger.Info("API server stopped gracefully")
	return nil
}

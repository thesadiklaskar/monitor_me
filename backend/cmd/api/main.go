package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	monitorv1 "github.com/WorkRCS/monitor_me/backend/gen/monitor/v1"
	"github.com/WorkRCS/monitor_me/backend/internal/agentcontrol"
	redisclient "github.com/WorkRCS/monitor_me/backend/internal/cache/redis"
	"github.com/WorkRCS/monitor_me/backend/internal/config"
	"github.com/WorkRCS/monitor_me/backend/internal/httpapi"
	"github.com/WorkRCS/monitor_me/backend/internal/identity"
	"github.com/WorkRCS/monitor_me/backend/internal/storage/postgres"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

	certificate, err := tls.LoadX509KeyPair(cfg.AgentTLSCertFile, cfg.AgentTLSKeyFile)
	if err != nil {
		return errors.New("load agent gRPC TLS certificate and key")
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

	httpServer := &http.Server{
		Addr:              cfg.APIAddress,
		Handler:           httpapi.NewRouter(cfg.ServiceVersion, dependencies, identity.NewStore(database)),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	grpcListener, err := net.Listen("tcp", cfg.AgentGRPCAddress)
	if err != nil {
		return errors.New("listen on agent gRPC address")
	}
	grpcServer := grpc.NewServer(
		grpc.Creds(credentials.NewTLS(&tls.Config{
			Certificates: []tls.Certificate{certificate},
			MinVersion:   tls.VersionTLS12,
		})),
		grpc.MaxRecvMsgSize(1<<20),
		grpc.MaxSendMsgSize(1<<20),
		grpc.MaxConcurrentStreams(128),
	)
	monitorv1.RegisterAgentControlServer(grpcServer, agentcontrol.NewService(database))

	shutdownSignals, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	serverErrors := make(chan error, 2)
	go func() {
		logger.Info("HTTP API starting", "address", cfg.APIAddress)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			serverErrors <- err
		}
	}()
	go func() {
		logger.Info("agent gRPC server starting with TLS", "address", cfg.AgentGRPCAddress)
		if err := grpcServer.Serve(grpcListener); err != nil && !errors.Is(err, grpc.ErrServerStopped) {
			serverErrors <- err
		}
	}()

	var serveErr error
	select {
	case serveErr = <-serverErrors:
		if serveErr != nil {
			logger.Error("service listener stopped unexpectedly", "error", serveErr)
		}
	case <-shutdownSignals.Done():
		logger.Info("shutdown signal received")
	}

	shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancelShutdown()

	grpcStopped := make(chan struct{})
	go func() {
		grpcServer.GracefulStop()
		close(grpcStopped)
	}()
	select {
	case <-grpcStopped:
	case <-time.After(5 * time.Second):
		grpcServer.Stop()
	}

	if err := httpServer.Shutdown(shutdownContext); err != nil {
		_ = httpServer.Close()
		if serveErr == nil {
			serveErr = err
		}
	}

	if serveErr != nil {
		return serveErr
	}
	logger.Info("HTTP and agent gRPC servers stopped gracefully")
	return nil
}

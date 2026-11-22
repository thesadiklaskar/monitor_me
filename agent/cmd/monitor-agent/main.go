package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/WorkRCS/monitor_me/agent/internal/collector"
	"github.com/WorkRCS/monitor_me/agent/internal/config"
	"github.com/WorkRCS/monitor_me/agent/internal/spool"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("invalid agent configuration", "error", err)
		os.Exit(2)
	}
	level := new(slog.LevelVar)
	switch cfg.LogLevel {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	if err := run(cfg, logger); err != nil {
		logger.Error("agent stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(cfg config.Config, logger *slog.Logger) error {
	store, err := spool.Open(cfg.SpoolDirectory, cfg.SpoolMaxBytes)
	if err != nil {
		return err
	}
	hostCollector := collector.NewProcCollector(cfg.ProcRoot)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	logger.Info("native monitoring agent started",
		"version", cfg.Version,
		"collection_interval", cfg.CollectionInterval.String(),
		"capabilities", hostCollector.Capabilities(),
		"spool_directory", cfg.SpoolDirectory,
	)
	ticker := time.NewTicker(cfg.CollectionInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			bytes, batches, statsErr := store.Stats()
			if statsErr != nil {
				logger.Error("could not inspect pending spool on shutdown", "error", statsErr)
			}
			logger.Info("agent shutdown complete", "pending_batches", batches, "spool_bytes", bytes)
			return nil
		case <-ticker.C:
			samples, collectErr := hostCollector.Collect(ctx)
			if len(samples) > 0 {
				batch, batchErr := spool.NewBatch(samples, time.Now().UTC().UnixNano())
				if batchErr == nil {
					batchErr = store.Append(batch)
				}
				if batchErr != nil {
					logger.Error("could not persist telemetry batch", "error", batchErr)
				} else {
					logger.Debug("telemetry batch persisted locally", "batch_id", batch.ID, "sample_count", len(samples))
				}
			}
			if collectErr != nil {
				logger.Warn("one or more host collectors failed", "error", collectErr)
			}
		}
	}
}

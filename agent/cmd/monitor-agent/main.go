package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/WorkRCS/monitor_me/agent/internal/collector"
	"github.com/WorkRCS/monitor_me/agent/internal/config"
	"github.com/WorkRCS/monitor_me/agent/internal/health"
	"github.com/WorkRCS/monitor_me/agent/internal/spool"
	"github.com/WorkRCS/monitor_me/agent/internal/state"
	"github.com/WorkRCS/monitor_me/agent/internal/transport"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func main() {
	if err := loadConfigFileArgument(); err != nil {
		slog.Error("could not load agent service configuration", "error", err)
		os.Exit(2)
	}
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
	if err := runPlatformService(cfg, logger); err != nil {
		logger.Error("agent stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := spool.Open(cfg.SpoolDirectory, cfg.SpoolMaxBytes)
	if err != nil {
		return err
	}
	hostCollector := collector.NewProcCollector(cfg.ProcRoot)
	healthState := health.New()
	logger.Info("native monitoring agent started",
		"version", cfg.Version,
		"collection_interval", cfg.CollectionInterval.String(),
		"capabilities", hostCollector.Capabilities(),
		"spool_directory", cfg.SpoolDirectory,
		"transport_enabled", cfg.GRPCAddress != "",
	)
	if cfg.GRPCAddress != "" {
		go runRemote(ctx, cfg, hostCollector.Capabilities(), store, healthState, logger)
	}

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
			if collectErr != nil {
				logger.Warn("one or more host collectors failed", "error", collectErr)
			}
			spoolBytes, pendingBatches, statsErr := store.Stats()
			if statsErr != nil {
				logger.Error("could not inspect telemetry spool", "error", statsErr)
			} else {
				// The snapshot is taken before this cycle's batch is appended, so
				// depth and size can lag the current health batch by one interval.
				samples = append(samples, healthState.Samples(
					time.Now().UTC(), health.CollectorErrorCount(collectErr), spoolBytes, pendingBatches,
				)...)
			}
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
		}
	}
}

func runRemote(ctx context.Context, cfg config.Config, capabilities []string, store *spool.Store, healthState *health.State, logger *slog.Logger) {
	backoff := time.Second
	for ctx.Err() == nil {
		client, err := transport.Dial(cfg.GRPCAddress, cfg.TLSCAFile)
		if err != nil {
			logger.Warn("agent transport initialization failed", "error", err, "retry_in", backoff.String())
			if !wait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		hostname, hostErr := os.Hostname()
		if hostErr != nil || hostname == "" {
			hostname = "unknown-host"
		}
		session, err := client.LoadOrEnroll(ctx, cfg, hostname, capabilities)
		if err != nil {
			_ = client.Close()
			logger.Warn("agent enrollment or session load failed", "error", err, "retry_in", backoff.String())
			if !wait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		heartbeatContext, cancelHeartbeat := context.WithTimeout(ctx, 5*time.Second)
		err = client.ReportHeartbeat(heartbeatContext, session, cfg.Version, capabilities)
		cancelHeartbeat()
		if err != nil {
			if status.Code(err) == codes.Unauthenticated && cfg.EnrollmentToken != "" {
				if removeErr := state.Remove(cfg.StateFile); removeErr != nil {
					logger.Error("could not remove rejected agent session", "error", removeErr)
				}
			}
			_ = client.Close()
			logger.Warn("initial agent heartbeat failed", "error", err, "retry_in", backoff.String())
			if !wait(ctx, backoff) {
				return
			}
			backoff = nextBackoff(backoff)
			continue
		}

		backoff = time.Second
		heartbeatInterval := time.Duration(session.HeartbeatIntervalSeconds) * time.Second
		if heartbeatInterval < 5*time.Second || heartbeatInterval > 5*time.Minute {
			heartbeatInterval = 15 * time.Second
		}
		heartbeatTicker := time.NewTicker(heartbeatInterval)
		deliveryTicker := time.NewTicker(5 * time.Second)
		logger.Info("agent session active", "agent_id", session.AgentID, "server_id", session.ServerID)

		connected := true
		for connected {
			select {
			case <-ctx.Done():
				connected = false
			case <-heartbeatTicker.C:
				callContext, cancel := context.WithTimeout(ctx, 5*time.Second)
				heartbeatErr := client.ReportHeartbeat(callContext, session, cfg.Version, capabilities)
				cancel()
				if heartbeatErr != nil {
					logger.Warn("agent heartbeat failed; reconnecting", "error", heartbeatErr)
					if status.Code(heartbeatErr) == codes.Unauthenticated && cfg.EnrollmentToken != "" {
						if removeErr := state.Remove(cfg.StateFile); removeErr != nil {
							logger.Error("could not remove rejected agent session", "error", removeErr)
						}
					}
					connected = false
				}
			case <-deliveryTicker.C:
				callContext, cancel := context.WithTimeout(ctx, 30*time.Second)
				sent, sendErr := client.SendPending(callContext, session, store, cfg.AllowedSystemdUnits)
				cancel()
				if sent > 0 {
					healthState.MarkSuccessfulSend(time.Now().UTC())
					logger.Debug("telemetry batches acknowledged by control plane", "batch_count", sent)
				}
				if sendErr != nil {
					logger.Warn("telemetry delivery failed; reconnecting", "error", sendErr)
					if status.Code(sendErr) == codes.Unauthenticated && cfg.EnrollmentToken != "" {
						if removeErr := state.Remove(cfg.StateFile); removeErr != nil {
							logger.Error("could not remove rejected agent session", "error", removeErr)
						}
					}
					connected = false
				}
			}
		}
		heartbeatTicker.Stop()
		deliveryTicker.Stop()
		_ = client.Close()
	}
}

func wait(ctx context.Context, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current time.Duration) time.Duration {
	if current >= 15*time.Second {
		return 30 * time.Second
	}
	return current * 2
}

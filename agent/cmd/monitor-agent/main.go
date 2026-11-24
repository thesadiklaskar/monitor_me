package main

import (
	"context"
	"log/slog"
	"os"
	"runtime"
	"time"

	"example.com/monitor-me/agent/internal/collector"
	"example.com/monitor-me/agent/internal/config"
	"example.com/monitor-me/agent/internal/health"
	"example.com/monitor-me/agent/internal/logcollector"
	"example.com/monitor-me/agent/internal/logspool"
	"example.com/monitor-me/agent/internal/spool"
	"example.com/monitor-me/agent/internal/state"
	"example.com/monitor-me/agent/internal/transport"
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
	logWriter, closeLog, logErr := openAgentLogWriter(os.Getenv("MONITOR_LOG_FILE"))
	if logErr != nil {
		slog.Error("could not initialize agent log output", "error", logErr)
		os.Exit(2)
	}
	defer closeLog()
	logger := slog.New(slog.NewJSONHandler(logWriter, &slog.HandlerOptions{Level: level}))
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
	logStore, err := logspool.Open(cfg.LogSpoolDirectory, cfg.LogSpoolMaxBytes)
	if err != nil {
		return err
	}
	journal := logcollector.NewJournal()
	capabilities := append([]string(nil), hostCollector.Capabilities()...)
	if journal.Available() {
		capabilities = append(capabilities, journal.Capability())
		logger.Info("native event log collection enabled", "capability", journal.Capability(), "log_spool_directory", cfg.LogSpoolDirectory)
	} else if runtime.GOOS == "linux" {
		logger.Warn("journalctl not found; host metrics remain active but journald collection is unavailable")
	} else if runtime.GOOS == "windows" {
		logger.Warn("PowerShell event-log reader not found; host metrics remain active but Windows Event Log collection is unavailable")
	}
	healthState := health.New()
	logger.Info("native monitoring agent started",
		"version", cfg.Version,
		"collection_interval", cfg.CollectionInterval.String(),
		"capabilities", capabilities,
		"spool_directory", cfg.SpoolDirectory,
		"transport_enabled", cfg.GRPCAddress != "",
	)
	if cfg.GRPCAddress != "" {
		go runRemote(ctx, cfg, capabilities, store, logStore, journal, healthState, logger)
	}

	ticker := time.NewTicker(cfg.CollectionInterval)
	defer ticker.Stop()
	logTicker := time.NewTicker(journal.PollInterval())
	defer logTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			bytes, batches, statsErr := store.Stats()
			if statsErr != nil {
				logger.Error("could not inspect pending spool on shutdown", "error", statsErr)
			}
			logger.Info("agent shutdown complete", "pending_batches", batches, "spool_bytes", bytes)
			return nil
		case <-logTicker.C:
			if !journal.Available() || cfg.GRPCAddress == "" {
				continue
			}
			// Read at most 16 bounded pages per tick. A full local WAL stops
			// collection without advancing the durable journal cursor.
			for page := 0; page < 16; page++ {
				cursor, cursorErr := logStore.QueuedCursor()
				if cursorErr != nil {
					logger.Error("could not read queued journal cursor", "error", cursorErr)
					break
				}
				events, nextCursor, collectErr := journal.Collect(ctx, cursor, 32)
				if collectErr != nil {
					logger.Warn("journald collection failed", "error", collectErr)
					break
				}
				if len(events) == 0 {
					break
				}
				if nextCursor == cursor {
					logger.Warn("journald page did not advance its cursor; stopping collection")
					break
				}
				batch, batchErr := logspool.NewBatch(events, nextCursor, time.Now().UTC().UnixNano())
				if batchErr == nil {
					batchErr = logStore.Append(batch)
				}
				if batchErr != nil {
					logger.Error("could not persist local log batch", "error", batchErr, "event_count", len(events))
					break
				}
				logger.Debug("journald batch persisted locally", "batch_id", batch.ID, "event_count", len(events))
			}
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

func runRemote(ctx context.Context, cfg config.Config, capabilities []string, store *spool.Store, logs *logspool.Store, journal *logcollector.Journal, healthState *health.State, logger *slog.Logger) {
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
				if sent > 0 {
					healthState.MarkSuccessfulSend(time.Now().UTC())
					logger.Debug("telemetry batches acknowledged by control plane", "batch_count", sent)
				}
				if sendErr == nil && journal.Available() {
					logSent, logErr := client.SendPendingLogs(callContext, session, logs)
					if logSent > 0 {
						healthState.MarkSuccessfulSend(time.Now().UTC())
						logger.Debug("log batches acknowledged by control plane", "batch_count", logSent)
					}
					if logErr != nil {
						sendErr = logErr
					}
				}
				cancel()
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

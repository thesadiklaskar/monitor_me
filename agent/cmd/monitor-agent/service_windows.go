//go:build windows

package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/WorkRCS/monitor_me/agent/internal/config"
	"golang.org/x/sys/windows/svc"
)

const windowsServiceName = "MonitorMeAgent"

type monitorMeWindowsService struct {
	config config.Config
	logger *slog.Logger
}

func runPlatformService(cfg config.Config, logger *slog.Logger) error {
	isService, err := svc.IsWindowsService()
	if err != nil {
		return err
	}
	if !isService {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		return run(ctx, cfg, logger)
	}
	return svc.Run(windowsServiceName, &monitorMeWindowsService{config: cfg, logger: logger})
}

func (service *monitorMeWindowsService) Execute(
	_ []string,
	requests <-chan svc.ChangeRequest,
	statuses chan<- svc.Status,
) (bool, uint32) {
	statuses <- svc.Status{State: svc.StartPending}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	finished := make(chan error, 1)
	go func() {
		finished <- run(ctx, service.config, service.logger)
	}()

	current := svc.Status{
		State:   svc.Running,
		Accepts: svc.AcceptStop | svc.AcceptShutdown | svc.AcceptInterrogate,
	}
	statuses <- current
	for {
		select {
		case request, ok := <-requests:
			if !ok {
				cancel()
				if err := <-finished; err != nil {
					service.logger.Error("Windows service stopped with an error", "error", err)
					return false, 1
				}
				return false, 0
			}
			switch request.Cmd {
			case svc.Interrogate:
				statuses <- current
			case svc.Stop, svc.Shutdown:
				statuses <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-finished; err != nil {
					service.logger.Error("Windows service shutdown failed", "error", err)
					return false, 1
				}
				return false, 0
			default:
				service.logger.Warn("unsupported Windows service control request", "command", request.Cmd)
			}
		case err := <-finished:
			if err != nil {
				service.logger.Error("Windows service runner failed", "error", err)
				return false, 1
			}
			return false, 0
		}
	}
}

package service

import (
	"context"
	"log/slog"
	"time"

	"probakgo/internal/store"
)

// StartAlertScheduler detects expired heartbeats even when no client sends a
// report. It also retries failed deliveries after transient network failures.
func StartAlertScheduler(ctx context.Context, st *store.Store, rep *ReportService) {
	go runAlertScheduler(ctx, st, rep, time.Minute)
}

func runAlertScheduler(ctx context.Context, st *store.Store, rep *ReportService, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if ctx.Err() != nil {
			return
		}
		if err := sendImmediateCriticalAlerts(ctx, st, rep); err != nil && ctx.Err() == nil {
			slog.Warn("periodic alert evaluation", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

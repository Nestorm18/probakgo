package service

import (
	"context"
	"log/slog"
	"time"

	"probakgo/internal/store"
)

// StartCleanupScheduler runs report retention cleanup once at startup and then every 24 hours.
func StartCleanupScheduler(ctx context.Context, st *store.Store) {
	go func() {
		runCleanup(ctx, st)
		for {
			select {
			case <-ctx.Done():
				return
			case <-time.After(24 * time.Hour):
				runCleanup(ctx, st)
			}
		}
	}()
}

func runCleanup(parent context.Context, st *store.Store) {
	ctx, cancel := context.WithTimeout(parent, 30*time.Second)
	defer cancel()
	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		slog.Error("cleanup: get config", "err", err)
		return
	}
	if !cfg.RetentionEnabled || cfg.RetentionMonths <= 0 {
		return
	}

	cutoff := time.Now().AddDate(0, -cfg.RetentionMonths, 0)

	pve, err := st.DeleteOldPVEReports(ctx, cutoff)
	if err != nil {
		slog.Error("cleanup: delete PVE reports", "err", err)
	}
	pbs, err := st.DeleteOldPBSReports(ctx, cutoff)
	if err != nil {
		slog.Error("cleanup: delete PBS reports", "err", err)
	}

	if pve+pbs > 0 {
		slog.Info("cleanup: old reports deleted",
			"pve_reports", pve, "pbs_reports", pbs,
			"cutoff", cutoff.Format("2006-01-02"),
			"retention_months", cfg.RetentionMonths)
	}
}

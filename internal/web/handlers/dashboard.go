package webhandlers

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

func (h *WebH) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	pveServers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		slog.Error("list pve servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pbsServers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		slog.Error("list pbs servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}

	pveReports, err := h.store.GetLatestPVEReports(ctx)
	if err != nil {
		slog.Error("list latest pve reports", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pveReportIDs := make([]int64, 0, len(pveReports))
	for _, rep := range pveReports {
		pveReportIDs = append(pveReportIDs, rep.ID)
	}
	pveTasks, err := h.store.GetPVEBackupTasksForReports(ctx, pveReportIDs)
	if err != nil {
		slog.Error("list pve backup tasks", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}

	pveBackupErrors := h.activePVEBackupErrorServers(ctx, pveServers, pveReports, pveTasks)
	var pveOK, pveStale int
	var pveRows []map[string]any
	for _, sv := range pveServers {
		rep := pveReports[sv.ID]
		configs, _ := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		ignoreStale := len(configs) > 0 && !domain.HasActiveVMBackupConfigs(configs)
		isStale := (rep == nil || rep.IsStale) && !ignoreStale
		hasBackupError := pveBackupErrors[sv.ID]
		if isStale {
			pveStale++
		} else if hasBackupError {
			// Counted separately as "backup con error".
		} else {
			pveOK++
		}
		row := map[string]any{"Server": sv, "IsStale": isStale, "HasBackupError": hasBackupError, "Swap": buildSwapView(false, 0, 0)}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
			row["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)
		}
		pveRows = append(pveRows, row)
	}

	pbsReports, err := h.store.GetLatestPBSReports(ctx)
	if err != nil {
		slog.Error("list latest pbs reports", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pbsReportIDs := make([]int64, 0, len(pbsReports))
	for _, rep := range pbsReports {
		pbsReportIDs = append(pbsReportIDs, rep.ID)
	}
	pbsStores, err := h.store.GetPBSStoresForReports(ctx, pbsReportIDs)
	if err != nil {
		slog.Error("list pbs stores", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}

	var pbsOK, pbsStale int
	var pbsRows []map[string]any
	for _, sv := range pbsServers {
		rep := pbsReports[sv.ID]
		isStale := rep == nil || rep.IsStale
		fillLabel, fillClass := "Llenado OK", "ok"
		if isStale {
			pbsStale++
		} else {
			pbsOK++
			fillLabel, fillClass = pbsFillBadge(pbsStores[rep.ID])
		}
		row := map[string]any{
			"Server":    sv,
			"IsStale":   isStale,
			"FillLabel": fillLabel,
			"FillClass": fillClass,
			"Swap":      buildSwapView(false, 0, 0),
		}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
			row["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)
		}
		pbsRows = append(pbsRows, row)
	}

	alertCritical, alertWarning := service.ActiveAlertCounts(ctx, h.store, h.report)
	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"AlertCritical":   alertCritical,
		"AlertWarning":    alertWarning,
		"PVERows":         pveRows,
		"PBSRows":         pbsRows,
		"PVEOk":           pveOK,
		"PVEStale":        pveStale,
		"PVEBackupErrors": len(pveBackupErrors),
		"PBSOk":           pbsOK,
		"PBSStale":        pbsStale,
	})
}

func (h *WebH) activePVEBackupErrorServers(ctx context.Context, servers []domain.PVEServer, reports map[int64]*domain.PVEReport, tasksByReport map[int64][]domain.PVEBackupTask) map[int64]bool {
	result := make(map[int64]bool)
	cfg, err := service.LoadAlertConfigs(ctx, h.store)
	if err != nil {
		return result
	}
	suppressed, _ := h.store.GetActiveSuppressions(ctx)
	for _, sv := range servers {
		rep := reports[sv.ID]
		if rep == nil {
			continue
		}
		svCfg := cfg.PVEConfigs[sv.ID]
		tasks := tasksByReport[rep.ID]
		if len(tasks) == 0 {
			status := strings.TrimSpace(rep.BackupStatus)
			if status == "" || strings.EqualFold(status, "OK") {
				continue
			}
			if !dashboardBackupErrEnabled(svCfg, nil, cfg.GlobalBackupErr) {
				continue
			}
			if _, ok := suppressed[fmt.Sprintf("backup_error:pve:%d", sv.ID)]; ok {
				continue
			}
			result[sv.ID] = true
			continue
		}
		for _, task := range tasks {
			if strings.EqualFold(strings.TrimSpace(task.Status), "OK") {
				continue
			}
			vmCfg := dashboardFindVMConfig(cfg.PVEVMConfigs[sv.ID], task.VMID)
			if !dashboardBackupErrEnabled(svCfg, vmCfg, cfg.GlobalBackupErr) {
				continue
			}
			if _, ok := suppressed[fmt.Sprintf("backup_error:pve:%d:%d", sv.ID, task.VMID)]; ok {
				continue
			}
			result[sv.ID] = true
			break
		}
	}
	return result
}

func dashboardBackupErrEnabled(svCfg domain.PVEAlertConfig, vmCfg *domain.PVEVMAlertConfig, global bool) bool {
	if vmCfg != nil && vmCfg.BackupErr != nil {
		return *vmCfg.BackupErr != 0
	}
	if svCfg.BackupErr != nil {
		return *svCfg.BackupErr != 0
	}
	return global
}

func dashboardFindVMConfig(configs []domain.PVEVMAlertConfig, vmid int64) *domain.PVEVMAlertConfig {
	for i := range configs {
		if configs[i].VMID == vmid {
			return &configs[i]
		}
	}
	return nil
}

func pbsFillBadge(stores []domain.PBSStore) (label, class string) {
	if len(stores) == 0 {
		return "Llenado OK", "ok"
	}
	now := time.Now()
	var nearest *time.Time
	maxPct := 0
	for _, store := range stores {
		if store.Total > 0 {
			pct := int(float64(store.Used) / float64(store.Total) * 100)
			if pct > maxPct {
				maxPct = pct
			}
		}
		if store.EstimatedFullDate == 0 {
			continue
		}
		fullAt := time.Unix(store.EstimatedFullDate, 0)
		if !fullAt.After(now) {
			continue
		}
		if nearest == nil || fullAt.Before(*nearest) {
			nearest = &fullAt
		}
	}
	if nearest == nil {
		switch {
		case maxPct > 95:
			return fmt.Sprintf("%d%% · Sin riesgo", maxPct), "ok"
		case maxPct > 85:
			return fmt.Sprintf("%d%% · Sin riesgo", maxPct), "ok"
		}
		return "Llenado OK", "ok"
	}
	days := pbsDaysUntil(*nearest, now)
	return fmt.Sprintf("Lleno en %dd", days), pbsFillClass(days)
}

type pbsStoreDisplay struct {
	StoreName  string
	Used       int64
	Total      int64
	BadgeLabel string
	BadgeClass string
	BadgeTitle string
	NoFillRisk bool
}

func pbsStoreDisplays(stores []domain.PBSStore) []pbsStoreDisplay {
	now := time.Now()
	rows := make([]pbsStoreDisplay, 0, len(stores))
	for _, store := range stores {
		row := pbsStoreDisplay{
			StoreName: store.Store,
			Used:      store.Used,
			Total:     store.Total,
		}
		if store.EstimatedFullDate > 0 {
			fullAt := time.Unix(store.EstimatedFullDate, 0)
			if fullAt.After(now) {
				days := pbsDaysUntil(fullAt, now)
				row.BadgeLabel = fmt.Sprintf("Lleno en %dd", days)
				row.BadgeClass = pbsFillClass(days)
				if days <= 14 {
					row.BadgeTitle = "Se llena en menos de 14 días"
				} else {
					row.BadgeTitle = fmt.Sprintf("Se llena en %d días", days)
				}
				rows = append(rows, row)
				continue
			}
		}
		if store.Total > 0 {
			pct := int(float64(store.Used) / float64(store.Total) * 100)
			switch {
			case pct > 95:
				row.BadgeLabel = fmt.Sprintf("%d%%", pct)
				row.BadgeClass = "ok"
				row.BadgeTitle = fmt.Sprintf("Disco al %d%%", pct)
				row.NoFillRisk = true
			case pct > 85:
				row.BadgeLabel = fmt.Sprintf("%d%%", pct)
				row.BadgeClass = "ok"
				row.BadgeTitle = fmt.Sprintf("Disco al %d%%", pct)
				row.NoFillRisk = true
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func pbsDaysUntil(fullAt, now time.Time) int {
	until := fullAt.Sub(now)
	days := int(until.Hours() / 24)
	if until%(24*time.Hour) != 0 {
		days++
	}
	if days < 1 {
		return 1
	}
	return days
}

func pbsFillClass(days int) string {
	switch {
	case days <= 14:
		return "bad"
	case days <= 30:
		return "warn"
	default:
		return "ok"
	}
}

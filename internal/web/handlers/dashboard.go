package webhandlers

import (
	"context"
	"log/slog"
	"net/http"

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

	pveBackupErrors := h.activePVEBackupErrorServers(ctx)
	var pveOK, pveStale int
	var pveRows []map[string]any
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(ctx, sv.ID)
		isStale := err != nil || rep.IsStale
		hasBackupError := pveBackupErrors[sv.ID]
		if isStale {
			pveStale++
		} else if hasBackupError {
			// Counted separately as "backup con error".
		} else {
			pveOK++
		}
		row := map[string]any{"Server": sv, "IsStale": isStale, "HasBackupError": hasBackupError}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
		}
		pveRows = append(pveRows, row)
	}

	var pbsOK, pbsStale int
	var pbsRows []map[string]any
	for _, sv := range pbsServers {
		rep, err := h.store.GetLatestPBSReport(ctx, sv.ID)
		isStale := err != nil || rep.IsStale
		if isStale {
			pbsStale++
		} else {
			pbsOK++
		}
		row := map[string]any{"Server": sv, "IsStale": isStale}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
		}
		pbsRows = append(pbsRows, row)
	}

	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"PVERows":         pveRows,
		"PBSRows":         pbsRows,
		"PVEOk":           pveOK,
		"PVEStale":        pveStale,
		"PVEBackupErrors": len(pveBackupErrors),
		"PBSOk":           pbsOK,
		"PBSStale":        pbsStale,
	})
}

func (h *WebH) activePVEBackupErrorServers(ctx context.Context) map[int64]bool {
	result := make(map[int64]bool)
	cfg, err := service.LoadAlertConfigs(ctx, h.store)
	if err != nil {
		return result
	}
	cfg.Report = h.report
	alerts, err := service.RunAll(h.store, cfg)
	if err != nil {
		return result
	}
	suppressed, _ := h.store.GetActiveSuppressions(ctx)
	for _, alert := range alerts {
		if alert.Type != domain.AlertTypeBackupError || alert.ServerType != "pve" {
			continue
		}
		if _, ok := suppressed[alert.ID]; ok {
			continue
		}
		result[alert.ServerID] = true
	}
	return result
}

package webhandlers

import (
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func (h *WebH) applyServerMaintenanceFromForm(r *http.Request, serverType string, serverID int64) bool {
	ctx := r.Context()
	if r.FormValue("maintenance_enabled") != "on" {
		if err := h.store.DeleteServerMaintenance(ctx, serverType, serverID); err != nil {
			slog.Warn("delete server maintenance", "err", err, "server_type", serverType, "server_id", serverID)
		}
		return false
	}
	hoursValue := r.FormValue("maintenance_hours")
	if hoursValue == "keep" {
		if current, err := h.store.GetServerMaintenance(ctx, serverType, serverID); err == nil && current.Active {
			return true
		}
		hoursValue = "24"
	}
	hours, err := strconv.Atoi(hoursValue)
	if err != nil || hours <= 0 {
		hours = 24
	}
	if hours > 24*365 {
		hours = 24 * 365
	}
	reason := strings.TrimSpace(r.FormValue("maintenance_reason"))
	if reason == "" {
		reason = "Modo mantenimiento"
	}
	until := time.Now().Add(time.Duration(hours) * time.Hour)
	if err := h.store.UpsertServerMaintenance(ctx, serverType, serverID, until, reason); err != nil {
		slog.Warn("upsert server maintenance", "err", err, "server_type", serverType, "server_id", serverID)
		return false
	}
	return true
}

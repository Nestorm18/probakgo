package webhandlers

import (
	"net/http"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) Dashboard(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)

	pveServers, err := h.store.ListPVEServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pbsServers, err := h.store.ListPBSServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var pveOK, pveStale int
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(sv.ID)
		if err != nil || rep.IsStale {
			pveStale++
		} else {
			pveOK++
		}
	}
	var pbsOK, pbsStale int
	for _, sv := range pbsServers {
		rep, err := h.store.GetLatestPBSReport(sv.ID)
		if err != nil || rep.IsStale {
			pbsStale++
		} else {
			pbsOK++
		}
	}

	var alerts []domain.Alert
	if cfg, err := h.store.GetEmailConfig(); err == nil {
		alerts, _ = h.store.GetAlerts(cfg.AlertDiskPct, cfg.AlertBackupErr)
	}

	h.tmpl.Render(w, "dashboard.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"PVEServers": pveServers,
		"PBSServers": pbsServers,
		"PVEOk":      pveOK,
		"PVEStale":   pveStale,
		"PBSOk":      pbsOK,
		"PBSStale":   pbsStale,
		"Alerts":     alerts,
	})
}

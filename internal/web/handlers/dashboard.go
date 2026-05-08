package webhandlers

import (
	"net/http"

	"probakgo/internal/domain"
	"probakgo/internal/service"
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

	var critical, warning int
	if cfg, err := service.LoadAlertConfigs(h.store); err == nil {
		alerts, _ := service.RunAll(h.store, cfg)
		for _, a := range alerts {
			if a.Severity == domain.AlertSeverityCritical {
				critical++
			} else {
				warning++
			}
		}
	}

	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":       username,
		"Role":           role,
		"PVEServers":     pveServers,
		"PBSServers":     pbsServers,
		"PVEOk":          pveOK,
		"PVEStale":       pveStale,
		"PBSOk":          pbsOK,
		"PBSStale":       pbsStale,
		"AlertCritical":  critical,
		"AlertWarning":   warning,
	})
}

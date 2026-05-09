package webhandlers

import (
	"net/http"

	"probakgo/internal/session"
)

func (h *WebH) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	pveServers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pbsServers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var pveOK, pveStale int
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(ctx, sv.ID)
		if err != nil || rep.IsStale {
			pveStale++
		} else {
			pveOK++
		}
	}
	var pbsOK, pbsStale int
	for _, sv := range pbsServers {
		rep, err := h.store.GetLatestPBSReport(ctx, sv.ID)
		if err != nil || rep.IsStale {
			pbsStale++
		} else {
			pbsOK++
		}
	}

	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"PVEServers": pveServers,
		"PBSServers": pbsServers,
		"PVEOk":      pveOK,
		"PVEStale":   pveStale,
		"PBSOk":      pbsOK,
		"PBSStale":   pbsStale,
	})
}

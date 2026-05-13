package webhandlers

import (
	"log/slog"
	"net/http"

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

	var pveOK, pveStale int
	var pveRows []map[string]any
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(ctx, sv.ID)
		isStale := err != nil || rep.IsStale
		if isStale {
			pveStale++
		} else {
			pveOK++
		}
		row := map[string]any{"Server": sv, "IsStale": isStale}
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
		"Username": username,
		"Role":     role,
		"PVERows":  pveRows,
		"PBSRows":  pbsRows,
		"PVEOk":    pveOK,
		"PVEStale": pveStale,
		"PBSOk":    pbsOK,
		"PBSStale": pbsStale,
	})
}

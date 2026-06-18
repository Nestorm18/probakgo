package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"probakgo/internal/api/apictx"
	"probakgo/internal/domain"
)

func (h *H) ReportPVE(w http.ResponseWriter, r *http.Request) {
	var req domain.PVEReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	if req.Hostname == "" {
		errJSON(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if !h.requireKeyServer(w, r, req.Hostname) {
		return
	}
	k, _ := apictx.APIKey(r.Context())
	if err := h.report.SavePVEReportForAPIKey(r.Context(), &req, k.ID); err != nil {
		internalErr(w, "save pve report", err)
		return
	}
	h.sendImmediateCriticalAlerts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "server": req.Hostname})
}

func (h *H) ReportPBS(w http.ResponseWriter, r *http.Request) {
	var req domain.PBSReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	if req.Hostname == "" {
		errJSON(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if !h.requireKeyServer(w, r, req.Hostname) {
		return
	}
	k, _ := apictx.APIKey(r.Context())
	if err := h.report.SavePBSReportForAPIKey(r.Context(), &req, k.ID); err != nil {
		internalErr(w, "save pbs report", err)
		return
	}
	h.sendImmediateCriticalAlerts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "server": req.Hostname})
}

func (h *H) ReportWindows(w http.ResponseWriter, r *http.Request) {
	var req domain.WindowsReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	if req.Hostname == "" {
		errJSON(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if !h.requireKeyServer(w, r, req.Hostname) {
		return
	}
	k, _ := apictx.APIKey(r.Context())
	if err := h.report.SaveWindowsReportForAPIKey(r.Context(), &req, k.ID); err != nil {
		internalErr(w, "save windows report", err)
		return
	}
	if sv, err := h.store.GetWindowsServerByName(r.Context(), req.Hostname); err == nil {
		_ = h.store.UpsertServerHeartbeat(r.Context(), domain.ServerHeartbeat{
			ServerType:    "windows",
			ServerID:      sv.ID,
			Hostname:      req.Hostname,
			IP:            req.IPAddress,
			PublicIP:      req.PublicIP,
			ClientVersion: req.ClientVersion,
			MachineID:     req.MachineID,
			LastSeenAt:    time.Now(),
		})
	}
	h.sendImmediateCriticalAlerts()
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "server": req.Hostname})
}

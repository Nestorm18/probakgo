package handlers

import (
	"encoding/json"
	"net/http"
	"strings"

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
	if err := h.report.SavePVEReport(r.Context(), &req); err != nil {
		internalErr(w, "save pve report", err)
		return
	}
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
	if err := h.report.SavePBSReport(r.Context(), &req); err != nil {
		internalErr(w, "save pbs report", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok", "server": req.Hostname})
}

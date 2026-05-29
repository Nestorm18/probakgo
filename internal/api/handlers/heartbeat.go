package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"probakgo/internal/domain"
)

func (h *H) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req domain.HeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	req.ServerType = strings.ToLower(strings.TrimSpace(req.ServerType))
	if req.Hostname == "" {
		errJSON(w, http.StatusBadRequest, "hostname is required")
		return
	}
	if req.ServerType == "" {
		req.ServerType = "pve"
	}
	if req.ServerType != "pve" && req.ServerType != "pbs" {
		errJSON(w, http.StatusBadRequest, "server_type must be pve or pbs")
		return
	}
	if !h.requireKeyServer(w, r, req.Hostname) {
		return
	}

	var (
		serverID int64
		err      error
	)
	switch req.ServerType {
	case "pve":
		if req.PublicIP == "" {
			if sv, getErr := h.store.GetPVEServerByName(r.Context(), req.Hostname); getErr == nil {
				req.PublicIP = sv.PublicIP
			}
		}
		serverID, err = h.store.UpsertPVEServer(r.Context(), req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID)
	case "pbs":
		if req.PublicIP == "" {
			if sv, getErr := h.store.GetPBSServerByName(r.Context(), req.Hostname); getErr == nil {
				req.PublicIP = sv.PublicIP
			}
		}
		serverID, err = h.store.UpsertPBSServer(r.Context(), req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID)
	}
	if err != nil {
		internalErr(w, "upsert heartbeat server", err)
		return
	}

	if err := h.store.UpsertServerHeartbeat(r.Context(), domain.ServerHeartbeat{
		ServerType:    req.ServerType,
		ServerID:      serverID,
		Hostname:      req.Hostname,
		IP:            req.IPAddress,
		PublicIP:      req.PublicIP,
		ClientVersion: req.ClientVersion,
		MachineID:     req.MachineID,
		LastSeenAt:    time.Now(),
	}); err != nil {
		internalErr(w, "save heartbeat", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "server": req.Hostname, "server_type": req.ServerType})
}

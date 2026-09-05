package handlers

import (
	"net/http"
	"strings"
	"time"

	"probakgo/internal/api/apictx"
	"probakgo/internal/domain"
)

func (h *H) Heartbeat(w http.ResponseWriter, r *http.Request) {
	var req domain.HeartbeatRequest
	if err := decodeJSONBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	req.Hostname = strings.TrimSpace(req.Hostname)
	req.ServerType = strings.ToLower(strings.TrimSpace(req.ServerType))
	if err := req.Validate(); err != nil {
		errJSON(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.ServerType == "" {
		req.ServerType = "pve"
	}
	if req.ServerType != "pve" && req.ServerType != "pbs" && req.ServerType != "windows" {
		errJSON(w, http.StatusBadRequest, "server_type must be pve, pbs or windows")
		return
	}
	if !h.requireKeyServer(w, r, req.Hostname) {
		return
	}
	k, _ := apictx.APIKey(r.Context())
	if req.PublicIP == "" {
		req.PublicIP, _ = h.store.ServerPublicIPForAPIKey(r.Context(), req.ServerType, k.ID)
	}

	var (
		serverID int64
		err      error
	)
	switch req.ServerType {
	case "pve":
		serverID, err = h.store.UpsertPVEServerForAPIKey(r.Context(), k.ID, req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID)
	case "pbs":
		serverID, err = h.store.UpsertPBSServerForAPIKey(r.Context(), k.ID, req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID)
	case "windows":
		serverID, err = h.store.UpsertWindowsServerForAPIKey(r.Context(), k.ID, req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID)
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
		SwapTotal:     req.SwapTotal,
		SwapUsed:      req.SwapUsed,
		SwapEnabled:   req.SwapEnabled != nil && *req.SwapEnabled,
		SwapReported:  req.SwapEnabled != nil,
		LastSeenAt:    time.Now(),
	}); err != nil {
		internalErr(w, "save heartbeat", err)
		return
	}
	h.sendImmediateCriticalAlerts()
	writeJSON(w, http.StatusOK, map[string]any{"status": "ok", "server": req.Hostname, "server_type": req.ServerType})
}

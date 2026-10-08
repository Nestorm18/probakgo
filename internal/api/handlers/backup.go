package handlers

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/api/apictx"
	"probakgo/internal/domain"
)

func (h *H) GetBackupConfig(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	k, ok := apictx.APIKey(r.Context())
	if !ok {
		errJSON(w, http.StatusUnauthorized, "invalid or inactive API key")
		return
	}
	if !keyMayReadServer(w, k, server) {
		return
	}
	// A read must not bind the key or create a PVE server; until the first
	// write or report there is simply no configuration.
	serverID, found, err := h.store.FindPVEServerForAPIKey(r.Context(), k.ID, server, k.MachineID)
	if err != nil {
		internalErr(w, "find pve server", err)
		return
	}
	var configs []domain.VMBackupConfig
	if found {
		configs, err = h.store.ListVMBackupConfigsForServerOrName(r.Context(), "pve", serverID, server)
		if err != nil {
			internalErr(w, "list vm backup configs", err)
			return
		}
	}
	resp := make([]domain.VMBackupConfigResponse, 0, len(configs))
	for _, c := range configs {
		resp = append(resp, toVMConfigResponse(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": server, "configs": resp})
}

func (h *H) UpdateBackupInventory(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	if !h.requireKeyServer(w, r, server) {
		return
	}
	serverID, err := h.pveServerIDForKey(r, server)
	if err != nil {
		internalErr(w, "resolve pve server", err)
		return
	}
	var req struct {
		HasVMs bool `json:"has_vms"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.SetPVEBackupInventory(r.Context(), serverID, req.HasVMs); err != nil {
		internalErr(w, "update pve backup inventory", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *H) CreateVMConfig(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	if !h.requireKeyServer(w, r, server) {
		return
	}
	serverID, err := h.pveServerIDForKey(r, server)
	if err != nil {
		internalErr(w, "resolve pve server", err)
		return
	}
	var req domain.CreateVMBackupConfigRequest
	if err := decodeJSONBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if !validVMID(req.VMID) {
		errJSON(w, http.StatusBadRequest, "vm_id must be a Proxmox VM ID")
		return
	}
	id, err := h.store.CreateVMBackupConfigForServer(r.Context(), "pve", serverID, server, req)
	if err != nil {
		internalErr(w, "create vm backup config", err)
		return
	}
	_ = h.store.SetPVEBackupInventory(r.Context(), serverID, true)
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *H) UpdateVMConfig(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	vmid := chi.URLParam(r, "vmid")
	if !validVMID(vmid) {
		errJSON(w, http.StatusBadRequest, "vmid must be a Proxmox VM ID")
		return
	}
	if !h.requireKeyServer(w, r, server) {
		return
	}
	serverID, err := h.pveServerIDForKey(r, server)
	if err != nil {
		internalErr(w, "resolve pve server", err)
		return
	}
	var req domain.CreateVMBackupConfigRequest
	if err := decodeJSONBody(r, &req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.UpdateVMBackupConfigForServer(r.Context(), "pve", serverID, vmid, req); err != nil {
		internalErr(w, "update vm backup config", err)
		return
	}
	_ = h.store.SetPVEBackupInventory(r.Context(), serverID, true)
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *H) DeleteVMConfig(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	vmid := chi.URLParam(r, "vmid")
	if !validVMID(vmid) {
		errJSON(w, http.StatusBadRequest, "vmid must be a Proxmox VM ID")
		return
	}
	if !h.requireKeyServer(w, r, server) {
		return
	}
	serverID, err := h.pveServerIDForKey(r, server)
	if err != nil {
		internalErr(w, "resolve pve server", err)
		return
	}
	if err := h.store.DeleteVMBackupConfigForServer(r.Context(), "pve", serverID, vmid); err != nil {
		internalErr(w, "delete vm backup config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *H) ToggleVMExclude(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(chi.URLParam(r, "server"))
	vmid := chi.URLParam(r, "vmid")
	if !validVMID(vmid) {
		errJSON(w, http.StatusBadRequest, "vmid must be a Proxmox VM ID")
		return
	}
	if !h.requireKeyServer(w, r, server) {
		return
	}
	serverID, err := h.pveServerIDForKey(r, server)
	if err != nil {
		internalErr(w, "resolve pve server", err)
		return
	}
	if err := h.store.ToggleVMExcludeForServer(r.Context(), "pve", serverID, vmid); err != nil {
		internalErr(w, "toggle vm exclude", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

func (h *H) pveServerIDForKey(r *http.Request, hostname string) (int64, error) {
	k, _ := apictx.APIKey(r.Context())
	return h.store.ResolvePVEServerForAPIKey(r.Context(), k.ID, hostname, k.MachineID)
}

func toVMConfigResponse(c domain.VMBackupConfig) domain.VMBackupConfigResponse {
	return domain.VMBackupConfigResponse{
		ID:         c.ID,
		ServerName: c.ServerName,
		VMID:       c.VMID,
		VMName:     c.VMName,
		Monday:     c.Monday,
		Tuesday:    c.Tuesday,
		Wednesday:  c.Wednesday,
		Thursday:   c.Thursday,
		Friday:     c.Friday,
		Saturday:   c.Saturday,
		Sunday:     c.Sunday,
		IsExcluded: c.IsExcluded,
	}
}

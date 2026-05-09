package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
)

func (h *H) GetBackupConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	configs, err := h.store.ListVMBackupConfigs(r.Context(), server)
	if err != nil {
		internalErr(w, "list vm backup configs", err)
		return
	}
	resp := make([]domain.VMBackupConfigResponse, 0, len(configs))
	for _, c := range configs {
		resp = append(resp, toVMConfigResponse(c))
	}
	writeJSON(w, http.StatusOK, map[string]any{"server": server, "configs": resp})
}

func (h *H) CreateVMConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	var req domain.CreateVMBackupConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.VMID == "" {
		errJSON(w, http.StatusBadRequest, "vm_id is required")
		return
	}
	id, err := h.store.CreateVMBackupConfig(r.Context(), server, req)
	if err != nil {
		internalErr(w, "create vm backup config", err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{"id": id})
}

func (h *H) UpdateVMConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	var req domain.CreateVMBackupConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.UpdateVMBackupConfig(r.Context(), server, vmid, req); err != nil {
		internalErr(w, "update vm backup config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *H) DeleteVMConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.DeleteVMBackupConfig(r.Context(), server, vmid); err != nil {
		internalErr(w, "delete vm backup config", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *H) ToggleVMExclude(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.ToggleVMExclude(r.Context(), server, vmid); err != nil {
		internalErr(w, "toggle vm exclude", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
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

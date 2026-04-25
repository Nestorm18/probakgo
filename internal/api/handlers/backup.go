package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"

	"probaky/internal/domain"
)

func (h *H) GetBackupConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	configs, err := h.store.ListVMBackupConfigs(server)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
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
	id, err := h.store.CreateVMBackupConfig(server, req)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
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
	if err := h.store.UpdateVMBackupConfig(server, vmid, req); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *H) DeleteVMConfig(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.DeleteVMBackupConfig(server, vmid); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *H) ToggleVMExclude(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.ToggleVMExclude(server, vmid); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
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

package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	qrcode "github.com/skip2/go-qrcode"

	"probaky/internal/domain"
	"probaky/internal/service"
)

func (h *H) ListAPIKeys(w http.ResponseWriter, r *http.Request) {
	keys, err := h.store.ListAPIKeys()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]domain.APIKeyResponse, 0, len(keys))
	for _, k := range keys {
		resp = append(resp, domain.APIKeyResponse{
			ID:         k.ID,
			Name:       k.Name,
			KeyPreview: service.KeyPreview(k.Key),
			KeyType:    k.KeyType,
			IsActive:   k.IsActive,
			MachineID:  k.MachineID,
			LastUsed:   k.LastUsed,
			ServerName: k.ServerName,
			CreatedAt:  k.CreatedAt,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{"api_keys": resp})
}

func (h *H) CreateAPIKey(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if req.Name == "" || req.KeyType == "" {
		errJSON(w, http.StatusBadRequest, "name and key_type are required")
		return
	}
	k, err := h.store.CreateAPIKey(req.Name, req.KeyType, req.ServerName)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, domain.CreateAPIKeyResponse{
		ID:   k.ID,
		Key:  k.Key,
		Name: k.Name,
	})
}

func (h *H) UpdateAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req domain.UpdateAPIKeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, http.StatusBadRequest, "invalid JSON")
		return
	}
	if err := h.store.UpdateAPIKey(id, req.Name, req.ServerName); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (h *H) DeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.DeleteAPIKey(id); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

func (h *H) ToggleAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.ToggleAPIKey(id); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "toggled"})
}

func (h *H) UnbindAPIKey(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	if err := h.store.UnbindAPIKeyMachineID(id); err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "unbound"})
}

func (h *H) QRImage(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid id")
		return
	}
	k, err := h.store.GetAPIKey(id)
	if err != nil {
		errJSON(w, http.StatusNotFound, "key not found")
		return
	}
	png, err := qrcode.Encode(k.Key, qrcode.Medium, 256)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, "qr generation failed")
		return
	}
	w.Header().Set("Content-Type", "image/png")
	_, _ = w.Write(png)
}

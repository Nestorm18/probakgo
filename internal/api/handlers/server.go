package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *H) ListPVEServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListPVEServers(r.Context())
	if err != nil {
		internalErr(w, "list pve servers", err)
		return
	}
	resp := make([]any, 0, len(servers))
	for _, sv := range servers {
		resp = append(resp, h.report.BuildPVEServerResponse(r.Context(), sv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": resp})
}

func (h *H) ListPBSServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListPBSServers(r.Context())
	if err != nil {
		internalErr(w, "list pbs servers", err)
		return
	}
	resp := make([]any, 0, len(servers))
	for _, sv := range servers {
		resp = append(resp, h.report.BuildPBSServerResponse(r.Context(), sv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": resp})
}

func (h *H) ListPVEReports(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid server id")
		return
	}
	sv, err := h.store.GetPVEServer(r.Context(), id)
	if err != nil {
		errJSON(w, http.StatusNotFound, "server not found")
		return
	}
	limit := 30
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 && n <= 100 {
			limit = n
		}
	}
	reports, err := h.store.ListPVEReports(r.Context(), id, limit)
	if err != nil {
		internalErr(w, "list pve reports", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server":  h.report.BuildPVEServerResponse(r.Context(), *sv),
		"reports": reports,
	})
}

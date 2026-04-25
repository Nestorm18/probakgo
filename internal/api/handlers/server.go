package handlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (h *H) ListPVEServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListPVEServers()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]any, 0, len(servers))
	for _, sv := range servers {
		resp = append(resp, h.report.BuildPVEServerResponse(sv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": resp})
}

func (h *H) ListPBSServers(w http.ResponseWriter, r *http.Request) {
	servers, err := h.store.ListPBSServers()
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	resp := make([]any, 0, len(servers))
	for _, sv := range servers {
		resp = append(resp, h.report.BuildPBSServerResponse(sv))
	}
	writeJSON(w, http.StatusOK, map[string]any{"servers": resp})
}

func (h *H) ListPVEReports(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		errJSON(w, http.StatusBadRequest, "invalid server id")
		return
	}
	sv, err := h.store.GetPVEServer(id)
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
	reports, err := h.store.ListPVEReports(id, limit)
	if err != nil {
		errJSON(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"server":  h.report.BuildPVEServerResponse(*sv),
		"reports": reports,
	})
}

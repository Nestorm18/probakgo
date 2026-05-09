package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"probakgo/internal/service"
	"probakgo/internal/store"
)

type H struct {
	store  *store.Store
	auth   *service.AuthService
	report *service.ReportService
}

func New(st *store.Store, auth *service.AuthService, rep *service.ReportService) *H {
	return &H{store: st, auth: auth, report: rep}
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func internalErr(w http.ResponseWriter, op string, err error) {
	slog.Error(op, "err", err)
	errJSON(w, http.StatusInternalServerError, "internal server error")
}

func (h *H) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *H) VerifyKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

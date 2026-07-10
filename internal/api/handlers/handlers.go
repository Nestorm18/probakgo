package handlers

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"

	"probakgo/internal/service"
	"probakgo/internal/store"
)

type H struct {
	store      *store.Store
	auth       *service.AuthService
	report     *service.ReportService
	alertQueue chan struct{}
}

func New(st *store.Store, auth *service.AuthService, rep *service.ReportService) *H {
	h := &H{store: st, auth: auth, report: rep, alertQueue: make(chan struct{}, 1)}
	go h.runAlertQueue()
	return h
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

func (h *H) sendImmediateCriticalAlerts() {
	select {
	case h.alertQueue <- struct{}{}:
	default:
	}
}

func (h *H) runAlertQueue() {
	for range h.alertQueue {
		if alerts, err := service.CurrentAlertsRaw(context.Background(), h.store, h.report); err == nil {
			_ = h.store.SyncAlertStates(context.Background(), alerts)
		} else {
			slog.Warn("sync alert states", "err", err)
		}
		if err := service.SendImmediateCriticalAlerts(h.store, h.report); err != nil {
			slog.Warn("send immediate critical alerts", "err", err)
		}
	}
}

func (h *H) Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *H) VerifyKey(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "valid"})
}

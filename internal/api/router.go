package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"probakgo/internal/api/handlers"
	"probakgo/internal/service"
	"probakgo/internal/store"
)

type Server struct {
	auth   *service.AuthService
	report *service.ReportService
	store  *store.Store
}

func NewServer(st *store.Store, auth *service.AuthService, rep *service.ReportService) *Server {
	return &Server{store: st, auth: auth, report: rep}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)

	h := handlers.New(s.store, s.auth, s.report)

	r.Get("/health", h.Health)

	// Server keys (client reports)
	r.With(s.requireServerKey).Post("/report/pve", h.ReportPVE)
	r.With(s.requireServerKey).Post("/report/pbs", h.ReportPBS)

	// Any valid key (read data)
	r.With(s.requireAnyKey).Get("/auth/verify", h.VerifyKey)
	r.With(s.requireAnyKey).Get("/servers/pve", h.ListPVEServers)
	r.With(s.requireAnyKey).Get("/servers/pve/{id}/reports", h.ListPVEReports)
	r.With(s.requireAnyKey).Get("/servers/pbs", h.ListPBSServers)

	// Backup config (server or admin key)
	r.With(s.requireAnyKey).Get("/backup-config/pve/{server}", h.GetBackupConfig)
	r.With(s.requireAnyKey).Post("/backup-config/pve/{server}/vms", h.CreateVMConfig)
	r.With(s.requireAnyKey).Put("/backup-config/pve/{server}/vms/{vmid}", h.UpdateVMConfig)
	r.With(s.requireAnyKey).Delete("/backup-config/pve/{server}/vms/{vmid}", h.DeleteVMConfig)
	r.With(s.requireAnyKey).Put("/backup-config/pve/{server}/vms/{vmid}/toggle-exclude", h.ToggleVMExclude)

	// Admin only
	r.With(s.requireAdminKey).Get("/admin/config-info", h.ConfigInfo)
	r.With(s.requireAdminKey).Post("/admin/api-keys", h.CreateAPIKey)
	r.With(s.requireAdminKey).Get("/admin/api-keys", h.ListAPIKeys)
	r.With(s.requireAdminKey).Put("/admin/api-keys/{id}", h.UpdateAPIKey)
	r.With(s.requireAdminKey).Delete("/admin/api-keys/{id}", h.DeleteAPIKey)
	r.With(s.requireAdminKey).Put("/admin/api-keys/{id}/toggle", h.ToggleAPIKey)
	r.With(s.requireAdminKey).Put("/admin/api-keys/{id}/unbind", h.UnbindAPIKey)
	r.With(s.requireAdminKey).Get("/admin/api-keys/{id}/qr-image", h.QRImage)

	// Client download
	r.Get("/download/latest-metadata", h.DownloadMetadata)
	r.With(s.requireAnyKey).Get("/download/latest", h.DownloadLatest)

	return r
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		slog.Info("api", "method", r.Method, "path", r.URL.Path,
			"status", ww.Status(), "ms", time.Since(start).Milliseconds())
	})
}

func jsonError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func withValue(ctx context.Context, key, val any) context.Context {
	return context.WithValue(ctx, key, val)
}

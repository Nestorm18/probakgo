package api

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"probakgo/internal/api/handlers"
	"probakgo/internal/ratelimit"
	"probakgo/internal/service"
	"probakgo/internal/store"
)

type Server struct {
	auth          *service.AuthService
	report        *service.ReportService
	store         *store.Store
	clientLimiter *ratelimit.Limiter
}

func NewServer(st *store.Store, auth *service.AuthService, rep *service.ReportService) *Server {
	return &Server{
		store:         st,
		auth:          auth,
		report:        rep,
		clientLimiter: ratelimit.New(300, time.Minute),
	}
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(ratelimit.New(120, time.Minute).JSONMiddleware)
	r.Use(requestLogger)
	r.Use(middleware.Recoverer)
	r.Use(apiSecurityHeaders)

	h := handlers.New(s.store, s.auth, s.report)

	r.Get("/health", h.Health)

	// Server keys (client reports)
	r.With(s.requireServerKey).Post("/report/pve", h.ReportPVE)
	r.With(s.requireServerKey).Post("/report/pbs", h.ReportPBS)

	r.With(s.requireServerKey).Get("/auth/verify", h.VerifyKey)
	r.With(s.requireServerKey).Get("/servers/pve", h.ListPVEServers)
	r.With(s.requireServerKey).Get("/servers/pve/{id}/reports", h.ListPVEReports)
	r.With(s.requireServerKey).Get("/servers/pbs", h.ListPBSServers)

	r.With(s.requireServerKey).Get("/backup-config/pve/{server}", h.GetBackupConfig)
	r.With(s.requireServerKey).Post("/backup-config/pve/{server}/vms", h.CreateVMConfig)
	r.With(s.requireServerKey).Put("/backup-config/pve/{server}/vms/{vmid}", h.UpdateVMConfig)
	r.With(s.requireServerKey).Delete("/backup-config/pve/{server}/vms/{vmid}", h.DeleteVMConfig)
	r.With(s.requireServerKey).Put("/backup-config/pve/{server}/vms/{vmid}/toggle-exclude", h.ToggleVMExclude)

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

func apiSecurityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

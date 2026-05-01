package web

import (
	"crypto/sha256"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/gorilla/csrf"

	"probakgo/internal/ratelimit"
	"probakgo/internal/service"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
)

// NewRouter builds the web UI router.
// templateFS is the full embedded FS (paths like web/templates/base.html).
// staticFS is a sub-FS rooted at web/static (served under /static/).
func NewRouter(st *store.Store, rep *service.ReportService, templateFS embed.FS, staticFS fs.FS, sessionKey string, secure bool, trustedOrigins []string) (http.Handler, error) {
	tmpl := webhandlers.NewTemplates(templateFS)
	h := webhandlers.New(st, tmpl, rep)

	// Progressive ban: 3 failures within 30 min → 24h → 7 days → permanent.
	ban := ratelimit.NewBanhammer(3, 30*time.Minute, st,
		24*time.Hour,
		7*24*time.Hour,
		0, // permanent
	)
	if err := ban.Load(); err != nil {
		slog.Warn("failed to load ip bans from db", "err", err)
	}
	h.SetBanhammer(ban)

	r := chi.NewRouter()
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	loginLimiter := ratelimit.New(10, time.Minute)

	r.Get("/login", h.LoginPage)
	r.With(loginLimiter.Middleware).Post("/login", h.LoginPost)
	r.Get("/logout", h.Logout)

	r.Group(func(r chi.Router) {
		r.Use(RequireLogin)

		r.Get("/", h.Dashboard)
		r.Get("/servers/pve", h.PVEServers)
		r.Get("/servers/pve/{id}", h.PVEServerDetail)
		r.Get("/servers/pve/{id}/reports", h.PVEServerReports)
		r.Get("/servers/pbs", h.PBSServers)
		r.Get("/servers/pbs/{id}", h.PBSServerDetail)

		// API keys - list visible to all, writes admin-only, reveal admin-only
		r.Get("/api-keys", h.APIKeys)
		r.With(RequireAdmin).Post("/api-keys", h.CreateAPIKeyPost)
		r.With(RequireAdmin).Get("/api-keys/{id}/edit", h.EditAPIKeyPage)
		r.With(RequireAdmin).Post("/api-keys/{id}/edit", h.EditAPIKeyPost)
		r.With(RequireAdmin).Post("/api-keys/{id}/toggle", h.ToggleAPIKeyPost)
		r.With(RequireAdmin).Post("/api-keys/{id}/delete", h.DeleteAPIKeyPost)
		r.With(RequireAdmin).Post("/api-keys/{id}/unbind", h.UnbindAPIKeyPost)
		r.With(RequireAdmin).Post("/api-keys/{id}/reveal", h.RevealAPIKeyPost)

		// Users - admin only
		r.With(RequireAdmin).Get("/users", h.Users)
		r.With(RequireAdmin).Post("/users", h.CreateUserPost)
		r.With(RequireAdmin).Post("/users/{id}/password", h.ChangePasswordPost)
		r.With(RequireAdmin).Post("/users/{id}/role", h.ChangeRolePost)
		r.With(RequireAdmin).Post("/users/{id}/toggle", h.ToggleUserPost)
		r.With(RequireAdmin).Post("/users/{id}/delete", h.DeleteUserPost)

		r.Get("/profile", h.Profile)
		r.Post("/profile", h.ProfilePost)

		// Backup config - editor + admin
		r.With(RequireEditor).Get("/backup-config/{server}", h.BackupConfig)
		r.With(RequireEditor).Get("/backup-config/{server}/vm/new", h.BackupConfigVMNewPage)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/new", h.BackupConfigVMNewPost)
		r.With(RequireEditor).Get("/backup-config/{server}/vm/{vmid}/edit", h.BackupConfigVMEditPage)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/edit", h.BackupConfigVMEditPost)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/delete", h.BackupConfigVMDelete)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/toggle", h.BackupConfigVMToggle)

		// Settings - admin only
		r.With(RequireAdmin).Get("/settings/email", h.EmailSettings)
		r.With(RequireAdmin).Post("/settings/email", h.EmailSettingsPost)
		r.With(RequireAdmin).Get("/settings/email/test", h.EmailTest)
		r.With(RequireAdmin).Get("/settings/maintenance", h.MaintenanceSettings)
		r.With(RequireAdmin).Post("/settings/maintenance", h.MaintenanceSettingsPost)
		r.With(RequireAdmin).Get("/settings/alerts", h.AlertsSettings)
		r.With(RequireAdmin).Post("/settings/alerts", h.AlertsSettingsPost)
		r.With(RequireAdmin).Get("/settings/ip-bans", h.IPBansPage)
		r.With(RequireAdmin).Post("/settings/ip-bans/unban", h.UnbanIPPost)
	})

	csrfKey := sha256.Sum256([]byte(sessionKey))
	csrfOpts := []csrf.Option{csrf.Secure(secure)}
	if len(trustedOrigins) > 0 {
		csrfOpts = append(csrfOpts, csrf.TrustedOrigins(trustedOrigins))
	}
	return csrf.Protect(csrfKey[:], csrfOpts...)(r), nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

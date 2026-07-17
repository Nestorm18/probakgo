package web

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"probakgo/internal/netutil"
	"probakgo/internal/ratelimit"
	"probakgo/internal/service"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
)

// NewRouter builds the web UI router.
// templateFS is the full embedded FS (paths like web/templates/base.html).
// staticFS is a sub-FS rooted at web/static (served under /static/).
func NewRouter(st *store.Store, rep *service.ReportService, templateFS embed.FS, staticFS fs.FS, sessionKey string, secure bool, trustedOrigins, trustedProxies []string, version string, dev bool, loc *time.Location) (http.Handler, error) {
	tmpl := webhandlers.NewTemplates(templateFS, version, loc, secure, func() (int, int) {
		return service.ActiveAlertCounts(context.Background(), st, rep)
	}, func() (bool, bool) {
		cfg, err := st.GetEmailConfig(context.Background())
		if err != nil || cfg == nil {
			return false, false
		}
		return cfg.SensitiveActionsRequireTOTP, cfg.VPNOnlyAccess
	})
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
	r.Use(netutil.TrustedProxyRealIP(trustedProxies))
	r.Use(middleware.Recoverer)
	r.Use(securityHeaders)
	r.Use(webhandlers.DebugBarMiddleware(dev))

	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	r.Get("/download/client/linux-amd64", h.DownloadClientLinuxAMD64)
	r.Get("/download/client/windows-amd64", h.DownloadClientWindowsAMD64)

	loginLimiter := ratelimit.New(10, time.Minute)
	sensitive := RequireTOTPForSensitiveAction(st)

	r.Get("/login", h.LoginPage)
	r.With(loginLimiter.Middleware).Post("/login", h.LoginPost)
	r.Get("/login/2fa", h.Login2FAPage)
	r.With(loginLimiter.Middleware).Post("/login/2fa", h.Login2FAPost)
	r.Get("/logout", h.Logout)

	r.Group(func(r chi.Router) {
		r.Use(RequireLogin(st))

		r.Get("/", h.Dashboard)
		r.Get("/alerts", h.Alerts)
		r.Get("/alerts/detail", h.AlertDetail)
		r.Get("/alerts.csv", h.AlertsCSV)
		r.Get("/alerts.json", h.AlertsJSON)
		r.Get("/alerts/status.json", h.AlertsStatus)
		r.Post("/alerts/suppress", h.AlertSuppressPost)
		r.Post("/alerts/unsuppress", h.AlertUnsuppressPost)
		r.Get("/servers/pve", h.PVEServers)
		r.Get("/servers/pve.csv", h.PVEServersCSV)
		r.Get("/servers/pve.json", h.PVEServersJSON)
		r.Get("/servers/pve/{id}", h.PVEServerDetail)
		r.Get("/servers/pve/{id}/reports", h.PVEServerReports)
		r.Get("/servers/pve/{id}/reports/csv", h.PVEServerReportsCSV)
		r.Get("/servers/pve/{id}/reports/json", h.PVEServerReportsJSON)
		r.Get("/servers/pbs", h.PBSServers)
		r.Get("/servers/pbs.csv", h.PBSServersCSV)
		r.Get("/servers/pbs.json", h.PBSServersJSON)
		r.Get("/servers/pbs/{id}", h.PBSServerDetail)
		r.Get("/servers/pbs/{id}/reports/csv", h.PBSServerReportsCSV)
		r.Get("/servers/pbs/{id}/reports/json", h.PBSServerReportsJSON)
		r.Get("/servers/windows", h.WindowsServers)
		r.Get("/servers/windows/{id}", h.WindowsServerDetail)
		r.With(RequireEditor, sensitive).Post("/servers/pve/{id}/alerts", h.PVEAlertConfigPost)
		r.With(RequireEditor, sensitive).Post("/servers/pve/{id}/alerts/vm", h.PVEVMAlertConfigPost)
		r.With(RequireEditor, sensitive).Post("/servers/pbs/{id}/alerts", h.PBSAlertConfigPost)
		r.With(RequireEditor, sensitive).Post("/servers/windows/{id}/alerts", h.WindowsAlertConfigPost)

		// API keys - list visible to all, writes admin-only, reveal admin-only
		r.Get("/api-keys", h.APIKeys)
		r.With(RequireAdmin).Get("/api-keys/new", h.NewAPIKeyPage)
		r.With(RequireAdmin, sensitive).Post("/api-keys", h.CreateAPIKeyPost)
		r.With(RequireAdmin).Get("/api-keys/{id}/edit", h.EditAPIKeyPage)
		r.With(RequireAdmin, sensitive).Post("/api-keys/{id}/edit", h.EditAPIKeyPost)
		r.With(RequireAdmin, sensitive).Post("/api-keys/{id}/toggle", h.ToggleAPIKeyPost)
		r.With(RequireAdmin, sensitive).Post("/api-keys/{id}/delete", h.DeleteAPIKeyPost)
		r.With(RequireAdmin, sensitive).Post("/api-keys/{id}/unbind", h.UnbindAPIKeyPost)
		r.With(RequireAdmin, sensitive).Post("/api-keys/{id}/reveal", h.RevealAPIKeyPost)

		// Users - admin only
		r.With(RequireAdmin).Get("/users", h.Users)
		r.With(RequireAdmin).Get("/users/new", h.UserNewPage)
		r.With(RequireAdmin).Get("/users/{id}/edit", h.UserEditPage)
		r.With(RequireAdmin, sensitive).Post("/users", h.CreateUserPost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/username", h.ChangeUsernamePost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/password", h.ChangePasswordPost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/2fa/disable", h.DisableUser2FAPost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/role", h.ChangeRolePost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/toggle", h.ToggleUserPost)
		r.With(RequireAdmin, sensitive).Post("/users/{id}/delete", h.DeleteUserPost)

		r.Get("/profile", h.Profile)
		r.With(sensitive).Post("/profile", h.ProfilePost)
		r.Post("/profile/2fa/setup", h.Profile2FASetup)
		r.Post("/profile/2fa/confirm", h.Profile2FAConfirm)
		r.With(sensitive).Post("/profile/2fa/disable", h.Profile2FADisable)

		// Backup config - editor + admin
		r.With(RequireEditor).Get("/backup-config/{server}", h.BackupConfig)
		r.With(RequireEditor).Get("/backup-config/{server}/vm/new", h.BackupConfigVMNewPage)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/new", h.BackupConfigVMNewPost)
		r.With(RequireEditor).Get("/backup-config/{server}/vm/{vmid}/edit", h.BackupConfigVMEditPage)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/edit", h.BackupConfigVMEditPost)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/delete", h.BackupConfigVMDelete)
		r.With(RequireEditor).Post("/backup-config/{server}/vm/{vmid}/toggle", h.BackupConfigVMToggle)

		// Settings - admin only
		r.With(RequireAdmin).Get("/settings", h.SettingsHub)
		r.With(RequireAdmin).Get("/settings/system", h.SystemSettings)
		r.With(RequireAdmin, sensitive).Post("/settings/system", h.SystemSettingsPost)
		r.With(RequireAdmin, sensitive).Post("/settings/system/session-secure", h.EnableSessionSecurePost)
		r.With(RequireAdmin).Get("/settings/email", h.EmailSettings)
		r.With(RequireAdmin, sensitive).Post("/settings/email", h.EmailSettingsPost)
		r.With(RequireAdmin).Post("/settings/email/test", h.EmailTest)
		r.With(RequireAdmin).Get("/settings/maintenance", h.MaintenanceSettings)
		r.With(RequireAdmin, sensitive).Post("/settings/maintenance", h.MaintenanceSettingsPost)
		r.With(RequireAdmin, sensitive).Post("/settings/maintenance/database/download", h.MaintenanceDatabaseDownload)
		r.With(RequireAdmin).Get("/settings/alerts", h.AlertsSettings)
		r.With(RequireAdmin, sensitive).Post("/settings/alerts", h.AlertsSettingsPost)
		r.With(RequireAdmin).Get("/settings/ip-bans", h.IPBansPage)
		r.With(RequireAdmin, sensitive).Post("/settings/ip-bans/unban", h.UnbanIPPost)
		r.With(RequireAdmin).Get("/settings/audit-log", h.AuditLogPage)
		r.With(RequireAdmin).Get("/settings/reset", h.ResetSettings)
		r.With(RequireAdmin, sensitive).Post("/settings/reset", h.ResetDatabasePost)

		r.With(RequireAdmin).Get("/about", h.About)
		r.With(RequireAdmin).Post("/about/update", h.AboutUpdatePost)
	})

	protection, err := newCrossOriginProtection(trustedOrigins)
	if err != nil {
		return nil, err
	}
	return protection.Handler(r), nil
}

func newCrossOriginProtection(trustedOrigins []string) (*http.CrossOriginProtection, error) {
	protection := http.NewCrossOriginProtection()
	for _, origin := range trustedOrigins {
		if err := protection.AddTrustedOrigin(origin); err != nil {
			return nil, err
		}
	}
	return protection, nil
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Frame-Options", "SAMEORIGIN")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		w.Header().Set("Content-Security-Policy",
			"default-src 'none'; "+
				"script-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net; "+
				"style-src 'self' 'unsafe-inline' https://cdn.jsdelivr.net https://fonts.googleapis.com; "+
				"font-src 'self' https://cdn.jsdelivr.net https://fonts.gstatic.com; "+
				"img-src 'self' data:; "+
				"connect-src 'self' https://cdn.jsdelivr.net; "+
				"frame-ancestors 'self'; "+
				"base-uri 'self'; "+
				"form-action 'self';")
		next.ServeHTTP(w, r)
	})
}

package webhandlers

import (
	"net/http"
	"strconv"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

func (h *WebH) SettingsHub(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	cfg, _ := h.store.GetEmailConfig(ctx)
	banCount := 0
	if h.ban != nil {
		banCount = len(h.ban.ListBanned())
	}
	h.tmpl.Render(w, r, "settings_hub.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Config":   cfg,
		"BanCount": banCount,
	})
}

func (h *WebH) EmailSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	cfg, err := h.store.GetEmailConfig(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "email_settings.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Config":   cfg,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) EmailSettingsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, _ := h.store.GetEmailConfig(ctx)

	port, _ := strconv.Atoi(r.FormValue("smtp_port"))
	if port == 0 {
		port = 587
	}
	sendTime := r.FormValue("send_time")
	if sendTime == "" {
		sendTime = "08:00"
	}
	pass := r.FormValue("smtp_pass")
	if pass == "" && existing != nil {
		pass = existing.SMTPPass
	}

	cfg := domain.EmailConfig{
		SMTPHost:   r.FormValue("smtp_host"),
		SMTPPort:   port,
		SMTPUser:   r.FormValue("smtp_user"),
		SMTPPass:   pass,
		Recipients: r.FormValue("recipients"),
		IsEnabled:  r.FormValue("is_enabled") == "on",
		SendTime:   sendTime,
	}
	if existing != nil {
		cfg.RetentionMonths = existing.RetentionMonths
		cfg.RetentionEnabled = existing.RetentionEnabled
		cfg.AlertDiskPct = existing.AlertDiskPct
		cfg.AlertBackupErr = existing.AlertBackupErr
		cfg.AlertPBSStaleHours = existing.AlertPBSStaleHours
	}
	if err := h.store.UpsertEmailConfig(ctx, cfg); err != nil {
		http.Redirect(w, r, "/settings/email?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/email?flash=Configuracion+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) MaintenanceSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	cfg, err := h.store.GetEmailConfig(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "maintenance_settings.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Config":   cfg,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) MaintenanceSettingsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, _ := h.store.GetEmailConfig(ctx)

	retMonths, _ := strconv.Atoi(r.FormValue("retention_months"))
	if retMonths < 1 {
		retMonths = 1
	}
	if retMonths > 60 {
		retMonths = 60
	}

	cfg := domain.EmailConfig{
		RetentionMonths:  retMonths,
		RetentionEnabled: r.FormValue("retention_enabled") == "on",
	}
	if existing != nil {
		cfg.SMTPHost = existing.SMTPHost
		cfg.SMTPPort = existing.SMTPPort
		cfg.SMTPUser = existing.SMTPUser
		cfg.SMTPPass = existing.SMTPPass
		cfg.Recipients = existing.Recipients
		cfg.IsEnabled = existing.IsEnabled
		cfg.SendTime = existing.SendTime
		cfg.AlertDiskPct = existing.AlertDiskPct
		cfg.AlertBackupErr = existing.AlertBackupErr
		cfg.AlertPBSStaleHours = existing.AlertPBSStaleHours
	}
	if err := h.store.UpsertEmailConfig(ctx, cfg); err != nil {
		http.Redirect(w, r, "/settings/maintenance?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/maintenance?flash=Configuracion+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) AlertsSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	cfg, err := h.store.GetEmailConfig(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "alerts_settings.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Config":   cfg,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) AlertsSettingsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, _ := h.store.GetEmailConfig(ctx)

	alertDisk, _ := strconv.Atoi(r.FormValue("alert_disk_pct"))
	if alertDisk < 0 || alertDisk > 99 {
		alertDisk = 0
	}
	pbsStaleHours, _ := strconv.Atoi(r.FormValue("alert_pbs_stale_hours"))
	if pbsStaleHours < 0 {
		pbsStaleHours = 0
	}

	cfg := domain.EmailConfig{
		AlertDiskPct:       alertDisk,
		AlertBackupErr:     r.FormValue("alert_backup_err") == "on",
		AlertPBSStaleHours: pbsStaleHours,
	}
	if existing != nil {
		cfg.SMTPHost = existing.SMTPHost
		cfg.SMTPPort = existing.SMTPPort
		cfg.SMTPUser = existing.SMTPUser
		cfg.SMTPPass = existing.SMTPPass
		cfg.Recipients = existing.Recipients
		cfg.IsEnabled = existing.IsEnabled
		cfg.SendTime = existing.SendTime
		cfg.RetentionMonths = existing.RetentionMonths
		cfg.RetentionEnabled = existing.RetentionEnabled
	}
	if err := h.store.UpsertEmailConfig(ctx, cfg); err != nil {
		http.Redirect(w, r, "/settings/alerts?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/alerts?flash=Configuracion+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) ResetSettings(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	h.tmpl.Render(w, r, "reset_settings.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) ResetDatabasePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)

	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		http.Redirect(w, r, "/settings/reset?flash=Error+al+obtener+usuario", http.StatusSeeOther)
		return
	}

	pass := r.FormValue("password")
	passConfirm := r.FormValue("password_confirm")

	if pass != passConfirm {
		http.Redirect(w, r, "/settings/reset?flash=Las+contrasenas+no+coinciden", http.StatusSeeOther)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(pass)) != nil {
		http.Redirect(w, r, "/settings/reset?flash=Contrasena+incorrecta", http.StatusSeeOther)
		return
	}

	if err := h.store.ResetAllData(ctx); err != nil {
		http.Redirect(w, r, "/settings/reset?flash=Error:+"+err.Error(), http.StatusSeeOther)
		return
	}

	if h.ban != nil {
		_ = h.ban.Load()
	}

	http.Redirect(w, r, "/settings/reset?flash=Base+de+datos+reiniciada+correctamente&ok=1", http.StatusSeeOther)
}

func (h *WebH) EmailTest(w http.ResponseWriter, r *http.Request) {
	rep := h.report
	if rep == nil {
		http.Redirect(w, r, "/settings/email?flash=Servicio+no+disponible", http.StatusSeeOther)
		return
	}
	if err := service.SendDailyReport(h.store, rep); err != nil {
		http.Redirect(w, r, "/settings/email?flash=Error:+"+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/email?flash=Email+de+prueba+enviado&ok=1", http.StatusSeeOther)
}

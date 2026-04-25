package webhandlers

import (
	"net/http"
	"strconv"

	"probaky/internal/domain"
	"probaky/internal/service"
	"probaky/internal/session"
)

func (h *WebH) EmailSettings(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	cfg, err := h.store.GetEmailConfig()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, "email_settings.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Config":   cfg,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) EmailSettingsPost(w http.ResponseWriter, r *http.Request) {
	port, _ := strconv.Atoi(r.FormValue("smtp_port"))
	if port == 0 {
		port = 587
	}
	sendTime := r.FormValue("send_time")
	if sendTime == "" {
		sendTime = "08:00"
	}

	// Keep existing password if form field is blank
	existing, _ := h.store.GetEmailConfig()
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
	if err := h.store.UpsertEmailConfig(cfg); err != nil {
		http.Redirect(w, r, "/settings/email?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/settings/email?flash=Configuracion+guardada&ok=1", http.StatusSeeOther)
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

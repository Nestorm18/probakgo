package webhandlers

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

// appTimezone is the TIMEZONE the schedulers use, which may differ from the
// process timezone.
func (h *WebH) appTimezone() string {
	if h.tmpl != nil && h.tmpl.loc != nil {
		return h.tmpl.loc.String()
	}
	return time.Local.String()
}

func (h *WebH) NASBackupNow(w http.ResponseWriter, r *http.Request) {
	q := url.Values{}
	if err := service.StartManualNASBackup(r.Context(), h.store); err != nil {
		q.Set("flash", err.Error())
	} else {
		h.audit(r, "settings.nas_backup", "settings", "maintenance", "Copia manual NAS", nil)
		q.Set("flash", "Copia al NAS iniciada. Actualiza la página para consultar el resultado.")
		q.Set("ok", "1")
	}
	http.Redirect(w, r, "/settings/maintenance?"+q.Encode(), http.StatusSeeOther)
}

func (h *WebH) NASBackupSettingsPost(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "formulario inválido", http.StatusBadRequest)
		return
	}
	redirect := func(message string, ok bool) {
		q := url.Values{"flash": {message}}
		if ok {
			q.Set("ok", "1")
		}
		http.Redirect(w, r, "/settings/maintenance?"+q.Encode(), http.StatusSeeOther)
	}
	existing, err := h.store.GetNASBackupConfig(r.Context())
	if err != nil {
		http.Error(w, "error cargando configuración NAS", http.StatusInternalServerError)
		return
	}
	port, _ := strconv.Atoi(r.FormValue("nas_port"))
	c := domain.NASBackupConfig{
		Enabled: r.FormValue("nas_enabled") == "on", Host: strings.TrimSpace(r.FormValue("nas_host")), Port: port,
		Username: strings.TrimSpace(r.FormValue("nas_username")), Password: r.FormValue("nas_password"),
		Directory: strings.TrimSpace(r.FormValue("nas_directory")), SendTime: r.FormValue("nas_time"),
	}
	// The saved password only applies to the saved destination; reusing it for a
	// new host would send the credential (and the backup) somewhere else.
	destinationChanged := c.Host != existing.Host || c.Port != existing.Port || c.Username != existing.Username
	keepPassword := c.Password == "" && existing.Password != "" && !destinationChanged
	if keepPassword {
		c.Password = existing.Password
	}
	render := func(message string, ok bool) {
		cfg, err := h.store.GetEmailConfig(r.Context())
		if err != nil {
			http.Error(w, "error cargando configuración", http.StatusInternalServerError)
			return
		}
		username, role, _ := session.GetUser(r)
		view := c
		view.Password = ""
		view.LastAttempt, view.LastSuccess, view.LastError = existing.LastAttempt, existing.LastSuccess, existing.LastError
		if r.FormValue("nas_password") != "" {
			message += " Vuelve a introducir la contraseña nueva antes de guardar; no se devuelve al navegador."
		}
		h.tmpl.Render(w, r, "maintenance_settings.html", map[string]any{
			"Username": username, "Role": role, "Config": cfg, "NAS": &view,
			"HasNASPassword": existing.Password != "", "ServerTimezone": h.appTimezone(), "Flash": message, "FlashOK": ok,
		})
	}
	action := r.FormValue("action")
	if action != "save" && action != "test" {
		http.Error(w, "acción inválida", http.StatusBadRequest)
		return
	}
	if c.Password == "" && existing.Password != "" && destinationChanged && (c.Enabled || action == "test") {
		render("Introduce la contraseña SFTP al cambiar el servidor, el puerto o el usuario del NAS.", false)
		return
	}
	if c.Enabled || action == "test" {
		if err := service.ValidateNASBackupConfig(c); err != nil {
			render(err.Error(), false)
			return
		}
	}
	if action == "test" {
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if err := service.TestNASBackup(ctx, c); err != nil {
			render(err.Error(), false)
			return
		}
		h.audit(r, "settings.nas_test", "settings", "maintenance", "Prueba NAS", nil)
		render("Conexión y permisos correctos. La prueba no guarda cambios; guarda la configuración para activar las copias.", true)
		return
	}
	if err := h.store.SaveNASBackupConfig(r.Context(), c); err != nil {
		redirect("No se pudo guardar la configuración NAS. Comprueba DATA_ENCRYPTION_KEY en el servidor.", false)
		return
	}
	h.audit(r, "settings.nas_update", "settings", "maintenance", "Copias NAS", map[string]any{"enabled": c.Enabled, "time": c.SendTime})
	redirect("Configuración de copias NAS guardada", true)
}

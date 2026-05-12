package webhandlers

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) BackupConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	configs, err := h.store.ListVMBackupConfigs(ctx, server)
	if err != nil {
		slog.Error("list vm backup configs", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "backup_config.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Configs":    configs,
		"Flash":      r.URL.Query().Get("flash"),
		"FlashOK":    r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) BackupConfigVMNewPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")

	var vm *domain.VMBackupConfig
	if copyID := r.URL.Query().Get("copy"); copyID != "" {
		configs, _ := h.store.ListVMBackupConfigs(ctx, server)
		for i, c := range configs {
			if c.VMID == copyID {
				clone := configs[i]
				clone.VMID = ""
				vm = &clone
				break
			}
		}
	}

	h.tmpl.Render(w, r, "vm_backup_config_form.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Action":     "new",
		"VM":         vm,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMNewPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	req := collectVMFormRequest(r)
	if req.VMID == "" {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash=VM+ID+requerido", http.StatusSeeOther)
		return
	}
	if _, err := h.store.CreateVMBackupConfig(ctx, server, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+creada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMEditPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	configs, _ := h.store.ListVMBackupConfigs(ctx, server)
	var vm *domain.VMBackupConfig
	for i, c := range configs {
		if c.VMID == vmid {
			vm = &configs[i]
			break
		}
	}
	h.tmpl.Render(w, r, "vm_backup_config_form.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Action":     "edit",
		"VM":         vm,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMEditPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	req := collectVMFormRequest(r)
	req.VMID = vmid
	if err := h.store.UpdateVMBackupConfig(ctx, server, vmid, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/"+vmid+"/edit?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.DeleteVMBackupConfig(ctx, server, vmid); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+eliminada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMToggle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.ToggleVMExclude(ctx, server, vmid); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=Estado+cambiado&ok=1", http.StatusSeeOther)
}

func collectVMFormRequest(r *http.Request) domain.CreateVMBackupConfigRequest {
	return domain.CreateVMBackupConfigRequest{
		VMID:      r.FormValue("vm_id"),
		VMName:    r.FormValue("vm_name"),
		Monday:    r.FormValue("monday") == "on",
		Tuesday:   r.FormValue("tuesday") == "on",
		Wednesday: r.FormValue("wednesday") == "on",
		Thursday:  r.FormValue("thursday") == "on",
		Friday:    r.FormValue("friday") == "on",
		Saturday:  r.FormValue("saturday") == "on",
		Sunday:    r.FormValue("sunday") == "on",
	}
}

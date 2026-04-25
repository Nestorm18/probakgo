package webhandlers

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) BackupConfig(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	configs, err := h.store.ListVMBackupConfigs(server)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, "backup_config.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Configs":    configs,
		"Flash":      r.URL.Query().Get("flash"),
		"FlashOK":    r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) BackupConfigVMNewPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	h.tmpl.Render(w, "vm_backup_config_form.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Action":     "new",
		"VM":         nil,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMNewPost(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	req := collectVMFormRequest(r)
	if req.VMID == "" {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash=VM+ID+requerido", http.StatusSeeOther)
		return
	}
	if _, err := h.store.CreateVMBackupConfig(server, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+creada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMEditPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	configs, _ := h.store.ListVMBackupConfigs(server)
	var vm *domain.VMBackupConfig
	for i, c := range configs {
		if c.VMID == vmid {
			vm = &configs[i]
			break
		}
	}
	h.tmpl.Render(w, "vm_backup_config_form.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": server,
		"Action":     "edit",
		"VM":         vm,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMEditPost(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	req := collectVMFormRequest(r)
	req.VMID = vmid
	if err := h.store.UpdateVMBackupConfig(server, vmid, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/"+vmid+"/edit?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMDelete(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.DeleteVMBackupConfig(server, vmid); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+eliminada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMToggle(w http.ResponseWriter, r *http.Request) {
	server := chi.URLParam(r, "server")
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.ToggleVMExclude(server, vmid); err != nil {
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

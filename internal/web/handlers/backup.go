package webhandlers

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) BackupConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	serverID, hostname, displayName := h.resolvePVEBackupServer(ctx, server)
	configs, err := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", serverID, hostname)
	if err != nil {
		slog.Error("list vm backup configs", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "backup_config.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"ServerName": displayName,
		"ServerRef":  server,
		"Configs":    configs,
		"Flash":      r.URL.Query().Get("flash"),
		"FlashOK":    r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) BackupConfigVMNewPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	serverID, hostname, displayName := h.resolvePVEBackupServer(ctx, server)

	var vm *domain.VMBackupConfig
	if copyID := r.URL.Query().Get("copy"); copyID != "" {
		configs, _ := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", serverID, hostname)
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
		"ServerName": displayName,
		"ServerRef":  server,
		"Action":     "new",
		"VM":         vm,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMNewPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	serverID, hostname, _ := h.resolvePVEBackupServer(ctx, server)
	req := collectVMFormRequest(r)
	if req.VMID == "" {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash=VM+ID+requerido", http.StatusSeeOther)
		return
	}
	if _, err := h.store.CreateVMBackupConfigForServer(ctx, "pve", serverID, hostname, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/new?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+creada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMEditPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	server := chi.URLParam(r, "server")
	serverID, hostname, displayName := h.resolvePVEBackupServer(ctx, server)
	vmid := chi.URLParam(r, "vmid")
	configs, _ := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", serverID, hostname)
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
		"ServerName": displayName,
		"ServerRef":  server,
		"Action":     "edit",
		"VM":         vm,
		"Flash":      r.URL.Query().Get("flash"),
	})
}

func (h *WebH) BackupConfigVMEditPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	serverID, _, _ := h.resolvePVEBackupServer(ctx, server)
	vmid := chi.URLParam(r, "vmid")
	req := collectVMFormRequest(r)
	req.VMID = vmid
	if err := h.store.UpdateVMBackupConfigForServer(ctx, "pve", serverID, vmid, req); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"/vm/"+vmid+"/edit?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	serverID, _, _ := h.resolvePVEBackupServer(ctx, server)
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.DeleteVMBackupConfigForServer(ctx, "pve", serverID, vmid); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=VM+eliminada&ok=1", http.StatusSeeOther)
}

func (h *WebH) BackupConfigVMToggle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	server := chi.URLParam(r, "server")
	serverID, _, _ := h.resolvePVEBackupServer(ctx, server)
	vmid := chi.URLParam(r, "vmid")
	if err := h.store.ToggleVMExcludeForServer(ctx, "pve", serverID, vmid); err != nil {
		http.Redirect(w, r, "/backup-config/"+server+"?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/backup-config/"+server+"?flash=Estado+cambiado&ok=1", http.StatusSeeOther)
}

func (h *WebH) resolvePVEBackupServer(ctx context.Context, ref string) (int64, string, string) {
	if id, err := strconv.ParseInt(ref, 10, 64); err == nil && id > 0 {
		if sv, err := h.store.GetPVEServer(ctx, id); err == nil {
			return sv.ID, sv.Name, sv.DisplayName
		}
		return id, ref, ref
	}
	if sv, err := h.store.GetPVEServerByName(ctx, ref); err == nil {
		return sv.ID, sv.Name, sv.DisplayName
	}
	return 0, ref, ref
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

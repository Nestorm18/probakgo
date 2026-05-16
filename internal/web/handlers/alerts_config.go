package webhandlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
)

func (h *WebH) PVEAlertConfigPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	cfg := domain.PVEAlertConfig{ServerID: id}
	if v := r.FormValue("disk_pct"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DiskPct = &n
		}
	}
	if v := r.FormValue("stale_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.StaleHours = &n
		}
	}
	if v := r.FormValue("backup_err"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.BackupErr = &n
		}
	}

	if err := h.store.UpsertPVEAlertConfig(ctx, cfg); err != nil {
		slog.Error("upsert pve alert config", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.audit(r, "alert_config.pve_update", "pve_server", strconv.FormatInt(id, 10), "", map[string]any{
		"disk_pct":    cfg.DiskPct,
		"stale_hours": cfg.StaleHours,
		"backup_err":  cfg.BackupErr,
	})
	if r.FormValue("back") == "list" {
		http.Redirect(w, r, "/servers/pve?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/servers/pve/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) PVEVMAlertConfigPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	vmid, err := strconv.ParseInt(r.FormValue("vmid"), 10, 64)
	if err != nil {
		http.Error(w, "invalid vmid", http.StatusBadRequest)
		return
	}

	backupErrStr := r.FormValue("backup_err")
	minSizeStr := r.FormValue("min_size_mb")

	if backupErrStr == "" && minSizeStr == "" {
		_ = h.store.DeletePVEVMAlertConfig(ctx, id, vmid)
		h.audit(r, "alert_config.pve_vm_delete", "pve_vm", strconv.FormatInt(vmid, 10), "", map[string]any{"server_id": id})
	} else {
		cfg := domain.PVEVMAlertConfig{ServerID: id, VMID: vmid}
		if backupErrStr != "" {
			if n, err2 := strconv.Atoi(backupErrStr); err2 == nil {
				cfg.BackupErr = &n
			}
		}
		if minSizeStr != "" {
			if n, err2 := strconv.Atoi(minSizeStr); err2 == nil {
				cfg.MinSizeMB = &n
			}
		}
		if err := h.store.UpsertPVEVMAlertConfig(ctx, cfg); err != nil {
			slog.Error("upsert pve vm alert config", "err", err)
			http.Error(w, "error interno del servidor", http.StatusInternalServerError)
			return
		}
		h.audit(r, "alert_config.pve_vm_update", "pve_vm", strconv.FormatInt(vmid, 10), "", map[string]any{
			"server_id":   id,
			"backup_err":  cfg.BackupErr,
			"min_size_mb": cfg.MinSizeMB,
		})
	}
	http.Redirect(w, r, "/servers/pve/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+VM+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) PBSAlertConfigPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}

	cfg := domain.PBSAlertConfig{
		ServerID:    id,
		VerifyAlert: r.FormValue("verify_alert") == "on",
	}
	if v := r.FormValue("disk_pct"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DiskPct = &n
		}
	}
	if v := r.FormValue("days_until_full"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.DaysUntilFull = &n
		}
	}
	if v := r.FormValue("stale_hours"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.StaleHours = &n
		}
	}

	if err := h.store.UpsertPBSAlertConfig(ctx, cfg); err != nil {
		slog.Error("upsert pbs alert config", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.audit(r, "alert_config.pbs_update", "pbs_server", strconv.FormatInt(id, 10), "", map[string]any{
		"disk_pct":        cfg.DiskPct,
		"days_until_full": cfg.DaysUntilFull,
		"stale_hours":     cfg.StaleHours,
		"verify_alert":    cfg.VerifyAlert,
	})
	if r.FormValue("back") == "list" {
		http.Redirect(w, r, "/servers/pbs?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/servers/pbs/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
}

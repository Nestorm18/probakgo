package webhandlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
)

func (h *WebH) PVEAlertConfigPost(w http.ResponseWriter, r *http.Request) {
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

	if err := h.store.UpsertPVEAlertConfig(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.FormValue("back") == "list" {
		http.Redirect(w, r, "/servers/pve?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/servers/pve/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) PVEVMAlertConfigPost(w http.ResponseWriter, r *http.Request) {
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
		_ = h.store.DeletePVEVMAlertConfig(id, vmid)
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
		if err := h.store.UpsertPVEVMAlertConfig(cfg); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}
	http.Redirect(w, r, "/servers/pve/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+VM+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) PBSAlertConfigPost(w http.ResponseWriter, r *http.Request) {
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

	if err := h.store.UpsertPBSAlertConfig(cfg); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if r.FormValue("back") == "list" {
		http.Redirect(w, r, "/servers/pbs?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/servers/pbs/"+strconv.FormatInt(id, 10)+"?flash=Configuración+de+alertas+guardada&ok=1", http.StatusSeeOther)
}

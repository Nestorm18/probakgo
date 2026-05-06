package webhandlers

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) Dashboard(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)

	pveServers, err := h.store.ListPVEServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	pbsServers, err := h.store.ListPBSServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var pveOK, pveStale int
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(sv.ID)
		if err != nil || rep.IsStale {
			pveStale++
		} else {
			pveOK++
		}
	}
	var pbsOK, pbsStale int
	for _, sv := range pbsServers {
		rep, err := h.store.GetLatestPBSReport(sv.ID)
		if err != nil || rep.IsStale {
			pbsStale++
		} else {
			pbsOK++
		}
	}

	var alerts []domain.Alert
	if cfg, err := h.store.GetEmailConfig(); err == nil {
		alerts, _ = h.store.GetAlerts(cfg.AlertDiskPct, cfg.AlertBackupErr)
		if cfg.AlertPBSStaleHours > 0 {
			pbsStale, _ := h.store.GetPBSStaleAlerts(cfg.AlertPBSStaleHours)
			alerts = append(alerts, pbsStale...)
		}
	}

	// Task-level alerts: ERROR and MISSING VMs from latest PVE backup job
	for _, sv := range pveServers {
		rep, err := h.store.GetLatestPVEReport(sv.ID)
		if err != nil {
			continue
		}
		tasks, _ := h.store.GetPVEBackupTasksForReport(rep.ID)

		var errorNames []string
		for _, t := range tasks {
			if t.Status != "OK" {
				name := t.VMName
				if name == "" {
					name = strconv.FormatInt(t.VMID, 10)
				}
				errorNames = append(errorNames, name)
			}
		}
		if len(errorNames) > 0 {
			alerts = append(alerts, domain.Alert{
				ServerName: sv.Name,
				Type:       "task_error",
				Message:    fmt.Sprintf("%d VM(s) con error: %s", len(errorNames), strings.Join(errorNames, ", ")),
			})
		}

		if len(tasks) > 0 {
			configs, _ := h.store.ListVMBackupConfigs(sv.Name)
			if len(configs) > 0 {
				jobDay := time.Unix(tasks[0].StartTime, 0).Weekday()
				seenVMIDs := make(map[string]bool)
				for _, t := range tasks {
					seenVMIDs[strconv.FormatInt(t.VMID, 10)] = true
				}
				var missingNames []string
				for _, c := range configs {
					if c.IsExcluded || !vmScheduledForDay(c, jobDay) || seenVMIDs[c.VMID] {
						continue
					}
					name := c.VMName
					if name == "" {
						name = c.VMID
					}
					missingNames = append(missingNames, name)
				}
				if len(missingNames) > 0 {
					alerts = append(alerts, domain.Alert{
						ServerName: sv.Name,
						Type:       "task_missing",
						Message:    fmt.Sprintf("%d VM(s) sin backup: %s", len(missingNames), strings.Join(missingNames, ", ")),
					})
				}
			}
		}
	}

	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":   username,
		"Role":       role,
		"PVEServers": pveServers,
		"PBSServers": pbsServers,
		"PVEOk":      pveOK,
		"PVEStale":   pveStale,
		"PBSOk":      pbsOK,
		"PBSStale":   pbsStale,
		"Alerts":     alerts,
	})
}

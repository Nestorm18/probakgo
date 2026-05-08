package service

import (
	"fmt"
	"log"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

// AlertConfigs holds resolved thresholds for all servers.
// Global values from email_config act as fallback when a server has no per-server config.
type AlertConfigs struct {
	GlobalDiskPct    int
	GlobalStaleHours int
	GlobalBackupErr  bool

	PVEConfigs   map[int64]domain.PVEAlertConfig
	PVEVMConfigs map[int64][]domain.PVEVMAlertConfig // server_id → vm overrides
	PBSConfigs   map[int64]domain.PBSAlertConfig
}

// AlertEvaluator is the function signature every alert type must implement.
// Adding a new alert type = writing a function with this signature and registering it below.
type AlertEvaluator func(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error)

// evaluators is the registry of active alert types.
var evaluators = []AlertEvaluator{
	evalPVEDisk,
	evalPVEBackupErrors,
	evalPVEBackupSize,
	evalPVEStale,
	evalPBSDisk,
	evalPBSFill,
	evalPBSStale,
	evalPBSVerify,
}

// RunAll executes all registered evaluators. Individual errors are logged but do not
// stop execution — partial alerts are better than none.
func RunAll(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	var all []domain.Alert
	for _, eval := range evaluators {
		alerts, err := eval(st, cfg)
		if err != nil {
			log.Printf("alert evaluator error: %v", err)
			continue
		}
		all = append(all, alerts...)
	}
	return all, nil
}

// LoadAlertConfigs builds AlertConfigs from the store using email_config as global fallback.
func LoadAlertConfigs(st *store.Store) (AlertConfigs, error) {
	emailCfg, err := st.GetEmailConfig()
	if err != nil {
		return AlertConfigs{}, err
	}
	cfg := AlertConfigs{
		GlobalDiskPct:    emailCfg.AlertDiskPct,
		GlobalBackupErr:  emailCfg.AlertBackupErr,
		GlobalStaleHours: emailCfg.AlertPBSStaleHours,
		PVEConfigs:       make(map[int64]domain.PVEAlertConfig),
		PVEVMConfigs:     make(map[int64][]domain.PVEVMAlertConfig),
		PBSConfigs:       make(map[int64]domain.PBSAlertConfig),
	}

	pveServers, err := st.ListPVEServers()
	if err != nil {
		return cfg, err
	}
	for _, sv := range pveServers {
		svCfg, _ := st.GetPVEAlertConfig(sv.ID)
		cfg.PVEConfigs[sv.ID] = svCfg
		vmCfgs, _ := st.GetPVEVMAlertConfigs(sv.ID)
		cfg.PVEVMConfigs[sv.ID] = vmCfgs
	}

	pbsServers, err := st.ListPBSServers()
	if err != nil {
		return cfg, err
	}
	for _, sv := range pbsServers {
		svCfg, _ := st.GetPBSAlertConfig(sv.ID)
		cfg.PBSConfigs[sv.ID] = svCfg
	}

	return cfg, nil
}

// ── Evaluators ────────────────────────────────────────────────────────────────

func evalPVEDisk(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPVEServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PVEConfigs[sv.ID]
		threshold := cfg.GlobalDiskPct
		if svCfg.DiskPct != nil {
			threshold = *svCfg.DiskPct
		}
		if threshold == 0 {
			continue
		}

		rep, err := st.GetLatestPVEReport(sv.ID)
		if err != nil {
			continue
		}
		storages, err := st.GetPVEStoragesForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, stg := range storages {
			if !strings.Contains(stg.Content, "backup") {
				continue
			}
			info, err := st.GetPVEStorageInfo(stg.ID)
			if err != nil || info.Total == 0 {
				continue
			}
			pct := int(float64(info.Used) / float64(info.Total) * 100)
			if pct < threshold {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:pve:%d:%s", sv.ID, stg.Storage),
				ServerName: sv.Name, ServerID: sv.ID, ServerType: "pve",
				StoreName:  stg.Storage,
				Type:       domain.AlertTypeDisk,
				Severity:   alertDiskSeverity(pct),
				Title:      "Disco casi lleno",
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, alertFmtBytes(info.Used), alertFmtBytes(info.Total)),
				Value:      fmt.Sprintf("%d", pct),
				Threshold:  fmt.Sprintf("%d%%", threshold),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPVEBackupErrors(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPVEServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PVEConfigs[sv.ID]
		vmCfgs := cfg.PVEVMConfigs[sv.ID]

		rep, err := st.GetLatestPVEReport(sv.ID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, t := range tasks {
			if t.Status == "OK" {
				continue
			}
			vmCfg := findVMConfig(vmCfgs, t.VMID)
			if !resolveBackupErr(svCfg, vmCfg, cfg.GlobalBackupErr) {
				continue
			}
			name := t.VMName
			if name == "" {
				name = fmt.Sprintf("VM %d", t.VMID)
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("backup_error:pve:%d:%d", sv.ID, t.VMID),
				ServerName: sv.Name, ServerID: sv.ID, ServerType: "pve",
				VMID: t.VMID, VMName: name,
				Type:       domain.AlertTypeBackupError,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Backup fallido",
				Message:    fmt.Sprintf("%s: %s", name, t.Status),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPVEBackupSize(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	var alerts []domain.Alert
	for serverID, vmCfgs := range cfg.PVEVMConfigs {
		hasMinSize := false
		for _, vc := range vmCfgs {
			if vc.MinSizeMB != nil {
				hasMinSize = true
				break
			}
		}
		if !hasMinSize {
			continue
		}

		sv, err := st.GetPVEServer(serverID)
		if err != nil {
			continue
		}
		rep, err := st.GetLatestPVEReport(serverID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, t := range tasks {
			vmCfg := findVMConfig(vmCfgs, t.VMID)
			if vmCfg == nil || vmCfg.MinSizeMB == nil {
				continue
			}
			minBytes := int64(*vmCfg.MinSizeMB) * 1024 * 1024
			if t.Size >= minBytes {
				continue
			}
			name := t.VMName
			if name == "" {
				name = fmt.Sprintf("VM %d", t.VMID)
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("backup_size:pve:%d:%d", serverID, t.VMID),
				ServerName: sv.Name, ServerID: serverID, ServerType: "pve",
				VMID: t.VMID, VMName: name,
				Type:       domain.AlertTypeBackupSize,
				Severity:   domain.AlertSeverityWarning,
				Title:      "Backup demasiado pequeño",
				Message:    fmt.Sprintf("%s: %s (mín. %d MB)", name, alertFmtBytes(t.Size), *vmCfg.MinSizeMB),
				Value:      alertFmtBytes(t.Size),
				Threshold:  fmt.Sprintf("%d MB", *vmCfg.MinSizeMB),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPVEStale(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPVEServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		rep, err := st.GetLatestPVEReport(sv.ID)
		if err != nil || !rep.IsStale {
			continue
		}
		reason := rep.StaleReason
		if reason == "" {
			reason = "sin reporte reciente"
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("pve_stale:pve:%d", sv.ID),
			ServerName: sv.Name, ServerID: sv.ID, ServerType: "pve",
			Type:       domain.AlertTypePVEStale,
			Severity:   domain.AlertSeverityCritical,
			Title:      "Sin reporte",
			Message:    reason,
			DetectedAt: time.Now(),
		})
	}
	return alerts, nil
}

func evalPBSDisk(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPBSServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PBSConfigs[sv.ID]
		threshold := cfg.GlobalDiskPct
		if svCfg.DiskPct != nil {
			threshold = *svCfg.DiskPct
		}
		if threshold == 0 {
			continue
		}

		rep, err := st.GetLatestPBSReport(sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, ds := range stores {
			if ds.Total == 0 {
				continue
			}
			pct := int(float64(ds.Used) / float64(ds.Total) * 100)
			if pct < threshold {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:pbs:%d:%s", sv.ID, ds.Store),
				ServerName: sv.Name, ServerID: sv.ID, ServerType: "pbs",
				StoreName:  ds.Store,
				Type:       domain.AlertTypeDisk,
				Severity:   alertDiskSeverity(pct),
				Title:      "Disco casi lleno",
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, alertFmtBytes(ds.Used), alertFmtBytes(ds.Total)),
				Value:      fmt.Sprintf("%d", pct),
				Threshold:  fmt.Sprintf("%d%%", threshold),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPBSFill(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPBSServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	now := time.Now()
	for _, sv := range servers {
		svCfg := cfg.PBSConfigs[sv.ID]
		if svCfg.DaysUntilFull == nil {
			continue
		}
		threshold := *svCfg.DaysUntilFull
		if threshold == 0 {
			continue
		}

		rep, err := st.GetLatestPBSReport(sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, ds := range stores {
			if ds.EstimatedFullDate == 0 {
				continue
			}
			fullTime := time.Unix(ds.EstimatedFullDate, 0)
			if !fullTime.After(now) {
				continue
			}
			daysLeft := int(fullTime.Sub(now).Hours() / 24)
			if daysLeft >= threshold {
				continue
			}
			sev := domain.AlertSeverityWarning
			if daysLeft < 7 {
				sev = domain.AlertSeverityCritical
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("pbs_fill:pbs:%d:%s", sv.ID, ds.Store),
				ServerName: sv.Name, ServerID: sv.ID, ServerType: "pbs",
				StoreName:  ds.Store,
				Type:       domain.AlertTypePBSFill,
				Severity:   sev,
				Title:      "Disco se llena pronto",
				Message:    fmt.Sprintf("Se llenará en %d días (%s)", daysLeft, fullTime.Format("02 Jan")),
				Value:      fmt.Sprintf("%d días", daysLeft),
				Threshold:  fmt.Sprintf("%d días", threshold),
				DetectedAt: now,
			})
		}
	}
	return alerts, nil
}

func evalPBSStale(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPBSServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	now := time.Now()
	for _, sv := range servers {
		svCfg := cfg.PBSConfigs[sv.ID]
		staleHours := cfg.GlobalStaleHours
		if svCfg.StaleHours != nil {
			staleHours = *svCfg.StaleHours
		}
		if staleHours == 0 {
			continue
		}
		cutoff := now.Unix() - int64(staleHours)*3600

		rep, err := st.GetLatestPBSReport(sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, ds := range stores {
			snaps, err := st.GetPBSSnapshotsForStore(ds.ID)
			if err != nil {
				continue
			}
			for _, sn := range snaps {
				if sn.LastBackup == 0 || sn.LastBackup >= cutoff {
					continue
				}
				h := int(now.Sub(time.Unix(sn.LastBackup, 0)).Hours())
				var since string
				if h >= 48 {
					since = fmt.Sprintf("%dd", h/24)
				} else {
					since = fmt.Sprintf("%dh", h)
				}
				alerts = append(alerts, domain.Alert{
					ID:         fmt.Sprintf("pbs_stale:pbs:%d:%s:%s/%s", sv.ID, ds.Store, sn.BackupType, sn.BackupID),
					ServerName: sv.Name, ServerID: sv.ID, ServerType: "pbs",
					StoreName:  ds.Store,
					Type:       domain.AlertTypePBSStale,
					Severity:   domain.AlertSeverityWarning,
					Title:      "Snapshot sin actualizar",
					Message:    fmt.Sprintf("%s/%s sin backup desde hace %s", sn.BackupType, sn.BackupID, since),
					Value:      since,
					Threshold:  fmt.Sprintf("%dh", staleHours),
					DetectedAt: now,
				})
			}
		}
	}
	return alerts, nil
}

func evalPBSVerify(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	servers, err := st.ListPBSServers()
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PBSConfigs[sv.ID]
		if !svCfg.VerifyAlert {
			continue
		}

		rep, err := st.GetLatestPBSReport(sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(rep.ID)
		if err != nil {
			continue
		}
		for _, ds := range stores {
			snaps, err := st.GetPBSSnapshotsForStore(ds.ID)
			if err != nil {
				continue
			}
			for _, sn := range snaps {
				if sn.VerificationState == "" || sn.VerificationState == "ok" {
					continue
				}
				alerts = append(alerts, domain.Alert{
					ID:         fmt.Sprintf("pbs_verify:pbs:%d:%s:%s/%s", sv.ID, ds.Store, sn.BackupType, sn.BackupID),
					ServerName: sv.Name, ServerID: sv.ID, ServerType: "pbs",
					StoreName:  ds.Store,
					Type:       domain.AlertTypePBSVerify,
					Severity:   domain.AlertSeverityWarning,
					Title:      "Verificación fallida",
					Message:    fmt.Sprintf("%s/%s: verificación %s", sn.BackupType, sn.BackupID, sn.VerificationState),
					Value:      sn.VerificationState,
					DetectedAt: time.Now(),
				})
			}
		}
	}
	return alerts, nil
}

// ── Priority resolution helpers ───────────────────────────────────────────────

func resolveBackupErr(svCfg domain.PVEAlertConfig, vmCfg *domain.PVEVMAlertConfig, global bool) bool {
	if vmCfg != nil && vmCfg.BackupErr != nil {
		return *vmCfg.BackupErr != 0
	}
	if svCfg.BackupErr != nil {
		return *svCfg.BackupErr != 0
	}
	return global
}

func findVMConfig(configs []domain.PVEVMAlertConfig, vmid int64) *domain.PVEVMAlertConfig {
	for i := range configs {
		if configs[i].VMID == vmid {
			return &configs[i]
		}
	}
	return nil
}

func alertDiskSeverity(pct int) string {
	if pct >= 95 {
		return domain.AlertSeverityCritical
	}
	return domain.AlertSeverityWarning
}

func alertFmtBytes(b int64) string {
	const unit = 1000
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

package service

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

// AlertConfigs holds resolved thresholds for all servers.
// Global values from email_config act as fallback when a server has no per-server config.
type AlertConfigs struct {
	GlobalDiskPct             int
	GlobalWindowsDiskPct      int
	GlobalStaleHours          int
	GlobalBackupErr           bool
	GlobalPVEHeartbeatMinutes int
	Report                    *ReportService

	PVEConfigs     map[int64]domain.PVEAlertConfig
	PVEVMConfigs   map[int64][]domain.PVEVMAlertConfig // server_id → vm overrides
	PBSConfigs     map[int64]domain.PBSAlertConfig
	WindowsConfigs map[int64]domain.WindowsAlertConfig
}

// AlertEvaluator is the function signature every alert type must implement.
// Adding a new alert type = writing a function with this signature and registering it below.
type AlertEvaluator func(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error)

// evaluators is the registry of active alert types.
var evaluators = []AlertEvaluator{
	evalPVEDisk,
	evalPVEBackupErrors,
	evalPVEBackupSize,
	evalPVEMissingVM,
	evalPVEUnknownVM,
	evalPVEStale,
	evalPVEHeartbeat,
	evalHostSwap,
	evalPBSReportStale,
	evalPBSDisk,
	evalPBSFill,
	evalPBSVerify,
	evalWindowsDisk,
	evalWindowsHeartbeat,
	evalWindowsDiskHealth,
	evalWindowsMissingVolume,
}

// RunAll executes all registered evaluators. Individual errors are logged but do not
// stop execution - partial alerts are better than none.
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

func FilterMaintenanceAlerts(ctx context.Context, st *store.Store, alerts []domain.Alert) []domain.Alert {
	if len(alerts) == 0 {
		return alerts
	}
	maintenance, err := st.GetActiveServerMaintenances(ctx)
	if err != nil || len(maintenance) == 0 {
		return alerts
	}
	filtered := make([]domain.Alert, 0, len(alerts))
	for _, alert := range alerts {
		if _, ok := maintenance[store.ServerMaintenanceKey(alert.ServerType, alert.ServerID)]; ok {
			continue
		}
		filtered = append(filtered, alert)
	}
	return filtered
}

func CurrentAlerts(ctx context.Context, st *store.Store, rep *ReportService) ([]domain.Alert, error) {
	alerts, err := CurrentAlertsRaw(ctx, st, rep)
	if err != nil {
		return nil, err
	}
	return FilterMaintenanceAlerts(ctx, st, alerts), nil
}

func CurrentAlertsRaw(ctx context.Context, st *store.Store, rep *ReportService) ([]domain.Alert, error) {
	cfg, err := LoadAlertConfigs(ctx, st)
	if err != nil {
		return nil, err
	}
	cfg.Report = rep
	return RunAll(st, cfg)
}

// LoadAlertConfigs builds AlertConfigs from the store using email_config as global fallback.
func LoadAlertConfigs(ctx context.Context, st *store.Store) (AlertConfigs, error) {
	emailCfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		return AlertConfigs{}, err
	}
	cfg := AlertConfigs{
		GlobalDiskPct:             emailCfg.AlertDiskPct,
		GlobalWindowsDiskPct:      emailCfg.AlertWindowsDiskPct,
		GlobalBackupErr:           emailCfg.AlertBackupErr,
		GlobalStaleHours:          emailCfg.AlertPBSStaleHours,
		GlobalPVEHeartbeatMinutes: emailCfg.AlertPVEHeartbeatMinutes,
		PVEConfigs:                make(map[int64]domain.PVEAlertConfig),
		PVEVMConfigs:              make(map[int64][]domain.PVEVMAlertConfig),
		PBSConfigs:                make(map[int64]domain.PBSAlertConfig),
		WindowsConfigs:            make(map[int64]domain.WindowsAlertConfig),
	}

	pveServers, err := st.ListPVEServers(ctx)
	if err != nil {
		return cfg, err
	}
	pveConfigs, err := st.ListPVEAlertConfigs(ctx)
	if err != nil {
		return cfg, err
	}
	pveVMConfigs, err := st.ListPVEVMAlertConfigs(ctx)
	if err != nil {
		return cfg, err
	}
	for _, sv := range pveServers {
		svCfg := pveConfigs[sv.ID]
		svCfg.ServerID = sv.ID
		cfg.PVEConfigs[sv.ID] = svCfg
		cfg.PVEVMConfigs[sv.ID] = pveVMConfigs[sv.ID]
	}

	pbsServers, err := st.ListPBSServers(ctx)
	if err != nil {
		return cfg, err
	}
	pbsConfigs, err := st.ListPBSAlertConfigs(ctx)
	if err != nil {
		return cfg, err
	}
	for _, sv := range pbsServers {
		svCfg, ok := pbsConfigs[sv.ID]
		if !ok {
			svCfg.VerifyAlert = true
		}
		svCfg.ServerID = sv.ID
		cfg.PBSConfigs[sv.ID] = svCfg
	}

	windowsServers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return cfg, err
	}
	windowsConfigs, err := st.ListWindowsAlertConfigs(ctx)
	if err != nil {
		return cfg, err
	}
	for _, sv := range windowsServers {
		svCfg := windowsConfigs[sv.ID]
		svCfg.ServerID = sv.ID
		cfg.WindowsConfigs[sv.ID] = svCfg
	}

	return cfg, nil
}

// ── Evaluators ────────────────────────────────────────────────────────────────

func evalPVEDisk(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
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

		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		storages, err := st.GetPVEStoragesForReport(ctx, rep.ID)
		if err != nil {
			continue
		}
		for _, stg := range storages {
			if !strings.Contains(stg.Content, "backup") {
				continue
			}
			info, err := st.GetPVEStorageInfo(ctx, stg.ID)
			if err != nil || info.Total == 0 {
				continue
			}
			pct := int(float64(info.Used) / float64(info.Total) * 100)
			if pct < threshold {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:pve:%d:%s", sv.ID, stg.Storage),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
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
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PVEConfigs[sv.ID]
		vmCfgs := cfg.PVEVMConfigs[sv.ID]

		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(ctx, rep.ID)
		if err != nil {
			continue
		}
		if len(tasks) == 0 {
			status := strings.TrimSpace(rep.BackupStatus)
			if status == "" || strings.EqualFold(status, "OK") {
				continue
			}
			if !resolveBackupErr(svCfg, nil, cfg.GlobalBackupErr) {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("backup_error:pve:%d", sv.ID),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
				Type:       domain.AlertTypeBackupError,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Backup fallido",
				Message:    fmt.Sprintf("Ultimo job: %s", status),
				DetectedAt: time.Now(),
			})
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
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
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
	ctx := context.Background()
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

		sv, err := st.GetPVEServer(ctx, serverID)
		if err != nil {
			continue
		}
		rep, err := st.GetLatestPVEReport(ctx, serverID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(ctx, rep.ID)
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
				ServerName: sv.DisplayName, ServerID: serverID, ServerType: "pve",
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
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		configs, _ := st.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		if len(configs) > 0 && !domain.HasActiveVMBackupConfigs(configs) {
			continue
		}
		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("pve_stale:pve:%d", sv.ID),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
				Type:       domain.AlertTypePVEStale,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Sin reporte",
				Message:    "no se han recibido reportes",
				DetectedAt: time.Now(),
			})
			continue
		}
		stale := rep.IsStale
		reason := rep.StaleReason
		if cfg.Report != nil {
			stale, reason = cfg.Report.IsStaleForServerID(ctx, rep.ReportedAt, sv.ID)
		}
		if !stale {
			continue
		}
		if reason == "" {
			reason = "sin reporte reciente"
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("pve_stale:pve:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
			Type:       domain.AlertTypePVEStale,
			Severity:   domain.AlertSeverityCritical,
			Title:      "Sin reporte",
			Message:    reason,
			DetectedAt: time.Now(),
		})
	}
	return alerts, nil
}

func evalPVEHeartbeat(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	if cfg.GlobalPVEHeartbeatMinutes <= 0 {
		return nil, nil
	}
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	heartbeats, err := st.ListServerHeartbeatsByType(ctx, "pve")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	threshold := time.Duration(cfg.GlobalPVEHeartbeatMinutes) * time.Minute
	var alerts []domain.Alert
	for _, sv := range servers {
		hb, ok := heartbeats[sv.ID]
		if !ok {
			continue
		}
		age := now.Sub(hb.LastSeenAt)
		if age <= threshold {
			continue
		}
		since := alertFmtAge(age)
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("pve_heartbeat:pve:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
			Type:       domain.AlertTypePVEHeartbeat,
			Severity:   domain.AlertSeverityCritical,
			Title:      "Servidor offline",
			Message:    fmt.Sprintf("No se recibe señal del servidor desde hace %s", since),
			Value:      since,
			Threshold:  fmt.Sprintf("%d min", cfg.GlobalPVEHeartbeatMinutes),
			DetectedAt: now,
		})
	}
	return alerts, nil
}

func evalHostSwap(st *store.Store, _ AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	var alerts []domain.Alert
	now := time.Now()

	pveServers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	for _, sv := range pveServers {
		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil || !rep.SwapEnabled {
			continue
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("swap:pve:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
			Type:       domain.AlertTypeSwap,
			Severity:   domain.AlertSeverityWarning,
			Title:      "Swap activa",
			Message:    hostSwapMessage(rep.SwapUsed, rep.SwapTotal),
			Value:      alertFmtBytes(rep.SwapUsed),
			Threshold:  "swap desactivada",
			DetectedAt: now,
		})
	}

	pbsServers, err := st.ListPBSServers(ctx)
	if err != nil {
		return nil, err
	}
	for _, sv := range pbsServers {
		rep, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil || !rep.SwapEnabled {
			continue
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("swap:pbs:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
			Type:       domain.AlertTypeSwap,
			Severity:   domain.AlertSeverityWarning,
			Title:      "Swap activa",
			Message:    hostSwapMessage(rep.SwapUsed, rep.SwapTotal),
			Value:      alertFmtBytes(rep.SwapUsed),
			Threshold:  "swap desactivada",
			DetectedAt: now,
		})
	}
	return alerts, nil
}

func evalPBSReportStale(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPBSServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		staleHours := cfg.GlobalStaleHours
		if svCfg, ok := cfg.PBSConfigs[sv.ID]; ok && svCfg.StaleHours != nil {
			staleHours = *svCfg.StaleHours
		}
		if staleHours == 0 {
			continue
		}
		rep, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("pbs_report_stale:pbs:%d", sv.ID),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
				Type:       domain.AlertTypePBSReportStale,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Sin reporte",
				Message:    "no se han recibido reportes",
				DetectedAt: time.Now(),
			})
			continue
		}
		age := time.Since(rep.ReportedAt)
		if age <= time.Duration(staleHours)*time.Hour {
			continue
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("pbs_report_stale:pbs:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
			Type:       domain.AlertTypePBSReportStale,
			Severity:   domain.AlertSeverityCritical,
			Title:      "Sin reporte",
			Message:    fmt.Sprintf("Ultimo reporte hace %s (umbral %dh)", alertFmtAge(age), staleHours),
			Value:      alertFmtAge(age),
			Threshold:  fmt.Sprintf("%dh", staleHours),
			DetectedAt: time.Now(),
		})
	}
	return alerts, nil
}

func evalPBSDisk(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPBSServers(ctx)
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

		rep, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(ctx, rep.ID)
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
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
				StoreName:  ds.Store,
				Type:       domain.AlertTypeDisk,
				Severity:   alertDiskSeverity(pct),
				Title:      "Disco casi lleno",
				Message:    pbsDiskMessage(pct, ds.Used, ds.Total, ds.EstimatedFullDate, time.Now()),
				Value:      fmt.Sprintf("%d", pct),
				Threshold:  fmt.Sprintf("%d%%", threshold),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPBSFill(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPBSServers(ctx)
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

		rep, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(ctx, rep.ID)
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
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
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
	// PBS can intentionally keep old VM/CT snapshots for long-term safety.
	// Staleness is therefore tracked at the server-report level, not per group.
	return nil, nil
}

func evalPBSVerify(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPBSServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		svCfg := cfg.PBSConfigs[sv.ID]
		if !svCfg.VerifyAlert {
			continue
		}

		rep, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		stores, err := st.GetPBSStoresForReport(ctx, rep.ID)
		if err != nil {
			continue
		}
		for _, ds := range stores {
			snaps, err := st.GetPBSSnapshotsForStore(ctx, ds.ID)
			if err != nil {
				continue
			}
			for _, sn := range snaps {
				if sn.VerificationState == "" || sn.VerificationState == "ok" {
					continue
				}
				alerts = append(alerts, domain.Alert{
					ID:         fmt.Sprintf("pbs_verify:pbs:%d:%s:%s/%s", sv.ID, ds.Store, sn.BackupType, sn.BackupID),
					ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pbs",
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

func evalWindowsDisk(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		threshold := cfg.GlobalWindowsDiskPct
		if svCfg, ok := cfg.WindowsConfigs[sv.ID]; ok && svCfg.DiskPct != nil {
			threshold = *svCfg.DiskPct
		}
		if threshold <= 0 {
			continue
		}
		rep, err := st.GetLatestWindowsReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		disks, err := st.GetWindowsDisksForReport(ctx, rep.ID)
		if err != nil {
			continue
		}
		for _, disk := range disks {
			if !isWindowsLogicalAlertDisk(disk) {
				continue
			}
			if disk.Total <= 0 {
				continue
			}
			pct := int(float64(disk.Used) / float64(disk.Total) * 100)
			if pct < threshold {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:windows:%d:%s", sv.ID, disk.Name),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "windows",
				StoreName:  disk.Name,
				Type:       domain.AlertTypeDisk,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Disco casi lleno",
				Message:    fmt.Sprintf("%s al %d%% (%s libre)", disk.Name, pct, alertFmtBytes(disk.Free)),
				Value:      fmt.Sprintf("%d%%", pct),
				Threshold:  fmt.Sprintf("%d%%", threshold),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalWindowsHeartbeat(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	if cfg.GlobalPVEHeartbeatMinutes <= 0 {
		return nil, nil
	}
	ctx := context.Background()
	servers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return nil, err
	}
	heartbeats, err := st.ListServerHeartbeatsByType(ctx, "windows")
	if err != nil {
		return nil, err
	}
	now := time.Now()
	threshold := time.Duration(cfg.GlobalPVEHeartbeatMinutes) * time.Minute
	var alerts []domain.Alert
	for _, sv := range servers {
		lastSeen := time.Time{}
		if hb, ok := heartbeats[sv.ID]; ok {
			lastSeen = hb.LastSeenAt
		}
		if lastSeen.IsZero() {
			if rep, err := st.GetLatestWindowsReport(ctx, sv.ID); err == nil {
				lastSeen = rep.ReportedAt
			}
		}
		if lastSeen.IsZero() {
			continue
		}
		age := now.Sub(lastSeen)
		if age <= threshold {
			continue
		}
		since := alertFmtAge(age)
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("windows_heartbeat:windows:%d", sv.ID),
			ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "windows",
			Type:       domain.AlertTypeWindowsHeartbeat,
			Severity:   domain.AlertSeverityCritical,
			Title:      "Servidor offline",
			Message:    fmt.Sprintf("No se recibe senal del servidor Windows desde hace %s", since),
			Value:      since,
			Threshold:  fmt.Sprintf("%d min", cfg.GlobalPVEHeartbeatMinutes),
			DetectedAt: now,
		})
	}
	return alerts, nil
}

func evalWindowsDiskHealth(st *store.Store, _ AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		rep, err := st.GetLatestWindowsReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		disks, err := st.GetWindowsDisksForReport(ctx, rep.ID)
		if err != nil {
			continue
		}
		for _, disk := range disks {
			if !isWindowsLogicalAlertDisk(disk) {
				continue
			}
			health := strings.TrimSpace(strings.ToLower(disk.Health))
			if health == "" || health == "ok" || health == "healthy" || health == "normal" {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("windows_disk_health:windows:%d:%s", sv.ID, disk.Name),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "windows",
				StoreName:  disk.Name,
				Type:       domain.AlertTypeWindowsDiskHealth,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Salud de disco",
				Message:    fmt.Sprintf("%s informa estado %s", disk.Name, disk.Health),
				Value:      disk.Health,
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalWindowsMissingVolume(st *store.Store, _ AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		reports, err := st.ListWindowsReports(ctx, sv.ID, 2)
		if err != nil || len(reports) < 2 {
			continue
		}
		current, err := st.GetWindowsDisksForReport(ctx, reports[0].ID)
		if err != nil {
			continue
		}
		previous, err := st.GetWindowsDisksForReport(ctx, reports[1].ID)
		if err != nil {
			continue
		}
		currentNames := make(map[string]bool, len(current))
		for _, disk := range current {
			if !isWindowsLogicalAlertDisk(disk) {
				continue
			}
			currentNames[strings.ToUpper(strings.TrimSpace(disk.Name))] = true
		}
		for _, disk := range previous {
			if !isWindowsLogicalAlertDisk(disk) {
				continue
			}
			name := strings.ToUpper(strings.TrimSpace(disk.Name))
			if currentNames[name] {
				continue
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("windows_volume_missing:windows:%d:%s", sv.ID, name),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "windows",
				StoreName:  name,
				Type:       domain.AlertTypeWindowsVolumeGone,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Volumen no detectado",
				Message:    fmt.Sprintf("%s aparecia en el reporte anterior y no aparece en el ultimo reporte", name),
				Value:      name,
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func isWindowsLogicalAlertDisk(disk domain.WindowsDisk) bool {
	name := strings.TrimSpace(disk.Name)
	driveType := strings.TrimSpace(strings.ToLower(disk.DriveType))
	if driveType != "" && driveType != "fixed" {
		return false
	}
	return len(name) == 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

// ActiveAlertCounts returns the number of non-suppressed critical and warning alerts.
// Used by the web UI to show the sidebar badge on every page.
func ActiveAlertCounts(ctx context.Context, st *store.Store, rep *ReportService) (critical, warning int) {
	cfg, err := LoadAlertConfigs(ctx, st)
	if err != nil {
		return
	}
	cfg.Report = rep
	all, _ := RunAll(st, cfg)
	all = FilterMaintenanceAlerts(ctx, st, all)
	supps, _ := st.GetActiveSuppressions(ctx)
	for _, a := range all {
		if _, suppressed := supps[a.ID]; suppressed {
			continue
		}
		if a.Severity == domain.AlertSeverityCritical {
			critical++
		} else {
			warning++
		}
	}
	return
}

func evalPVEMissingVM(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(ctx, rep.ID)
		if err != nil || len(tasks) == 0 {
			continue
		}
		configs, err := st.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		if err != nil || len(configs) == 0 {
			continue
		}
		jobDay := time.Unix(tasks[0].StartTime, 0).Weekday()
		seenVMIDs := make(map[string]bool, len(tasks))
		for _, t := range tasks {
			seenVMIDs[strconv.FormatInt(t.VMID, 10)] = true
		}
		for _, c := range configs {
			if c.IsExcluded || !domain.VMScheduledForDay(c, jobDay) || seenVMIDs[c.VMID] {
				continue
			}
			name := c.VMName
			if name == "" {
				name = fmt.Sprintf("VM %s", c.VMID)
			}
			vmid, _ := strconv.ParseInt(c.VMID, 10, 64)
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("pve_missing_vm:pve:%d:%s", sv.ID, c.VMID),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
				VMID: vmid, VMName: name,
				Type:       domain.AlertTypePVEMissingVM,
				Severity:   domain.AlertSeverityCritical,
				Title:      "VM sin backup",
				Message:    fmt.Sprintf("%s: no aparece en el último job", name),
				DetectedAt: time.Now(),
			})
		}
	}
	return alerts, nil
}

func evalPVEUnknownVM(st *store.Store, cfg AlertConfigs) ([]domain.Alert, error) {
	ctx := context.Background()
	servers, err := st.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	var alerts []domain.Alert
	for _, sv := range servers {
		rep, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			continue
		}
		tasks, err := st.GetPVEBackupTasksForReport(ctx, rep.ID)
		if err != nil || len(tasks) == 0 {
			continue
		}
		configs, err := st.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		if err != nil || len(configs) == 0 {
			continue
		}
		configured := make(map[string]bool, len(configs))
		for _, c := range configs {
			configured[c.VMID] = true
		}
		for _, t := range tasks {
			vmidStr := strconv.FormatInt(t.VMID, 10)
			if configured[vmidStr] {
				continue
			}
			name := t.VMName
			if name == "" {
				name = fmt.Sprintf("VM %d", t.VMID)
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("pve_unknown_vm:pve:%d:%d", sv.ID, t.VMID),
				ServerName: sv.DisplayName, ServerID: sv.ID, ServerType: "pve",
				VMID: t.VMID, VMName: name,
				Type:       domain.AlertTypePVEUnknownVM,
				Severity:   domain.AlertSeverityWarning,
				Title:      "VM no contemplada",
				Message:    fmt.Sprintf("%s: aparece en el job pero no está en el backup config", name),
				DetectedAt: time.Now(),
			})
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

func alertFmtAge(d time.Duration) string {
	if d < time.Hour {
		mins := int(d.Minutes())
		if mins < 1 {
			mins = 1
		}
		return fmt.Sprintf("%dmin", mins)
	}
	if d < 48*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

func alertFmtBytes(b int64) string { return domain.FormatBytes(b) }

func hostSwapMessage(used, total int64) string {
	if total <= 0 {
		return "El host tiene swap activa"
	}
	if used > 0 {
		return fmt.Sprintf("Swap activa: %s usados de %s", alertFmtBytes(used), alertFmtBytes(total))
	}
	return fmt.Sprintf("Swap activa: %s configurados, sin uso actual", alertFmtBytes(total))
}

func pbsDiskMessage(pct int, used, total int64, estimatedFullDate int64, now time.Time) string {
	msg := fmt.Sprintf("%d%% usado (%s / %s)", pct, alertFmtBytes(used), alertFmtBytes(total))
	if estimatedFullDate == 0 {
		return msg
	}
	fullTime := time.Unix(estimatedFullDate, 0)
	if !fullTime.After(now) {
		return msg + "; estimacion de llenado vencida"
	}
	daysLeft := int(fullTime.Sub(now).Hours() / 24)
	if daysLeft < 1 {
		return msg + "; estimacion: menos de 1 dia"
	}
	return fmt.Sprintf("%s; estimacion: %d dias", msg, daysLeft)
}

package webhandlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func (h *WebH) Dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	pveServers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		slog.Error("list pve servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pbsServers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		slog.Error("list pbs servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	windowsServers, err := h.store.ListWindowsServers(ctx)
	if err != nil {
		slog.Error("list windows servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	maintenance, _ := h.store.GetActiveServerMaintenances(ctx)
	presentAlerts, _ := h.store.ListPresentAlerts(ctx)
	suppressed, _ := h.store.GetActiveSuppressions(ctx)
	alertSummary := summarizeDashboardAlerts(presentAlerts, suppressed, maintenance)

	pveReports, err := h.store.GetLatestPVEReports(ctx)
	if err != nil {
		slog.Error("list latest pve reports", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pveConfigs, _ := h.store.ListPVEVMBackupConfigsByServer(ctx)
	pveAlertConfigs, _ := h.store.ListPVEAlertConfigs(ctx)

	var pveOK, pveStale, pveBackupErrorCount, pveWithAlerts, pveMaintenance int
	var pveStaleIDs []int64
	var pveRows []map[string]any
	for _, sv := range pveServers {
		rep := pveReports[sv.ID]
		configs := pveConfigs[sv.ID]
		noVMsConfirmed := sv.BackupInventoryKnown && !sv.HasBackupVMs
		ignoreStale := domain.PVEStaleSuppressed(configs, pveAlertConfigs[sv.ID], noVMsConfirmed)
		isStale := (rep == nil || rep.IsStale) && !ignoreStale
		backupSeverity := alertSummary.PVEBackupSeverity[sv.ID]
		hasBackupError := backupSeverity == domain.AlertSeverityCritical
		hasBackupWarning := backupSeverity == domain.AlertSeverityWarning
		maint := maintenanceByServer(maintenance, "pve", sv.ID)
		health := buildServerHealth(alertSummary.ByServer[dashboardServerKey("pve", sv.ID)])
		rank := dashboardRankOK
		if maint.Active {
			pveMaintenance++
			rank = dashboardRankMaintenance
		} else if isStale {
			if _, silenced := suppressed[fmt.Sprintf("pve_stale:pve:%d", sv.ID)]; !silenced {
				pveStale++
				pveStaleIDs = append(pveStaleIDs, sv.ID)
			}
			rank = dashboardRankCritical
		} else if hasBackupError {
			pveBackupErrorCount++
			rank = dashboardRankCritical
		} else if health.HasAlerts {
			pveWithAlerts++
			rank = dashboardHealthRank(health)
		} else {
			pveOK++
		}
		row := map[string]any{
			"Server": sv, "IsStale": isStale,
			"HasBackupError": hasBackupError, "HasBackupWarning": hasBackupWarning,
			"Swap": buildSwapView(false, 0, 0), "Maintenance": maint,
			"Health": health, "Rank": rank,
		}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
			row["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)
		}
		pveRows = append(pveRows, row)
	}

	pbsReports, err := h.store.GetLatestPBSReports(ctx)
	if err != nil {
		slog.Error("list latest pbs reports", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pbsReportIDs := make([]int64, 0, len(pbsReports))
	for _, rep := range pbsReports {
		pbsReportIDs = append(pbsReportIDs, rep.ID)
	}
	pbsStores, err := h.store.GetPBSStoresForReports(ctx, pbsReportIDs)
	if err != nil {
		slog.Error("list pbs stores", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	pbsTasks, err := h.store.GetPBSTasksForReports(ctx, pbsReportIDs)
	if err != nil {
		slog.Error("list pbs maintenance tasks", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}

	var pbsOK, pbsStale, pbsWithAlerts, pbsMaintenance int
	var pbsStaleIDs []int64
	var pbsRows []map[string]any
	for _, sv := range pbsServers {
		rep := pbsReports[sv.ID]
		isStale := rep == nil || rep.IsStale
		maint := maintenanceByServer(maintenance, "pbs", sv.ID)
		fillLabel, fillClass := "Llenado OK", "ok"
		health := buildServerHealth(alertSummary.ByServer[dashboardServerKey("pbs", sv.ID)])
		rank := dashboardRankOK
		if maint.Active {
			pbsMaintenance++
			rank = dashboardRankMaintenance
		} else if isStale {
			if _, silenced := suppressed[fmt.Sprintf("pbs_report_stale:pbs:%d", sv.ID)]; !silenced {
				pbsStale++
				pbsStaleIDs = append(pbsStaleIDs, sv.ID)
			}
			rank = dashboardRankCritical
		} else {
			if health.HasAlerts {
				pbsWithAlerts++
				rank = dashboardHealthRank(health)
			} else {
				pbsOK++
			}
			fillLabel, fillClass = pbsFillBadge(pbsStores[rep.ID])
		}
		row := map[string]any{
			"Server":      sv,
			"IsStale":     isStale,
			"FillLabel":   fillLabel,
			"FillClass":   fillClass,
			"Swap":        buildSwapView(false, 0, 0),
			"Maintenance": maint,
			"Health":      health,
			"Rank":        rank,
		}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
			row["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)
			label, class, failed := pbsTaskSummary(pbsTasks[rep.ID])
			row["TaskLabel"] = label
			row["TaskClass"] = class
			row["HasTaskFailure"] = failed
		}
		pbsRows = append(pbsRows, row)
	}

	emailCfg, _ := h.store.GetEmailConfig(ctx)
	diskThreshold := 90
	heartbeatThreshold := 15
	if emailCfg != nil {
		diskThreshold = emailCfg.AlertWindowsDiskPct
		heartbeatThreshold = emailCfg.AlertPVEHeartbeatMinutes
	}
	windowsAlertConfigs, _ := h.store.ListWindowsAlertConfigs(ctx)
	windowsReports, err := h.store.GetLatestWindowsReports(ctx)
	if err != nil {
		slog.Error("list latest windows reports", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	windowsReportIDs := make([]int64, 0, len(windowsReports))
	for _, rep := range windowsReports {
		windowsReportIDs = append(windowsReportIDs, rep.ID)
	}
	windowsDisks, err := h.store.GetWindowsDisksForReports(ctx, windowsReportIDs)
	if err != nil {
		slog.Error("list windows disks", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	windowsHeartbeats, _ := h.store.ListServerHeartbeatsByType(ctx, "windows")
	var windowsOK, windowsOffline, windowsDiskAlerts, windowsWithAlerts int
	var windowsMaintenance int
	var windowsRows []map[string]any
	for _, sv := range windowsServers {
		rep := windowsReports[sv.ID]
		disks := []domain.WindowsDisk(nil)
		if rep != nil {
			disks = windowsDisks[rep.ID]
		}
		heartbeat := buildHeartbeatView(windowsHeartbeats[sv.ID], heartbeatThreshold)
		serverDiskThreshold := diskThreshold
		if alertCfg, ok := windowsAlertConfigs[sv.ID]; ok && alertCfg.DiskPct != nil {
			serverDiskThreshold = *alertCfg.DiskPct
		}
		diskAlert := windowsHasDiskAlert(disks, serverDiskThreshold)
		missingVolumeAlert := alertSummary.WindowsMissingVolume[sv.ID]
		maint := maintenanceByServer(maintenance, "windows", sv.ID)
		health := buildServerHealth(alertSummary.ByServer[dashboardServerKey("windows", sv.ID)])
		rank := dashboardRankOK
		if maint.Active {
			windowsMaintenance++
			rank = dashboardRankMaintenance
		} else if !heartbeat.Online {
			windowsOffline++
			rank = dashboardRankCritical
		} else if diskAlert || missingVolumeAlert {
			windowsDiskAlerts++
			rank = dashboardRankCritical
		} else if health.HasAlerts {
			windowsWithAlerts++
			rank = dashboardHealthRank(health)
		} else {
			windowsOK++
		}
		diskSummary := windowsDiskSummary(disks, serverDiskThreshold)
		if missingVolumeAlert {
			diskSummary = "Volumen no detectado"
		}
		row := map[string]any{
			"Server":       sv,
			"Heartbeat":    heartbeat,
			"HasDiskAlert": diskAlert || missingVolumeAlert,
			"DiskSummary":  diskSummary,
			"Maintenance":  maint,
			"Health":       health,
			"Rank":         rank,
		}
		if rep != nil {
			row["LastReport"] = rep.ReportedAt
		}
		windowsRows = append(windowsRows, row)
	}

	sortDashboardRows(pveRows)
	sortDashboardRows(pbsRows)
	sortDashboardRows(windowsRows)

	platforms := []dashboardPlatform{
		{
			Key: "pve", Title: "Proxmox VE", Icon: serverTypeIcon("pve"), ListURL: "/servers/pve", Rows: pveRows,
			Stats: []dashboardStat{
				{Label: "Operativos", Value: pveOK, Level: "ok", URL: "/servers/pve"},
				{Label: "Backup error", Value: pveBackupErrorCount, Level: "bad", URL: "/servers/pve?health=alerts"},
				{Label: "Con avisos", Value: pveWithAlerts, Level: "warn", URL: "/servers/pve?health=alerts"},
				{Label: "Sin reporte", Value: pveStale, Level: "bad", URL: dashboardStatusURL("pve", pveStaleIDs, "Sin reporte")},
				{Label: "Mantenimiento", Value: pveMaintenance, Level: "info", URL: "/servers/pve?filter=" + url.QueryEscape("Mantenimiento"), Optional: true},
			},
		},
		{
			Key: "pbs", Title: "Proxmox Backup", Icon: serverTypeIcon("pbs"), ListURL: "/servers/pbs", Rows: pbsRows,
			Stats: []dashboardStat{
				{Label: "Operativos", Value: pbsOK, Level: "ok", URL: "/servers/pbs"},
				{Label: "Con avisos", Value: pbsWithAlerts, Level: "warn", URL: "/servers/pbs?health=alerts"},
				{Label: "Sin reporte", Value: pbsStale, Level: "bad", URL: dashboardStatusURL("pbs", pbsStaleIDs, "Sin reporte")},
				{Label: "Mantenimiento", Value: pbsMaintenance, Level: "info", URL: "/servers/pbs?filter=" + url.QueryEscape("Mantenimiento"), Optional: true},
			},
		},
		{
			Key: "windows", Title: "Windows", Icon: serverTypeIcon("windows"), ListURL: "/servers/windows", Rows: windowsRows,
			Stats: []dashboardStat{
				{Label: "Operativos", Value: windowsOK, Level: "ok", URL: "/servers/windows"},
				{Label: "Disco", Value: windowsDiskAlerts, Level: "bad", URL: "/servers/windows?health=alerts"},
				{Label: "Con avisos", Value: windowsWithAlerts, Level: "warn", URL: "/servers/windows?health=alerts"},
				{Label: "Offline", Value: windowsOffline, Level: "bad", URL: "/servers/windows?filter=Offline"},
				{Label: "Mantenimiento", Value: windowsMaintenance, Level: "info", URL: "/servers/windows?filter=" + url.QueryEscape("Mantenimiento"), Optional: true},
			},
		},
	}

	h.tmpl.Render(w, r, "dashboard.html", map[string]any{
		"Username":           username,
		"Role":               role,
		"AlertCritical":      alertSummary.Critical,
		"AlertWarning":       alertSummary.Warning,
		"Platforms":          platforms,
		"ServerTotal":        len(pveRows) + len(pbsRows) + len(windowsRows),
		"PVEMaintenance":     pveMaintenance,
		"PBSMaintenance":     pbsMaintenance,
		"WindowsMaintenance": windowsMaintenance,
		"MaintenanceTotal":   pveMaintenance + pbsMaintenance + windowsMaintenance,
		"MaintenanceURL":     dashboardMaintenanceURL(maintenance),
	})
}

// dashboardStat is one counter inside a dashboard platform card. Problem
// counters only take their colour when they are non-zero.
type dashboardStat struct {
	Label    string
	Value    int
	Level    string
	URL      string
	Optional bool // hidden while zero
}

type dashboardPlatform struct {
	Key     string
	Title   string
	Icon    string
	ListURL string
	Stats   []dashboardStat
	Rows    []map[string]any
}

// Dashboard rows are ordered by urgency so problems are visible without scrolling.
const (
	dashboardRankCritical = iota
	dashboardRankWarning
	dashboardRankMaintenance
	dashboardRankOK
)

func dashboardHealthRank(health serverHealthView) int {
	if health.Critical > 0 {
		return dashboardRankCritical
	}
	return dashboardRankWarning
}

func sortDashboardRows(rows []map[string]any) {
	sort.SliceStable(rows, func(i, j int) bool {
		ri, _ := rows[i]["Rank"].(int)
		rj, _ := rows[j]["Rank"].(int)
		return ri < rj
	})
}

func dashboardServerKey(serverType string, serverID int64) string {
	return serverType + ":" + strconv.FormatInt(serverID, 10)
}

func dashboardStatusURL(serverType string, serverIDs []int64, filter string) string {
	if len(serverIDs) == 0 {
		return ""
	}
	base := "/servers/" + serverType
	if len(serverIDs) == 1 {
		return base + "/" + strconv.FormatInt(serverIDs[0], 10)
	}
	return base + "?filter=" + url.QueryEscape(filter)
}

func dashboardMaintenanceURL(maintenance map[string]domain.ServerMaintenance) string {
	active := make([]domain.ServerMaintenance, 0, len(maintenance))
	for _, item := range maintenance {
		if item.Active {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		return ""
	}
	if len(active) == 1 {
		return "/servers/" + active[0].ServerType + "/" + strconv.FormatInt(active[0].ServerID, 10)
	}
	return "/?filter=" + url.QueryEscape("Mantenimiento") + "#dashboard-servers"
}

type dashboardAlertSummary struct {
	ByServer             map[string]serverHealthView
	PVEBackupSeverity    map[int64]string
	WindowsMissingVolume map[int64]bool
	Critical             int
	Warning              int
}

func summarizeDashboardAlerts(alerts []domain.Alert, suppressed map[string]time.Time, maintenance map[string]domain.ServerMaintenance) dashboardAlertSummary {
	summary := dashboardAlertSummary{
		ByServer:             make(map[string]serverHealthView),
		PVEBackupSeverity:    make(map[int64]string),
		WindowsMissingVolume: make(map[int64]bool),
	}
	for _, alert := range alerts {
		if _, ok := suppressed[alert.ID]; ok || maintenanceByServer(maintenance, alert.ServerType, alert.ServerID).Active {
			continue
		}
		key := dashboardServerKey(alert.ServerType, alert.ServerID)
		health := summary.ByServer[key]
		health.Total++
		if alert.Severity == domain.AlertSeverityCritical {
			summary.Critical++
			health.Critical++
		} else {
			summary.Warning++
			health.Warning++
		}
		summary.ByServer[key] = health
		if alert.ServerType == "pve" && alert.Type == domain.AlertTypeBackupError {
			current := summary.PVEBackupSeverity[alert.ServerID]
			if alert.Severity == domain.AlertSeverityCritical || current == "" {
				summary.PVEBackupSeverity[alert.ServerID] = alert.Severity
			}
		}
		if alert.ServerType == "windows" && alert.Type == domain.AlertTypeWindowsVolumeGone {
			summary.WindowsMissingVolume[alert.ServerID] = true
		}
	}
	return summary
}

func pbsFillBadge(stores []domain.PBSStore) (label, class string) {
	if len(stores) == 0 {
		return "Llenado OK", "ok"
	}
	now := time.Now()
	var nearest *time.Time
	maxPct := 0
	for _, store := range stores {
		if store.Total > 0 {
			pct := int(float64(store.Used) / float64(store.Total) * 100)
			if pct > maxPct {
				maxPct = pct
			}
		}
		if store.EstimatedFullDate == 0 {
			continue
		}
		fullAt := time.Unix(store.EstimatedFullDate, 0)
		if !fullAt.After(now) {
			continue
		}
		if nearest == nil || fullAt.Before(*nearest) {
			nearest = &fullAt
		}
	}
	if nearest == nil {
		switch {
		case maxPct > 95:
			return fmt.Sprintf("%d%% · Sin riesgo", maxPct), "ok"
		case maxPct > 85:
			return fmt.Sprintf("%d%% · Sin riesgo", maxPct), "ok"
		}
		return "Llenado OK", "ok"
	}
	days := pbsDaysUntil(*nearest, now)
	return fmt.Sprintf("Lleno en %dd", days), pbsFillClass(days)
}

type pbsTaskDisplay struct {
	Title       string
	Detail      string
	Status      string
	StatusTitle string
	CSSClass    string
	EndedAt     time.Time
}

func pbsTaskDisplays(tasks []domain.PBSTask) []pbsTaskDisplay {
	rows := make([]pbsTaskDisplay, 0, len(tasks))
	for _, task := range tasks {
		row := pbsTaskDisplay{Status: "OK", StatusTitle: task.Status, CSSClass: "ok"}
		if domain.PBSTaskFailed(task) {
			row.CSSClass = "bad"
			row.Status = "ERROR"
		}
		if task.EndTime > 0 {
			row.EndedAt = time.Unix(task.EndTime, 0)
		}
		switch task.TaskType {
		case "gc":
			row.Title = "Garbage collection"
			row.Detail = task.Store
		default:
			row.Title = "Sincronizacion"
			remote := task.Remote
			if remote == "" {
				remote = task.JobID
			}
			datastore := task.RemoteStore
			if datastore == "" {
				datastore = task.Store
			}
			row.Detail = fmt.Sprintf("%s / %s", remote, datastore)
		}
		rows = append(rows, row)
	}
	return rows
}

func pbsTaskSummary(tasks []domain.PBSTask) (label, class string, failed bool) {
	if len(tasks) == 0 {
		return "", "", false
	}
	for _, task := range tasks {
		if domain.PBSTaskFailed(task) {
			return "Error sync/GC", "bad", true
		}
	}
	return "Sync/GC OK", "ok", false
}

type pbsStoreDisplay struct {
	StoreName  string
	Used       int64
	Total      int64
	BadgeLabel string
	BadgeClass string
	BadgeTitle string
	NoFillRisk bool
}

func pbsStoreDisplays(stores []domain.PBSStore) []pbsStoreDisplay {
	now := time.Now()
	rows := make([]pbsStoreDisplay, 0, len(stores))
	for _, store := range stores {
		row := pbsStoreDisplay{
			StoreName: store.Store,
			Used:      store.Used,
			Total:     store.Total,
		}
		if store.EstimatedFullDate > 0 {
			fullAt := time.Unix(store.EstimatedFullDate, 0)
			if fullAt.After(now) {
				days := pbsDaysUntil(fullAt, now)
				row.BadgeLabel = fmt.Sprintf("Lleno en %dd", days)
				row.BadgeClass = pbsFillClass(days)
				if days <= 14 {
					row.BadgeTitle = "Se llena en menos de 14 días"
				} else {
					row.BadgeTitle = fmt.Sprintf("Se llena en %d días", days)
				}
				rows = append(rows, row)
				continue
			}
		}
		if store.Total > 0 {
			pct := int(float64(store.Used) / float64(store.Total) * 100)
			switch {
			case pct > 95:
				row.BadgeLabel = fmt.Sprintf("%d%%", pct)
				row.BadgeClass = "ok"
				row.BadgeTitle = fmt.Sprintf("Disco al %d%%", pct)
				row.NoFillRisk = true
			case pct > 85:
				row.BadgeLabel = fmt.Sprintf("%d%%", pct)
				row.BadgeClass = "ok"
				row.BadgeTitle = fmt.Sprintf("Disco al %d%%", pct)
				row.NoFillRisk = true
			}
		}
		rows = append(rows, row)
	}
	return rows
}

func pbsMaxStoreUsagePercent(stores []domain.PBSStore) int {
	maxUsage := -1
	for _, store := range stores {
		if store.Total <= 0 {
			continue
		}
		usage := int(float64(store.Used) / float64(store.Total) * 100)
		if usage > maxUsage {
			maxUsage = usage
		}
	}
	return maxUsage
}

func pbsDaysUntil(fullAt, now time.Time) int {
	until := fullAt.Sub(now)
	days := int(until.Hours() / 24)
	if until%(24*time.Hour) != 0 {
		days++
	}
	if days < 1 {
		return 1
	}
	return days
}

func pbsFillClass(days int) string {
	switch {
	case days <= 14:
		return "bad"
	case days <= 30:
		return "warn"
	default:
		return "ok"
	}
}

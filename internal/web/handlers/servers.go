package webhandlers

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

type pveBackupJobRow struct {
	VMID       int64
	VMIDText   string
	VMName     string
	Status     string
	Duration   int64
	Size       int64
	Filename   string
	IsMissing  bool
	IsExcluded bool
}

type heartbeatView struct {
	Seen      bool
	Online    bool
	Label     string
	CSSClass  string
	LastSeen  time.Time
	Threshold int
}

type swapView struct {
	Enabled  bool
	Label    string
	CSSClass string
	Title    string
	Used     int64
	Total    int64
}

type alertOverrideView struct {
	Has   bool
	Count int
	Label string
	Title string
}

func buildSwapView(enabled bool, used, total int64) swapView {
	if !enabled {
		return swapView{Label: "Sin swap", CSSClass: "ok"}
	}
	view := swapView{
		Enabled:  true,
		Label:    "swap",
		CSSClass: "bad",
		Title:    fmt.Sprintf("Swap activa: %s usados de %s", domain.FormatBytes(used), domain.FormatBytes(total)),
		Used:     used,
		Total:    total,
	}
	if used <= 0 {
		view.Title = fmt.Sprintf("Swap activa: %s configurados, sin uso actual", domain.FormatBytes(total))
	}
	return view
}

func buildPVEAlertOverrideView(cfg domain.PVEAlertConfig) alertOverrideView {
	var parts []string
	if cfg.DiskPct != nil {
		parts = append(parts, fmt.Sprintf("Disco %d%%", *cfg.DiskPct))
	}
	if cfg.StaleHours != nil {
		if *cfg.StaleHours == 0 {
			parts = append(parts, "Sin reporte desactivado")
		} else {
			parts = append(parts, fmt.Sprintf("Sin reporte %dh", *cfg.StaleHours))
		}
	}
	if cfg.BackupErr != nil {
		if *cfg.BackupErr == 0 {
			parts = append(parts, "Backup fallido desactivado")
		} else {
			parts = append(parts, "Backup fallido activado")
		}
	}
	if cfg.ExpectedFinishTime != nil && *cfg.ExpectedFinishTime != "" {
		parts = append(parts, "Hora limite "+*cfg.ExpectedFinishTime)
	}
	return buildAlertOverrideView(parts)
}

func buildPBSAlertOverrideView(cfg domain.PBSAlertConfig) alertOverrideView {
	var parts []string
	if cfg.DiskPct != nil {
		parts = append(parts, fmt.Sprintf("Disco %d%%", *cfg.DiskPct))
	}
	if cfg.DaysUntilFull != nil {
		parts = append(parts, fmt.Sprintf("Llenado %dd", *cfg.DaysUntilFull))
	}
	if !cfg.VerifyAlert {
		parts = append(parts, "Verificacion desactivada")
	}
	return buildAlertOverrideView(parts)
}

func buildAlertOverrideView(parts []string) alertOverrideView {
	if len(parts) == 0 {
		return alertOverrideView{}
	}
	label := parts[0]
	if len(parts) > 1 {
		label = fmt.Sprintf("%d ajustes", len(parts))
	}
	return alertOverrideView{
		Has:   true,
		Count: len(parts),
		Label: label,
		Title: "Alertas personalizadas: " + strings.Join(parts, " | "),
	}
}

func latestPVESwapView(reports []domain.PVEReport) swapView {
	if len(reports) == 0 {
		return buildSwapView(false, 0, 0)
	}
	return buildSwapView(reports[0].SwapEnabled, reports[0].SwapUsed, reports[0].SwapTotal)
}

func latestPBSSwapView(reports []domain.PBSReport) swapView {
	if len(reports) == 0 {
		return buildSwapView(false, 0, 0)
	}
	return buildSwapView(reports[0].SwapEnabled, reports[0].SwapUsed, reports[0].SwapTotal)
}

func (h *WebH) PVEServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	servers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		slog.Error("list pve servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serverURLs := buildServerURLMap(h.store.ListAPIKeys(ctx))
	emailCfg, _ := h.store.GetEmailConfig(ctx)
	heartbeatThreshold := 15
	if emailCfg != nil {
		heartbeatThreshold = emailCfg.AlertPVEHeartbeatMinutes
	}
	heartbeats, _ := h.store.ListServerHeartbeatsByType(ctx, "pve")
	var rows []map[string]any
	for _, sv := range servers {
		configs, _ := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		ignoreStale := len(configs) > 0 && !domain.HasActiveVMBackupConfigs(configs)
		rep, _ := h.store.GetLatestPVEReport(ctx, sv.ID)
		stale := rep == nil && !ignoreStale
		if rep != nil {
			stale, _ = h.report.IsStaleForServerID(ctx, rep.ReportedAt, sv.ID)
		}
		alertCfg, _ := h.store.GetPVEAlertConfig(ctx, sv.ID)
		r2 := map[string]any{
			"Server":         sv,
			"IsStale":        stale,
			"TaskMissing":    0,
			"TaskUnknown":    0,
			"AlertConfig":    alertCfg,
			"AlertOverrides": buildPVEAlertOverrideView(alertCfg),
			"ServerURL":      serverURLFor(sv.APIKeyID, sv.Name, serverURLs),
			"Heartbeat":      buildHeartbeatView(heartbeats[sv.ID], heartbeatThreshold),
			"Swap":           buildSwapView(false, 0, 0),
		}
		if rep != nil {
			r2["LastReport"] = rep.ReportedAt
			r2["BackupStatus"] = rep.BackupStatus
			r2["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)

			tasks, _ := h.store.GetPVEBackupTasksForReport(ctx, rep.ID)
			if len(tasks) > 0 {
				if len(configs) > 0 {
					jobDay := time.Unix(tasks[0].StartTime, 0).Weekday()
					configured := make(map[string]bool, len(configs))
					for _, c := range configs {
						configured[c.VMID] = true
					}
					seenVMIDs := make(map[string]bool, len(tasks))
					for _, t := range tasks {
						seenVMIDs[strconv.FormatInt(t.VMID, 10)] = true
					}
					missing, unknown := 0, 0
					for _, c := range configs {
						if !c.IsExcluded && domain.VMScheduledForDay(c, jobDay) && !seenVMIDs[c.VMID] {
							missing++
						}
					}
					for _, t := range tasks {
						if !configured[strconv.FormatInt(t.VMID, 10)] {
							unknown++
						}
					}
					r2["TaskMissing"] = missing
					r2["TaskUnknown"] = unknown
				}
			}
		}
		rows = append(rows, r2)
	}
	h.tmpl.Render(w, r, "servers_pve.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) PVEServerDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reports, _ := h.store.ListPVEReports(ctx, id, 14)

	var storages []map[string]any
	var latestReportID int64
	if len(reports) > 0 {
		latestReportID = reports[0].ID
		sts, _ := h.store.GetPVEStoragesForReport(ctx, latestReportID)
		storageIDs := make([]int64, len(sts))
		for i, st := range sts {
			storageIDs[i] = st.ID
		}
		contentByStorage, _ := h.store.GetPVEStorageContentForStorages(ctx, storageIDs)
		for _, st := range sts {
			storages = append(storages, map[string]any{
				"Storage": st,
				"Content": contentByStorage[st.ID],
			})
		}
	}
	backupTasks, _ := h.store.GetPVEBackupTasksForReport(ctx, latestReportID)
	emailCfg, _ := h.store.GetEmailConfig(ctx)
	heartbeatThreshold := 15
	if emailCfg != nil {
		heartbeatThreshold = emailCfg.AlertPVEHeartbeatMinutes
	}
	hb, _ := h.store.GetServerHeartbeat(ctx, "pve", id)

	configuredVMIDs := make(map[string]bool)
	var missingVMs []map[string]any
	var backupRows []pveBackupJobRow
	var backupJobStart int64
	for _, t := range backupTasks {
		if backupJobStart == 0 || (t.StartTime > 0 && t.StartTime < backupJobStart) {
			backupJobStart = t.StartTime
		}
		backupRows = append(backupRows, pveBackupJobRow{
			VMID:     t.VMID,
			VMIDText: strconv.FormatInt(t.VMID, 10),
			VMName:   t.VMName,
			Status:   t.Status,
			Duration: t.Duration,
			Size:     t.Size,
			Filename: t.Filename,
		})
	}
	if len(backupTasks) > 0 {
		configs, _ := h.store.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		for _, c := range configs {
			configuredVMIDs[c.VMID] = true
		}
		if len(configs) > 0 {
			jobDay := time.Unix(backupTasks[0].StartTime, 0).Weekday()
			seenVMIDs := make(map[string]bool)
			for _, t := range backupTasks {
				seenVMIDs[strconv.FormatInt(t.VMID, 10)] = true
			}
			for _, c := range configs {
				if !domain.VMScheduledForDay(c, jobDay) || seenVMIDs[c.VMID] {
					continue
				}
				vmidNum, _ := strconv.ParseInt(c.VMID, 10, 64)
				missingVMs = append(missingVMs, map[string]any{
					"VMID":       c.VMID,
					"VMName":     c.VMName,
					"IsExcluded": c.IsExcluded,
				})
				backupRows = append(backupRows, pveBackupJobRow{
					VMID:       vmidNum,
					VMIDText:   c.VMID,
					VMName:     c.VMName,
					IsMissing:  true,
					IsExcluded: c.IsExcluded,
				})
			}
		}
	}
	sort.SliceStable(backupRows, func(i, j int) bool {
		if backupRows[i].VMID == backupRows[j].VMID {
			return backupRows[i].VMIDText < backupRows[j].VMIDText
		}
		return backupRows[i].VMID < backupRows[j].VMID
	})

	// Job history: up to 6 previous reports with tasks (reports[0] is already the latest)
	var historyIDs []int64
	for i := 1; i < len(reports); i++ {
		historyIDs = append(historyIDs, reports[i].ID)
	}
	historyTasks, _ := h.store.GetPVEBackupTasksForReports(ctx, historyIDs)
	var jobHistory []map[string]any
	for i := 1; i < len(reports) && len(jobHistory) < 6; i++ {
		tasks := historyTasks[reports[i].ID]
		if len(tasks) == 0 {
			continue
		}
		allOK := true
		for _, t := range tasks {
			if t.Status != "OK" {
				allOK = false
				break
			}
		}
		jobHistory = append(jobHistory, map[string]any{
			"Report": reports[i],
			"Tasks":  tasks,
			"AllOK":  allOK,
		})
	}

	alertCfg, _ := h.store.GetPVEAlertConfig(ctx, id)

	vmAlertCfgs, _ := h.store.GetPVEVMAlertConfigs(ctx, id)
	vmAlertMap := make(map[int64]domain.PVEVMAlertConfig, len(vmAlertCfgs))
	for _, c := range vmAlertCfgs {
		vmAlertMap[c.VMID] = c
	}

	h.tmpl.Render(w, r, "server_pve_detail.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"Server":          sv,
		"Reports":         reports,
		"Storages":        storages,
		"BackupTasks":     backupTasks,
		"BackupRows":      backupRows,
		"BackupJobStart":  backupJobStart,
		"Heartbeat":       buildHeartbeatViewPtr(hb, heartbeatThreshold),
		"Swap":            latestPVESwapView(reports),
		"MissingVMs":      missingVMs,
		"ConfiguredVMIDs": configuredVMIDs,
		"JobHistory":      jobHistory,
		"AlertConfig":     alertCfg,
		"VMAlertConfigs":  vmAlertMap,
		"Flash":           r.URL.Query().Get("flash"),
		"FlashOK":         r.URL.Query().Get("ok") == "1",
	})
}

func buildHeartbeatView(hb domain.ServerHeartbeat, thresholdMinutes int) heartbeatView {
	if hb.ID == 0 {
		return heartbeatView{Label: "Sin datos", CSSClass: "muted", Threshold: thresholdMinutes}
	}
	return buildHeartbeatViewPtr(&hb, thresholdMinutes)
}

func buildHeartbeatViewPtr(hb *domain.ServerHeartbeat, thresholdMinutes int) heartbeatView {
	if hb == nil || hb.ID == 0 {
		return heartbeatView{Label: "Sin datos", CSSClass: "muted", Threshold: thresholdMinutes}
	}
	online := true
	if thresholdMinutes > 0 {
		online = time.Since(hb.LastSeenAt) <= time.Duration(thresholdMinutes)*time.Minute
	}
	if online {
		return heartbeatView{
			Seen: true, Online: true, Label: "Online", CSSClass: "ok",
			LastSeen: hb.LastSeenAt, Threshold: thresholdMinutes,
		}
	}
	return heartbeatView{
		Seen: true, Online: false, Label: "Offline", CSSClass: "bad",
		LastSeen: hb.LastSeenAt, Threshold: thresholdMinutes,
	}
}

func (h *WebH) PVEServerReports(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	days := 7
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err2 := strconv.Atoi(d); err2 == nil && n >= 1 && n <= 365 {
			days = n
		}
	}

	reports, _ := h.store.ListPVEReportsByDays(ctx, id, days)

	var storages []map[string]any
	var totalBackups int
	if len(reports) > 0 {
		sts, _ := h.store.GetPVEStoragesForReport(ctx, reports[0].ID)
		storageIDs := make([]int64, len(sts))
		for i, st := range sts {
			storageIDs[i] = st.ID
		}
		infoByStorage, _ := h.store.GetPVEStorageInfoForStorages(ctx, storageIDs)
		contentByStorage, _ := h.store.GetPVEStorageContentForStorages(ctx, storageIDs)
		for _, st := range sts {
			content := contentByStorage[st.ID]
			backupCount := 0
			for _, c := range content {
				if c.Content == "backup" {
					backupCount++
				}
			}
			totalBackups += backupCount
			storages = append(storages, map[string]any{
				"Storage":     st,
				"Info":        infoByStorage[st.ID],
				"Content":     content,
				"BackupCount": backupCount,
			})
		}
	}

	// Build chart data: labels + durations (seconds) for JS
	type chartPoint struct {
		Label    string `json:"label"`
		Duration int64  `json:"duration"`
		Status   string `json:"status"`
	}
	var chartData []chartPoint
	for i := len(reports) - 1; i >= 0; i-- {
		rep := reports[i]
		chartData = append(chartData, chartPoint{
			Label:    rep.ReportedAt.Format("02/01 15:04"),
			Duration: rep.BackupDuration,
			Status:   rep.BackupStatus,
		})
	}

	h.tmpl.Render(w, r, "reports_pve.html", map[string]any{
		"Username":     username,
		"Role":         role,
		"Server":       sv,
		"Reports":      reports,
		"Days":         days,
		"Storages":     storages,
		"TotalBackups": totalBackups,
		"ChartData":    chartData,
	})
}

func (h *WebH) PBSServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	servers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		slog.Error("list pbs servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serverURLs := buildServerURLMap(h.store.ListAPIKeys(ctx))
	var rows []map[string]any
	for _, sv := range servers {
		rep, _ := h.store.GetLatestPBSReport(ctx, sv.ID)
		alertCfg, _ := h.store.GetPBSAlertConfig(ctx, sv.ID)
		r2 := map[string]any{
			"Server":         sv,
			"IsStale":        rep == nil || rep.IsStale,
			"AlertConfig":    alertCfg,
			"AlertOverrides": buildPBSAlertOverrideView(alertCfg),
			"ServerURL":      serverURLFor(sv.APIKeyID, sv.Name, serverURLs),
			"Swap":           buildSwapView(false, 0, 0),
		}
		if rep != nil {
			r2["LastReport"] = rep.ReportedAt
			r2["Swap"] = buildSwapView(rep.SwapEnabled, rep.SwapUsed, rep.SwapTotal)
			stores, _ := h.store.GetPBSStoresForReport(ctx, rep.ID)
			r2["Stores"] = pbsStoreDisplays(stores)
		}
		rows = append(rows, r2)
	}
	h.tmpl.Render(w, r, "servers_pbs.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) PBSServerDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPBSServer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reports, _ := h.store.ListPBSReports(ctx, id, 14)

	var storeDetails []map[string]any
	if len(reports) > 0 {
		stores, _ := h.store.GetPBSStoresForReport(ctx, reports[0].ID)
		for _, st := range stores {
			gc, _ := h.store.GetPBSGCStatus(ctx, st.ID)
			history, _ := h.store.GetPBSHistory(ctx, st.ID)
			snapshots, _ := h.store.GetPBSSnapshotsForStore(ctx, st.ID)
			storeDetails = append(storeDetails, map[string]any{
				"Store":     st,
				"GC":        gc,
				"History":   history,
				"Snapshots": snapshots,
			})
		}
	}

	alertCfg, _ := h.store.GetPBSAlertConfig(ctx, id)

	h.tmpl.Render(w, r, "server_pbs_detail.html", map[string]any{
		"Username":    username,
		"Role":        role,
		"Server":      sv,
		"Reports":     reports,
		"Stores":      storeDetails,
		"Swap":        latestPBSSwapView(reports),
		"AlertConfig": alertCfg,
		"Flash":       r.URL.Query().Get("flash"),
		"FlashOK":     r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) WindowsServers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	servers, err := h.store.ListWindowsServers(ctx)
	if err != nil {
		slog.Error("list windows servers", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	emailCfg, _ := h.store.GetEmailConfig(ctx)
	diskThreshold := 90
	heartbeatThreshold := 15
	if emailCfg != nil {
		diskThreshold = emailCfg.AlertDiskPct
		heartbeatThreshold = emailCfg.AlertPVEHeartbeatMinutes
	}
	reports, _ := h.store.GetLatestWindowsReports(ctx)
	reportIDs := make([]int64, 0, len(reports))
	for _, rep := range reports {
		reportIDs = append(reportIDs, rep.ID)
	}
	disksByReport, _ := h.store.GetWindowsDisksForReports(ctx, reportIDs)
	heartbeats, _ := h.store.ListServerHeartbeatsByType(ctx, "windows")
	serverURLs := buildServerURLMap(h.store.ListAPIKeys(ctx))

	var rows []map[string]any
	for _, sv := range servers {
		rep := reports[sv.ID]
		disks := []domain.WindowsDisk(nil)
		if rep != nil {
			disks = disksByReport[rep.ID]
		}
		rows = append(rows, map[string]any{
			"Server":       sv,
			"LastReport":   windowsReportTime(rep),
			"Disks":        windowsDiskDisplays(disks, diskThreshold),
			"DiskSummary":  windowsDiskSummary(disks, diskThreshold),
			"Heartbeat":    buildHeartbeatView(heartbeats[sv.ID], heartbeatThreshold),
			"ServerURL":    serverURLFor(sv.APIKeyID, sv.Name, serverURLs),
			"HasDiskAlert": windowsHasDiskAlert(disks, diskThreshold),
		})
	}
	h.tmpl.Render(w, r, "servers_windows.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) WindowsServerDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetWindowsServer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reports, _ := h.store.ListWindowsReports(ctx, id, 14)
	emailCfg, _ := h.store.GetEmailConfig(ctx)
	diskThreshold := 90
	heartbeatThreshold := 15
	if emailCfg != nil {
		diskThreshold = emailCfg.AlertDiskPct
		heartbeatThreshold = emailCfg.AlertPVEHeartbeatMinutes
	}
	var disks []domain.WindowsDisk
	if len(reports) > 0 {
		disks, _ = h.store.GetWindowsDisksForReport(ctx, reports[0].ID)
	}
	hb, _ := h.store.GetServerHeartbeat(ctx, "windows", id)
	heartbeat := domain.ServerHeartbeat{}
	if hb != nil {
		heartbeat = *hb
	}
	diskRows := windowsDiskDisplays(disks, diskThreshold)
	backURL := fmt.Sprintf("/servers/windows/%d", sv.ID)
	suppressions, _ := h.store.GetActiveSuppressions(ctx)
	alertControls := windowsAlertControls(sv.ID, diskRows, suppressions)
	h.tmpl.Render(w, r, "server_windows_detail.html", map[string]any{
		"Username":      username,
		"Role":          role,
		"Server":        sv,
		"Reports":       reports,
		"Disks":         diskRows,
		"DiskThreshold": diskThreshold,
		"Heartbeat":     buildHeartbeatView(heartbeat, heartbeatThreshold),
		"AlertControls": alertControls,
		"AllAlertIDs":   windowsAlertControlIDs(alertControls),
		"BackURL":       backURL,
	})
}

type windowsDiskDisplay struct {
	domain.WindowsDisk
	UsedPct    int
	BadgeClass string
	BadgeLabel string
	Title      string
}

type windowsAlertControl struct {
	ID         string
	Title      string
	Detail     string
	Suppressed bool
	Until      time.Time
}

func windowsReportTime(rep *domain.WindowsReport) *time.Time {
	if rep == nil {
		return nil
	}
	return &rep.ReportedAt
}

func windowsDiskDisplays(disks []domain.WindowsDisk, threshold int) []windowsDiskDisplay {
	rows := make([]windowsDiskDisplay, 0, len(disks))
	for _, disk := range disks {
		if !isWindowsLogicalDisk(disk) {
			continue
		}
		pct := 0
		if disk.Total > 0 {
			pct = int(float64(disk.Used) / float64(disk.Total) * 100)
		}
		row := windowsDiskDisplay{
			WindowsDisk: disk,
			UsedPct:     pct,
			BadgeClass:  "ok",
			BadgeLabel:  fmt.Sprintf("%d%%", pct),
			Title:       fmt.Sprintf("%s usado de %s", domain.FormatBytes(disk.Used), domain.FormatBytes(disk.Total)),
		}
		if threshold > 0 && pct >= threshold {
			row.BadgeClass = "bad"
		} else if pct >= 85 {
			row.BadgeClass = "warn"
		}
		if !windowsDiskHealthOK(disk.Health) {
			row.BadgeClass = "bad"
			row.BadgeLabel = disk.Health
			row.Title = "Estado del disco: " + disk.Health
		}
		rows = append(rows, row)
	}
	return rows
}

func windowsDiskSummary(disks []domain.WindowsDisk, threshold int) string {
	disks = windowsLogicalDisks(disks)
	if len(disks) == 0 {
		return "Sin discos"
	}
	if windowsHasDiskAlert(disks, threshold) {
		return "Revisar discos"
	}
	return "Discos OK"
}

func windowsHasDiskAlert(disks []domain.WindowsDisk, threshold int) bool {
	for _, disk := range disks {
		if !isWindowsLogicalDisk(disk) {
			continue
		}
		if !windowsDiskHealthOK(disk.Health) {
			return true
		}
		if threshold > 0 && disk.Total > 0 && int(float64(disk.Used)/float64(disk.Total)*100) >= threshold {
			return true
		}
	}
	return false
}

func windowsDiskHealthOK(health string) bool {
	health = strings.TrimSpace(strings.ToLower(health))
	return health == "" || health == "ok" || health == "healthy" || health == "normal"
}

func windowsLogicalDisks(disks []domain.WindowsDisk) []domain.WindowsDisk {
	out := make([]domain.WindowsDisk, 0, len(disks))
	for _, disk := range disks {
		if isWindowsLogicalDisk(disk) {
			out = append(out, disk)
		}
	}
	return out
}

func isWindowsLogicalDisk(disk domain.WindowsDisk) bool {
	name := strings.TrimSpace(disk.Name)
	driveType := strings.TrimSpace(strings.ToLower(disk.DriveType))
	if driveType != "" && driveType != "fixed" {
		return false
	}
	return len(name) == 2 && name[1] == ':' && ((name[0] >= 'A' && name[0] <= 'Z') || (name[0] >= 'a' && name[0] <= 'z'))
}

func windowsAlertControls(serverID int64, disks []windowsDiskDisplay, suppressions map[string]time.Time) []windowsAlertControl {
	rows := []windowsAlertControl{
		windowsAlertControlFromID(fmt.Sprintf("windows_heartbeat:windows:%d", serverID), "Conexión", "Servidor Windows sin heartbeat", suppressions),
	}
	for _, disk := range disks {
		rows = append(rows,
			windowsAlertControlFromID(fmt.Sprintf("disk:windows:%d:%s", serverID, disk.Name), "Disco lleno "+disk.Name, "Uso de disco por encima del umbral global", suppressions),
			windowsAlertControlFromID(fmt.Sprintf("windows_disk_health:windows:%d:%s", serverID, disk.Name), "Salud "+disk.Name, "Estado SMART/salud de la unidad", suppressions),
		)
	}
	return rows
}

func windowsAlertControlFromID(id, title, detail string, suppressions map[string]time.Time) windowsAlertControl {
	until, suppressed := suppressions[id]
	return windowsAlertControl{
		ID:         id,
		Title:      title,
		Detail:     detail,
		Suppressed: suppressed,
		Until:      until,
	}
}

func windowsAlertControlIDs(rows []windowsAlertControl) string {
	ids := make([]string, 0, len(rows))
	for _, row := range rows {
		ids = append(ids, row.ID)
	}
	return strings.Join(ids, ",")
}

func (h *WebH) PVEServerReportsCSV(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	days := 90
	if d := r.URL.Query().Get("days"); d != "" {
		if n, err2 := strconv.Atoi(d); err2 == nil && n >= 1 && n <= 365 {
			days = n
		}
	}
	reports, _ := h.store.ListPVEReportsByDays(ctx, id, days)

	filename := fmt.Sprintf("reportes_%s_%s.csv", sv.DisplayName, time.Now().Format("20060102"))
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)

	wr := csv.NewWriter(w)
	_ = wr.Write([]string{"Fecha", "Estado backup", "Inicio backup", "Duracion (s)", "Sin reporte"})
	for _, rep := range reports {
		stale := "no"
		if rep.IsStale {
			stale = "si"
		}
		start := ""
		if rep.BackupStarttime != 0 {
			start = time.Unix(rep.BackupStarttime, 0).Format("02/01/2006 15:04")
		}
		_ = wr.Write([]string{
			rep.ReportedAt.Format("02/01/2006 15:04"),
			rep.BackupStatus,
			start,
			strconv.FormatInt(rep.BackupDuration, 10),
			stale,
		})
	}
	wr.Flush()
}

func buildServerURLMap(keys []domain.APIKey, _ error) map[string]string {
	m := make(map[string]string)
	for _, k := range keys {
		if k.ID > 0 && k.ServerURL != "" {
			m["id:"+strconv.FormatInt(k.ID, 10)] = k.ServerURL
		}
		if k.ServerName != "" && k.ServerURL != "" {
			m["name:"+k.ServerName] = k.ServerURL
		}
	}
	return m
}

func serverURLFor(apiKeyID int64, hostname string, urls map[string]string) string {
	if apiKeyID > 0 {
		if u := urls["id:"+strconv.FormatInt(apiKeyID, 10)]; u != "" {
			return u
		}
	}
	return urls["name:"+hostname]
}

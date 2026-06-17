package webhandlers

import (
	"encoding/csv"
	"fmt"
	"log/slog"
	"net/http"
	"sort"
	"strconv"
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
			"Server":      sv,
			"IsStale":     stale,
			"TaskMissing": 0,
			"TaskUnknown": 0,
			"AlertConfig": alertCfg,
			"ServerURL":   serverURLFor(sv.APIKeyID, sv.Name, serverURLs),
			"Heartbeat":   buildHeartbeatView(heartbeats[sv.ID], heartbeatThreshold),
			"Swap":        buildSwapView(false, 0, 0),
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
			"Server":      sv,
			"IsStale":     rep == nil || rep.IsStale,
			"AlertConfig": alertCfg,
			"ServerURL":   serverURLFor(sv.APIKeyID, sv.Name, serverURLs),
			"Swap":        buildSwapView(false, 0, 0),
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

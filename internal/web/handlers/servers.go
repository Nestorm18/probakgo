package webhandlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/session"
)

func (h *WebH) PVEServers(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	servers, err := h.store.ListPVEServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rows []map[string]any
	for _, sv := range servers {
		rep, _ := h.store.GetLatestPVEReport(sv.ID)
		r2 := map[string]any{
			"Server":  sv,
			"IsStale": rep == nil || rep.IsStale,
		}
		if rep != nil {
			r2["LastReport"] = rep.ReportedAt
			r2["BackupStatus"] = rep.BackupStatus
		}
		rows = append(rows, r2)
	}
	h.tmpl.Render(w, "servers_pve.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
	})
}

func (h *WebH) PVEServerDetail(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reports, _ := h.store.ListPVEReports(id, 14)

	var storages []map[string]any
	if len(reports) > 0 {
		sts, _ := h.store.GetPVEStoragesForReport(reports[0].ID)
		for _, st := range sts {
			content, _ := h.store.GetPVEStorageContent(st.ID)
			storages = append(storages, map[string]any{
				"Storage": st,
				"Content": content,
			})
		}
	}

	h.tmpl.Render(w, "server_pve_detail.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Server":   sv,
		"Reports":  reports,
		"Storages": storages,
	})
}

func (h *WebH) PVEServerReports(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(id)
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

	reports, _ := h.store.ListPVEReportsByDays(id, days)

	var storages []map[string]any
	var totalBackups int
	if len(reports) > 0 {
		sts, _ := h.store.GetPVEStoragesForReport(reports[0].ID)
		for _, st := range sts {
			info, _ := h.store.GetPVEStorageInfo(st.ID)
			content, _ := h.store.GetPVEStorageContent(st.ID)
			backupCount := 0
			for _, c := range content {
				if c.Content == "backup" {
					backupCount++
				}
			}
			totalBackups += backupCount
			storages = append(storages, map[string]any{
				"Storage":     st,
				"Info":        info,
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

	h.tmpl.Render(w, "reports_pve.html", map[string]any{
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
	username, role, _ := session.GetUser(r)
	servers, err := h.store.ListPBSServers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	var rows []map[string]any
	for _, sv := range servers {
		rep, _ := h.store.GetLatestPBSReport(sv.ID)
		r2 := map[string]any{
			"Server":  sv,
			"IsStale": rep == nil || rep.IsStale,
		}
		if rep != nil {
			r2["LastReport"] = rep.ReportedAt
			stores, _ := h.store.GetPBSStoresForReport(rep.ID)
			r2["Stores"] = stores
		}
		rows = append(rows, r2)
	}
	h.tmpl.Render(w, "servers_pbs.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Rows":     rows,
	})
}

func (h *WebH) PBSServerDetail(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPBSServer(id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	reports, _ := h.store.ListPBSReports(id, 14)

	var storeDetails []map[string]any
	if len(reports) > 0 {
		stores, _ := h.store.GetPBSStoresForReport(reports[0].ID)
		for _, st := range stores {
			gc, _ := h.store.GetPBSGCStatus(st.ID)
			history, _ := h.store.GetPBSHistory(st.ID)
			storeDetails = append(storeDetails, map[string]any{
				"Store":   st,
				"GC":      gc,
				"History": history,
			})
		}
	}

	h.tmpl.Render(w, "server_pbs_detail.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Server":   sv,
		"Reports":  reports,
		"Stores":   storeDetails,
	})
}

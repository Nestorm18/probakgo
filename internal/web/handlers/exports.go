package webhandlers

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
)

func (h *WebH) AlertsCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := h.alertExportRows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveCSV(w, "alertas_"+time.Now().Format("20060102")+".csv", func(wr *csv.Writer) {
		_ = writeSafeCSV(wr, []string{"Estado", "Suprimida hasta", "Severidad", "Tipo", "Servidor", "Alerta", "Mensaje", "Valor", "Umbral", "Datastore", "VMID", "VM"})
		for _, row := range rows {
			a := row.Alert
			_ = writeSafeCSV(wr, []string{row.State, row.SuppressedUntil, a.Severity, a.ServerType, a.ServerName, a.Title, a.Message, a.Value, a.Threshold, a.StoreName, int64Text(a.VMID), a.VMName})
		}
	})
}

func (h *WebH) AlertsJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.alertExportRows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	critical, warning := 0, 0
	for _, row := range rows {
		if row.State != "active" {
			continue
		}
		if row.Alert.Severity == domain.AlertSeverityCritical {
			critical++
		} else {
			warning++
		}
	}
	serveJSON(w, map[string]any{
		"critical":     critical,
		"warning":      warning,
		"generated_at": time.Now(),
		"alerts":       rows,
	})
}

func (h *WebH) PVEServersCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := h.exportPVERows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveCSV(w, "pve_"+time.Now().Format("20060102")+".csv", func(wr *csv.Writer) {
		_ = writeSafeCSV(wr, []string{"ID", "Nombre", "Hostname", "IP", "IP publica", "Cliente", "Conexion", "Ultimo heartbeat", "Ultimo reporte", "Backup", "Estado"})
		for _, row := range rows {
			_ = writeSafeCSV(wr, []string{row.ID, row.Name, row.Hostname, row.IP, row.PublicIP, row.ClientVersion, row.Connection, row.LastHeartbeat, row.LastReport, row.BackupStatus, row.State})
		}
	})
}

func (h *WebH) PVEServersJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.exportPVERows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveJSON(w, rows)
}

func (h *WebH) PBSServersCSV(w http.ResponseWriter, r *http.Request) {
	rows, err := h.exportPBSRows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveCSV(w, "pbs_"+time.Now().Format("20060102")+".csv", func(wr *csv.Writer) {
		_ = writeSafeCSV(wr, []string{"ID", "Nombre", "Hostname", "IP", "IP publica", "Cliente", "Ultimo reporte", "Estado", "Datastores"})
		for _, row := range rows {
			_ = writeSafeCSV(wr, []string{row.ID, row.Name, row.Hostname, row.IP, row.PublicIP, row.ClientVersion, row.LastReport, row.State, row.Datastores})
		}
	})
}

func (h *WebH) PBSServersJSON(w http.ResponseWriter, r *http.Request) {
	rows, err := h.exportPBSRows(r)
	if err != nil {
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	serveJSON(w, rows)
}

func (h *WebH) PVEServerReportsJSON(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPVEServer(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	days := exportDays(r, 90)
	reports, _ := h.store.ListPVEReportsByDays(r.Context(), id, days)
	serveJSON(w, map[string]any{"server": sv, "days": days, "reports": reports})
}

func (h *WebH) PBSServerReportsCSV(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPBSServer(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	limit := exportLimit(r, 365)
	reports, _ := h.store.ListPBSReports(r.Context(), id, limit)
	filename := fmt.Sprintf("reportes_%s_%s.csv", sv.DisplayName, time.Now().Format("20060102"))
	serveCSV(w, filename, func(wr *csv.Writer) {
		_ = writeSafeCSV(wr, []string{"Fecha", "Sin reporte", "Motivo"})
		for _, rep := range reports {
			stale := "no"
			if rep.IsStale {
				stale = "si"
			}
			_ = writeSafeCSV(wr, []string{formatExportTime(rep.ReportedAt), stale, rep.StaleReason})
		}
	})
}

func (h *WebH) PBSServerReportsJSON(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	sv, err := h.store.GetPBSServer(r.Context(), id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	limit := exportLimit(r, 365)
	reports, _ := h.store.ListPBSReports(r.Context(), id, limit)
	serveJSON(w, map[string]any{"server": sv, "limit": limit, "reports": reports})
}

type pveExportRow struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Hostname      string `json:"hostname"`
	IP            string `json:"ip"`
	PublicIP      string `json:"public_ip"`
	ClientVersion string `json:"client_version"`
	Connection    string `json:"connection"`
	LastHeartbeat string `json:"last_heartbeat"`
	LastReport    string `json:"last_report"`
	BackupStatus  string `json:"backup_status"`
	State         string `json:"state"`
}

type pbsExportRow struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Hostname      string `json:"hostname"`
	IP            string `json:"ip"`
	PublicIP      string `json:"public_ip"`
	ClientVersion string `json:"client_version"`
	LastReport    string `json:"last_report"`
	State         string `json:"state"`
	Datastores    string `json:"datastores"`
}

type alertExportRow struct {
	State           string       `json:"state"`
	SuppressedUntil string       `json:"suppressed_until,omitempty"`
	Alert           domain.Alert `json:"alert"`
}

func (h *WebH) exportPVERows(r *http.Request) ([]pveExportRow, error) {
	ctx := r.Context()
	servers, err := h.store.ListPVEServers(ctx)
	if err != nil {
		return nil, err
	}
	cfg, _ := h.store.GetEmailConfig(ctx)
	threshold := 15
	if cfg != nil {
		threshold = cfg.AlertPVEHeartbeatMinutes
	}
	heartbeats, _ := h.store.ListServerHeartbeatsByType(ctx, "pve")
	rows := make([]pveExportRow, 0, len(servers))
	for _, sv := range servers {
		rep, _ := h.store.GetLatestPVEReport(ctx, sv.ID)
		state := "Sin reporte"
		lastReport := ""
		backupStatus := ""
		if rep != nil {
			lastReport = formatExportTime(rep.ReportedAt)
			backupStatus = rep.BackupStatus
			stale, _ := h.report.IsStaleForServerID(ctx, rep.ReportedAt, sv.ID)
			if stale || rep.IsStale {
				state = "Sin reporte"
			} else {
				state = "Activo"
			}
		}
		hb := buildHeartbeatView(heartbeats[sv.ID], threshold)
		rows = append(rows, pveExportRow{
			ID: strconv.FormatInt(sv.ID, 10), Name: sv.DisplayName, Hostname: sv.Name,
			IP: sv.IP, PublicIP: sv.PublicIP, ClientVersion: sv.ClientVersion,
			Connection: hb.Label, LastHeartbeat: formatExportTime(hb.LastSeen),
			LastReport: lastReport, BackupStatus: backupStatus, State: state,
		})
	}
	return rows, nil
}

func (h *WebH) alertExportRows(r *http.Request) ([]alertExportRow, error) {
	ctx := r.Context()
	allAlerts, err := h.runAlerts(ctx, true)
	if err != nil {
		return nil, err
	}
	suppressions, _ := h.store.GetActiveSuppressions(ctx)
	seen := make(map[string]bool)
	rows := make([]alertExportRow, 0, len(allAlerts)+len(suppressions))
	for _, alert := range allAlerts {
		seen[alert.ID] = true
		row := alertExportRow{State: "active", Alert: alert}
		if until, ok := suppressions[alert.ID]; ok {
			row.State = "suppressed"
			row.SuppressedUntil = formatExportTime(until)
		}
		rows = append(rows, row)
	}
	for alertID, until := range suppressions {
		if seen[alertID] {
			continue
		}
		rows = append(rows, alertExportRow{
			State:           "suppressed",
			SuppressedUntil: formatExportTime(until),
			Alert:           h.alertFromSuppressionID(ctx, alertID),
		})
	}
	return rows, nil
}

func (h *WebH) exportPBSRows(r *http.Request) ([]pbsExportRow, error) {
	ctx := r.Context()
	servers, err := h.store.ListPBSServers(ctx)
	if err != nil {
		return nil, err
	}
	rows := make([]pbsExportRow, 0, len(servers))
	for _, sv := range servers {
		rep, _ := h.store.GetLatestPBSReport(ctx, sv.ID)
		state := "Sin reporte"
		lastReport := ""
		datastores := ""
		if rep != nil {
			lastReport = formatExportTime(rep.ReportedAt)
			if !rep.IsStale {
				state = "Activo"
			}
			if stores, err := h.store.GetPBSStoresForReport(ctx, rep.ID); err == nil {
				datastores = summarizePBSStores(stores)
			}
		}
		rows = append(rows, pbsExportRow{
			ID: strconv.FormatInt(sv.ID, 10), Name: sv.DisplayName, Hostname: sv.Name,
			IP: sv.IP, PublicIP: sv.PublicIP, ClientVersion: sv.ClientVersion,
			LastReport: lastReport, State: state, Datastores: datastores,
		})
	}
	return rows, nil
}

func serveCSV(w http.ResponseWriter, filename string, fn func(*csv.Writer)) {
	w.Header().Set("Content-Type", "text/csv; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="`+filename+`"`)
	wr := csv.NewWriter(w)
	fn(wr)
	wr.Flush()
}

func writeSafeCSV(wr *csv.Writer, row []string) error {
	for i, value := range row {
		row[i] = escapeCSVFormula(value)
	}
	return wr.Write(row)
}

func escapeCSVFormula(value string) string {
	trimmed := strings.TrimLeft(value, " \t\r\n")
	if trimmed == "" {
		return value
	}
	switch trimmed[0] {
	case '=', '+', '-', '@':
		return "'" + value
	default:
		return value
	}
}

func serveJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func exportDays(r *http.Request, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get("days")); err == nil && n >= 1 && n <= 365 {
		return n
	}
	return fallback
}

func exportLimit(r *http.Request, fallback int) int {
	if n, err := strconv.Atoi(r.URL.Query().Get("limit")); err == nil && n >= 1 && n <= 5000 {
		return n
	}
	return fallback
}

func formatExportTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}

func int64Text(v int64) string {
	if v == 0 {
		return ""
	}
	return strconv.FormatInt(v, 10)
}

func summarizePBSStores(stores []domain.PBSStore) string {
	parts := make([]string, 0, len(stores))
	for _, st := range stores {
		pct := int64(0)
		if st.Total > 0 {
			pct = st.Used * 100 / st.Total
		}
		parts = append(parts, fmt.Sprintf("%s %d%% (%s/%s)", st.Store, pct, domain.FormatBytes(st.Used), domain.FormatBytes(st.Total)))
	}
	return stringsJoin(parts, "; ")
}

func stringsJoin(parts []string, sep string) string {
	if len(parts) == 0 {
		return ""
	}
	out := parts[0]
	for _, part := range parts[1:] {
		out += sep + part
	}
	return out
}

package webhandlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

type alertGroup struct {
	ServerName string
	ServerType string
	ServerID   int64
	Critical   int
	Warning    int
	Alerts     []domain.Alert
}

type suppressedAlertRow struct {
	Alert domain.Alert
	Until time.Time
}

type suppressedAlertGroup struct {
	ServerName string
	ServerType string
	ServerID   int64
	Rows       []suppressedAlertRow
}

type alertStatusItem struct {
	ID         string `json:"id"`
	Severity   string `json:"severity"`
	Title      string `json:"title"`
	Message    string `json:"message"`
	ServerName string `json:"server_name"`
	ServerType string `json:"server_type"`
	ServerID   int64  `json:"server_id"`
	Type       string `json:"type"`
}

type alertStatusResponse struct {
	Critical    int               `json:"critical"`
	Warning     int               `json:"warning"`
	Alerts      []alertStatusItem `json:"alerts"`
	GeneratedAt int64             `json:"generated_at"`
}

func (h *WebH) Alerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	allAlerts, err := h.runAlerts(ctx, true)
	if err != nil {
		slog.Error("run alerts", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}

	suppressions, _ := h.store.GetActiveSuppressions(ctx)

	var active, suppressed []domain.Alert
	matchedSuppressionIDs := make(map[string]bool)
	for _, a := range allAlerts {
		if _, ok := suppressions[a.ID]; ok {
			suppressed = append(suppressed, a)
			matchedSuppressionIDs[a.ID] = true
		} else {
			active = append(active, a)
		}
	}

	var critical, warning int
	for _, a := range active {
		if a.Severity == domain.AlertSeverityCritical {
			critical++
		} else {
			warning++
		}
	}

	filterSeverity := r.URL.Query().Get("severity")
	filterServer := r.URL.Query().Get("server")

	var filtered []domain.Alert
	for _, a := range active {
		if filterSeverity != "" && a.Severity != filterSeverity {
			continue
		}
		if filterServer != "" && a.ServerName != filterServer {
			continue
		}
		filtered = append(filtered, a)
	}

	// Build suppressed list with until times for template.
	var suppressedRows []suppressedAlertRow
	for _, a := range suppressed {
		suppressedRows = append(suppressedRows, suppressedAlertRow{
			Alert: a,
			Until: suppressions[a.ID],
		})
	}
	for alertID, until := range suppressions {
		if matchedSuppressionIDs[alertID] {
			continue
		}
		suppressedRows = append(suppressedRows, suppressedAlertRow{
			Alert: h.alertFromSuppressionID(ctx, alertID),
			Until: until,
		})
	}

	serverNames := uniqueServerNames(active)

	const alertHistoryPageSize = 25
	historyPage, _ := strconv.Atoi(r.URL.Query().Get("history_page"))
	if historyPage < 1 {
		historyPage = 1
	}
	totalEvents, _ := h.store.CountAlertStateEvents(ctx)
	historyQuery := "severity=" + url.QueryEscape(filterSeverity) + "&server=" + url.QueryEscape(filterServer)
	historyPagination := buildPagination(historyPage, totalEvents, alertHistoryPageSize, historyQuery)
	events, _ := h.store.ListAlertStateEventsPage(ctx, alertHistoryPageSize, (historyPagination.Page-1)*alertHistoryPageSize)

	h.tmpl.Render(w, r, "alerts.html", map[string]any{
		"Username":          username,
		"Role":              role,
		"AlertGroups":       groupAlertsByServer(filtered),
		"Suppressed":        suppressedRows,
		"SuppressedGroups":  groupSuppressedByServer(suppressedRows),
		"AlertCritical":     critical,
		"AlertWarning":      warning,
		"FilterSeverity":    filterSeverity,
		"FilterServer":      filterServer,
		"ServerNames":       serverNames,
		"AlertEvents":       events,
		"HistoryPage":       historyPagination.Page,
		"HistoryPrevPage":   historyPagination.PrevPage,
		"HistoryNextPage":   historyPagination.NextPage,
		"HistoryHasPrev":    historyPagination.HasPrev,
		"HistoryHasNext":    historyPagination.HasNext,
		"HistoryPagination": historyPagination,
	})
}

func (h *WebH) AlertsStatus(w http.ResponseWriter, r *http.Request) {
	active, critical, warning, err := h.activeAlerts(r.Context())
	if err != nil {
		slog.Error("load alert status", "err", err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "error interno del servidor"})
		return
	}

	resp := alertStatusResponse{
		Critical:    critical,
		Warning:     warning,
		GeneratedAt: time.Now().Unix(),
	}
	for _, a := range active {
		resp.Alerts = append(resp.Alerts, alertStatusItem{
			ID:         a.ID,
			Severity:   a.Severity,
			Title:      a.Title,
			Message:    a.Message,
			ServerName: a.ServerName,
			ServerType: a.ServerType,
			ServerID:   a.ServerID,
			Type:       a.Type,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (h *WebH) AlertDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	alertID := strings.TrimSpace(r.URL.Query().Get("alert_id"))
	if alertID == "" {
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}

	allAlerts, err := h.runAlerts(ctx, false)
	if err != nil {
		slog.Error("run alerts detail", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	alertByID := make(map[string]domain.Alert, len(allAlerts))
	for _, alert := range allAlerts {
		alertByID[alert.ID] = alert
	}

	alert := alertByID[alertID]
	isPresent := alert.ID != ""
	if !isPresent {
		if info, err := h.store.GetAlertStateEventInfo(ctx, alertID); err == nil {
			alert = alertFromEventInfo(info)
		}
	}
	if alert.ID == "" {
		alert = h.alertFromSuppressionID(ctx, alertID)
	}

	suppression, err := h.store.GetAlertSuppression(ctx, alertID)
	isSuppressed := err == nil && suppression.Active
	if err != nil && err != sql.ErrNoRows {
		slog.Warn("load alert suppression detail", "err", err)
	}

	events, _ := h.store.ListAlertStateEventsForAlert(ctx, alertID, 100)
	if !isPresent && !isSuppressed && len(events) == 0 {
		http.NotFound(w, r)
		return
	}

	statusLabel := "Resuelta"
	statusClass := "ok"
	if isSuppressed {
		statusLabel = "Suprimida"
		statusClass = "muted"
	} else if isPresent {
		statusLabel = "Activa"
		statusClass = "bad"
		if alert.Severity == domain.AlertSeverityWarning {
			statusClass = "warn"
		}
	}

	h.tmpl.Render(w, r, "alert_detail.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"Alert":           alert,
		"AlertID":         alertID,
		"IsPresent":       isPresent,
		"IsSuppressed":    isSuppressed,
		"Suppression":     suppression,
		"Events":          events,
		"StatusLabel":     statusLabel,
		"StatusClass":     statusClass,
		"ServerDetailURL": alertServerDetailURL(alert),
	})
}

func alertFromEventInfo(ev domain.AlertStateEvent) domain.Alert {
	return domain.Alert{
		ID:         ev.AlertID,
		ServerName: ev.ServerName,
		ServerID:   ev.ServerID,
		ServerType: ev.ServerType,
		StoreName:  ev.StoreName,
		VMID:       ev.VMID,
		VMName:     ev.VMName,
		Type:       strings.Split(ev.AlertID, ":")[0],
		Severity:   ev.Severity,
		Title:      ev.Title,
		Message:    ev.Message,
	}
}

func alertServerDetailURL(alert domain.Alert) string {
	if alert.ServerType == "" || alert.ServerID <= 0 {
		return ""
	}
	return "/servers/" + alert.ServerType + "/" + strconv.FormatInt(alert.ServerID, 10)
}

func (h *WebH) activeAlerts(ctx context.Context) ([]domain.Alert, int, int, error) {
	allAlerts, err := h.runAlerts(ctx, true)
	if err != nil {
		return nil, 0, 0, err
	}
	suppressions, _ := h.store.GetActiveSuppressions(ctx)

	var active []domain.Alert
	var critical, warning int
	for _, a := range allAlerts {
		if _, ok := suppressions[a.ID]; ok {
			continue
		}
		active = append(active, a)
		if a.Severity == domain.AlertSeverityCritical {
			critical++
		} else {
			warning++
		}
	}
	return active, critical, warning, nil
}

func (h *WebH) runAlerts(ctx context.Context, syncState bool) ([]domain.Alert, error) {
	cfg, err := service.LoadAlertConfigs(ctx, h.store)
	if err != nil {
		return nil, err
	}
	cfg.Report = h.report
	allAlerts, err := service.RunAll(h.store, cfg)
	if err != nil {
		return nil, err
	}
	if syncState {
		_ = h.store.SyncAlertStates(ctx, allAlerts)
	}
	return allAlerts, nil
}

func (h *WebH) AlertSuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertIDs := formAlertIDs(r)
	days, _ := strconv.Atoi(r.FormValue("days"))
	reason := r.FormValue("reason")
	if len(alertIDs) == 0 || days <= 0 {
		http.Redirect(w, r, alertRedirectBack(r), http.StatusSeeOther)
		return
	}
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	current := h.alertMap(ctx)
	for _, alertID := range alertIDs {
		_ = h.store.UpsertAlertSuppression(ctx, alertID, until, reason)
		alert := current[alertID]
		if alert.ID == "" {
			alert = h.alertFromSuppressionID(ctx, alertID)
		}
		_ = h.store.InsertAlertStateEvent(ctx, alertStateEventFromAlert(alert, "suppressed", reason))
	}
	http.Redirect(w, r, alertRedirectBack(r), http.StatusSeeOther)
}

func (h *WebH) AlertUnsuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	current := h.alertMap(ctx)
	for _, alertID := range formAlertIDs(r) {
		_ = h.store.DeleteAlertSuppression(ctx, alertID)
		alert := current[alertID]
		if alert.ID == "" {
			alert = h.alertFromSuppressionID(ctx, alertID)
		}
		_ = h.store.InsertAlertStateEvent(ctx, alertStateEventFromAlert(alert, "unsuppressed", ""))
	}
	http.Redirect(w, r, alertRedirectBack(r), http.StatusSeeOther)
}

func alertRedirectBack(r *http.Request) string {
	back := strings.TrimSpace(r.FormValue("back"))
	if back == "" || !strings.HasPrefix(back, "/") || strings.HasPrefix(back, "//") || strings.Contains(back, "\n") || strings.Contains(back, "\r") {
		return "/alerts"
	}
	return back
}

func (h *WebH) alertMap(ctx context.Context) map[string]domain.Alert {
	allAlerts, _ := h.runAlerts(ctx, false)
	out := make(map[string]domain.Alert, len(allAlerts))
	for _, alert := range allAlerts {
		out[alert.ID] = alert
	}
	return out
}

func alertStateEventFromAlert(alert domain.Alert, eventType, note string) domain.AlertStateEvent {
	return domain.AlertStateEvent{
		AlertID: alert.ID, EventType: eventType, Severity: alert.Severity, Title: alert.Title,
		Message: alert.Message, ServerName: alert.ServerName, ServerType: alert.ServerType,
		ServerID: alert.ServerID, StoreName: alert.StoreName, VMID: alert.VMID, VMName: alert.VMName, Note: note,
	}
}

func formAlertIDs(r *http.Request) []string {
	_ = r.ParseForm()
	seen := make(map[string]bool)
	var ids []string
	for _, raw := range r.Form["alert_id"] {
		for _, part := range strings.Split(raw, ",") {
			id := strings.TrimSpace(part)
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			ids = append(ids, id)
		}
	}
	return ids
}

func (h *WebH) alertFromSuppressionID(ctx context.Context, alertID string) domain.Alert {
	a := domain.Alert{
		ID:       alertID,
		Severity: domain.AlertSeverityWarning,
		Title:    alertTitleFromID(alertID),
		Message:  "Supresion activa guardada",
	}
	parts := strings.Split(alertID, ":")
	if len(parts) >= 3 {
		a.Type = parts[0]
		a.ServerType = parts[1]
		a.ServerID, _ = strconv.ParseInt(parts[2], 10, 64)
	}
	if len(parts) >= 4 {
		switch a.Type {
		case domain.AlertTypeDisk, domain.AlertTypePBSFill, domain.AlertTypePBSStale, domain.AlertTypePBSVerify:
			a.StoreName = parts[3]
		default:
			if vmid, err := strconv.ParseInt(parts[3], 10, 64); err == nil {
				a.VMID = vmid
				a.VMName = parts[3]
			}
		}
	}
	switch a.ServerType {
	case "pve":
		if sv, err := h.store.GetPVEServer(ctx, a.ServerID); err == nil {
			a.ServerName = sv.DisplayName
		}
	case "pbs":
		if sv, err := h.store.GetPBSServer(ctx, a.ServerID); err == nil {
			a.ServerName = sv.DisplayName
		}
	case "windows":
		if sv, err := h.store.GetWindowsServer(ctx, a.ServerID); err == nil {
			a.ServerName = sv.DisplayName
		}
	}
	if a.ServerName == "" {
		a.ServerName = a.ServerType + " " + strconv.FormatInt(a.ServerID, 10)
	}
	return a
}

func alertTitleFromID(alertID string) string {
	switch strings.Split(alertID, ":")[0] {
	case domain.AlertTypeDisk:
		return "Disco casi lleno"
	case domain.AlertTypeBackupError:
		return "Backup con error"
	case domain.AlertTypeBackupSize:
		return "Backup demasiado pequeño"
	case domain.AlertTypePBSFill:
		return "PBS casi lleno"
	case domain.AlertTypePBSStale:
		return "Snapshot sin actualizar"
	case domain.AlertTypePBSVerify:
		return "Verificacion fallida"
	case domain.AlertTypePVEStale:
		return "PVE sin reporte"
	case domain.AlertTypePBSReportStale:
		return "PBS sin reporte"
	case domain.AlertTypePVEHeartbeat:
		return "Servidor offline"
	case domain.AlertTypePVEMissingVM:
		return "VM sin backup"
	case domain.AlertTypePVEUnknownVM:
		return "VM no configurada"
	case domain.AlertTypeSwap:
		return "Swap activa"
	case domain.AlertTypeWindowsHeartbeat:
		return "Servidor offline"
	case domain.AlertTypeWindowsDiskHealth:
		return "Salud de disco"
	case domain.AlertTypeWindowsVolumeGone:
		return "Volumen no detectado"
	default:
		return "Alerta suprimida"
	}
}

func uniqueServerNames(alerts []domain.Alert) []string {
	seen := make(map[string]bool)
	var out []string
	for _, a := range alerts {
		if !seen[a.ServerName] {
			seen[a.ServerName] = true
			out = append(out, a.ServerName)
		}
	}
	return out
}

func groupAlertsByServer(alerts []domain.Alert) []alertGroup {
	index := make(map[string]int)
	var groups []alertGroup
	for _, a := range alerts {
		key := a.ServerType + ":" + strconv.FormatInt(a.ServerID, 10) + ":" + a.ServerName
		i, ok := index[key]
		if !ok {
			groups = append(groups, alertGroup{
				ServerName: a.ServerName,
				ServerType: a.ServerType,
				ServerID:   a.ServerID,
			})
			i = len(groups) - 1
			index[key] = i
		}
		groups[i].Alerts = append(groups[i].Alerts, a)
		if a.Severity == domain.AlertSeverityCritical {
			groups[i].Critical++
		} else {
			groups[i].Warning++
		}
	}
	return groups
}

func groupSuppressedByServer(rows []suppressedAlertRow) []suppressedAlertGroup {
	index := make(map[string]int)
	var groups []suppressedAlertGroup
	for _, row := range rows {
		a := row.Alert
		key := a.ServerType + ":" + strconv.FormatInt(a.ServerID, 10) + ":" + a.ServerName
		i, ok := index[key]
		if !ok {
			groups = append(groups, suppressedAlertGroup{
				ServerName: a.ServerName,
				ServerType: a.ServerType,
				ServerID:   a.ServerID,
			})
			i = len(groups) - 1
			index[key] = i
		}
		groups[i].Rows = append(groups[i].Rows, row)
	}
	return groups
}

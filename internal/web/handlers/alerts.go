package webhandlers

import (
	"context"
	"log/slog"
	"net/http"
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

func (h *WebH) Alerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	cfg, err := service.LoadAlertConfigs(ctx, h.store)
	if err != nil {
		slog.Error("load alert configs", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	allAlerts, err := service.RunAll(h.store, cfg)
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

	h.tmpl.Render(w, r, "alerts.html", map[string]any{
		"Username":         username,
		"Role":             role,
		"AlertGroups":      groupAlertsByServer(filtered),
		"Suppressed":       suppressedRows,
		"SuppressedGroups": groupSuppressedByServer(suppressedRows),
		"AlertCritical":    critical,
		"AlertWarning":     warning,
		"FilterSeverity":   filterSeverity,
		"FilterServer":     filterServer,
		"ServerNames":      serverNames,
	})
}

func (h *WebH) AlertSuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertIDs := formAlertIDs(r)
	days, _ := strconv.Atoi(r.FormValue("days"))
	reason := r.FormValue("reason")
	if len(alertIDs) == 0 || days <= 0 {
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	for _, alertID := range alertIDs {
		_ = h.store.UpsertAlertSuppression(ctx, alertID, until, reason)
	}
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

func (h *WebH) AlertUnsuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	for _, alertID := range formAlertIDs(r) {
		_ = h.store.DeleteAlertSuppression(ctx, alertID)
	}
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
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
			a.ServerName = sv.Name
		}
	case "pbs":
		if sv, err := h.store.GetPBSServer(ctx, a.ServerID); err == nil {
			a.ServerName = sv.Name
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
	case domain.AlertTypePVEMissingVM:
		return "VM sin backup"
	case domain.AlertTypePVEUnknownVM:
		return "VM no configurada"
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

package webhandlers

import (
	"net/http"
	"strconv"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

func (h *WebH) Alerts(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)

	cfg, err := service.LoadAlertConfigs(h.store)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	allAlerts, err := service.RunAll(h.store, cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	suppressions, _ := h.store.GetActiveSuppressions(ctx)

	var active, suppressed []domain.Alert
	for _, a := range allAlerts {
		if _, ok := suppressions[a.ID]; ok {
			suppressed = append(suppressed, a)
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

	// Build suppressed list with until times for template
	var suppressedRows []map[string]any
	for _, a := range suppressed {
		suppressedRows = append(suppressedRows, map[string]any{
			"Alert": a,
			"Until": suppressions[a.ID],
		})
	}

	serverNames := uniqueServerNames(active)

	h.tmpl.Render(w, r, "alerts.html", map[string]any{
		"Username":       username,
		"Role":           role,
		"Alerts":         filtered,
		"Suppressed":     suppressedRows,
		"AlertCritical":  critical,
		"AlertWarning":   warning,
		"FilterSeverity": filterSeverity,
		"FilterServer":   filterServer,
		"ServerNames":    serverNames,
	})
}

func (h *WebH) AlertSuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertID := r.FormValue("alert_id")
	days, _ := strconv.Atoi(r.FormValue("days"))
	reason := r.FormValue("reason")
	if alertID == "" || days <= 0 {
		http.Redirect(w, r, "/alerts", http.StatusSeeOther)
		return
	}
	until := time.Now().Add(time.Duration(days) * 24 * time.Hour)
	_ = h.store.UpsertAlertSuppression(ctx, alertID, until, reason)
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
}

func (h *WebH) AlertUnsuppressPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	alertID := r.FormValue("alert_id")
	if alertID != "" {
		_ = h.store.DeleteAlertSuppression(ctx, alertID)
	}
	http.Redirect(w, r, "/alerts", http.StatusSeeOther)
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

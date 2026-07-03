package webhandlers

import (
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/session"
)

func TestTemplatesRenderWithRepresentativeData(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() bool { return false })
	fixtures := templateFixtures(time.Date(2026, 5, 17, 10, 0, 0, 0, time.UTC))

	entries, err := os.ReadDir("../../../web/templates")
	if err != nil {
		t.Fatalf("read templates: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "base.html" {
			continue
		}
		if _, ok := fixtures[entry.Name()]; !ok {
			t.Fatalf("missing render fixture for %s", entry.Name())
		}
	}

	for name, data := range fixtures {
		t.Run(name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			rr := httptest.NewRecorder()

			tmpl.Render(rr, req, name, data)

			res := rr.Result()
			body := rr.Body.String()
			if res.StatusCode != http.StatusOK {
				t.Fatalf("render status %d, body:\n%s", res.StatusCode, body)
			}
			if strings.Contains(body, "Error renderizando plantilla") {
				t.Fatalf("template error fallback rendered:\n%s", body)
			}
		})
	}
}

func TestTemplatesRenderFlashFromQuery(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() bool { return false })
	req := httptest.NewRequest(http.MethodGet, "/about?flash=Mensaje+visible&ok=1", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "about.html", templateFixtures(time.Now())["about.html"])

	body := rr.Body.String()
	if !strings.Contains(body, "Mensaje visible") {
		t.Fatalf("flash message not rendered:\n%s", body)
	}
	if !strings.Contains(body, "alert-success") {
		t.Fatalf("success flash style not rendered:\n%s", body)
	}
}

func TestProfile2FASetupAllowsQRDataURI(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() bool { return false })
	req := httptest.NewRequest(http.MethodGet, "/profile/2fa/setup", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "profile_2fa_setup.html", templateFixtures(time.Now())["profile_2fa_setup.html"])

	body := rr.Body.String()
	if !strings.Contains(body, `src="data:image/png;base64,test"`) {
		t.Fatalf("qr data uri was not rendered safely:\n%s", body)
	}
	if strings.Contains(body, "#ZgotmplZ") {
		t.Fatalf("qr data uri was blocked by html/template:\n%s", body)
	}
}

func templateFixtures(now time.Time) map[string]map[string]any {
	base := func(extra map[string]any) map[string]any {
		data := map[string]any{
			"Username":      "admin",
			"Role":          "admin",
			"Active":        "",
			"Version":       "test",
			"AlertCritical": 0,
			"AlertWarning":  0,
			"CSRFField":     template.HTML(""),
			"CSRFToken":     "test-csrf",
		}
		for k, v := range extra {
			data[k] = v
		}
		return data
	}

	pveServer := domain.PVEServer{ID: 1, Name: "pve-1", IP: "10.0.0.1", PublicIP: "203.0.113.10", ClientVersion: "test"}
	pbsServer := domain.PBSServer{ID: 2, Name: "pbs-1", IP: "10.0.0.2", PublicIP: "203.0.113.11", ClientVersion: "test"}
	windowsServer := domain.WindowsServer{ID: 3, Name: "win-1", DisplayName: "win-1", IP: "10.0.0.3", PublicIP: "203.0.113.12", ClientVersion: "test"}
	pagination := paginationView{Page: 1, TotalPages: 1, TotalItems: 0, PageSize: reportHistoryPageSize, Pages: []int{1}}
	emailConfig := domain.EmailConfig{
		SMTPHost:                 "smtp.example.test",
		SMTPPort:                 587,
		SMTPUser:                 "admin@example.test",
		Recipients:               "ops@example.test",
		IsEnabled:                true,
		SendTime:                 "09:00",
		RetentionMonths:          6,
		RetentionEnabled:         true,
		AlertDiskPct:             85,
		AlertWindowsDiskPct:      90,
		AlertBackupErr:           true,
		AlertPBSStaleHours:       36,
		AlertPVEHeartbeatMinutes: 15,
	}

	return map[string]map[string]any{
		"about.html": base(map[string]any{
			"Uptime":       "1h",
			"StartTime":    now,
			"DBSize":       int64(1024),
			"PVECount":     1,
			"PBSCount":     1,
			"WindowsCount": 1,
		}),
		"alerts.html": base(map[string]any{
			"AlertGroups": []alertGroup{},
			"Suppressed": []struct {
				Alert domain.Alert
				Until time.Time
			}{},
			"SuppressedGroups":  []suppressedAlertGroup{},
			"ServerNames":       []string{"pve-1"},
			"FilterSeverity":    "",
			"FilterServer":      "",
			"AlertEvents":       []domain.AlertStateEvent{{EventType: "appeared", ServerName: "pve-1", Title: "Sin reporte", Message: "test", CreatedAt: now}},
			"HistoryPage":       1,
			"HistoryPrevPage":   0,
			"HistoryNextPage":   2,
			"HistoryHasPrev":    false,
			"HistoryHasNext":    true,
			"HistoryPagination": buildPagination(1, 1, 25, "severity=&server="),
		}),
		"alert_detail.html": base(map[string]any{
			"Alert": domain.Alert{
				ID:         "disk:pve:1:local",
				ServerName: "pve-1",
				ServerType: "pve",
				ServerID:   1,
				StoreName:  "local",
				Severity:   domain.AlertSeverityWarning,
				Title:      "Disco casi lleno",
				Message:    "90% usado",
				Value:      "90%",
				Threshold:  "85%",
			},
			"AlertID":         "disk:pve:1:local",
			"StatusLabel":     "Activa",
			"StatusClass":     "warn",
			"ServerDetailURL": "/servers/pve/1",
			"Events":          []domain.AlertStateEvent{{AlertID: "disk:pve:1:local", EventType: "appeared", Message: "90% usado", CreatedAt: now}},
		}),
		"alerts_settings.html": base(map[string]any{"Config": emailConfig}),
		"api_key_created.html": base(map[string]any{
			"Name":        "cliente-pve",
			"Key":         "pbk-1234567890abcdef",
			"APIURL":      "http://probakgo.test:36748",
			"GitHubToken": "",
		}),
		"api_key_edit.html": base(map[string]any{
			"Key": domain.APIKey{ID: 1, Name: "cliente-pve", Key: "pbk-1234567890abcdef", ServerName: "pve-1", ServerURL: "https://10.0.0.1:8006"},
		}),
		"api_keys.html": base(map[string]any{
			"Keys":               []map[string]any{},
			"SearchQuery":        "",
			"SearchQueryEscaped": "",
			"KeysPage":           1,
			"KeysPrevPage":       0,
			"KeysNextPage":       2,
			"KeysHasPrev":        false,
			"KeysHasNext":        false,
		}),
		"audit_log.html": base(map[string]any{
			"Rows":          []domain.AuditLog{},
			"Users":         []domain.User{},
			"AuditPage":     1,
			"AuditPrevPage": 0,
			"AuditNextPage": 2,
			"AuditHasPrev":  false,
			"AuditHasNext":  false,
		}),
		"backup_config.html": base(map[string]any{
			"ServerName": "pve-1",
			"Configs":    []domain.VMBackupConfig{},
		}),
		"dashboard.html": base(map[string]any{
			"PVEOk":             1,
			"PVEBackupErrors":   0,
			"PVEStale":          0,
			"PBSOk":             1,
			"PBSStale":          0,
			"WindowsOK":         1,
			"WindowsDiskAlerts": 0,
			"WindowsOffline":    0,
			"PVERows":           []map[string]any{},
			"PBSRows":           []map[string]any{},
			"WindowsRows":       []map[string]any{},
		}),
		"email_settings.html": base(map[string]any{"Config": emailConfig}),
		"ip_bans.html": base(map[string]any{
			"Bans":             []map[string]any{},
			"LoginAttempts":    []domain.LoginAttempt{},
			"BansPage":         1,
			"BansPrevPage":     0,
			"BansNextPage":     2,
			"BansHasPrev":      false,
			"BansHasNext":      false,
			"AttemptsPage":     1,
			"AttemptsPrevPage": 0,
			"AttemptsNextPage": 2,
			"AttemptsHasPrev":  false,
			"AttemptsHasNext":  false,
		}),
		"login.html":                base(map[string]any{"Error": ""}),
		"login_2fa.html":            base(map[string]any{"Error": ""}),
		"maintenance_settings.html": base(map[string]any{"Config": emailConfig}),
		"profile.html": base(map[string]any{
			"User": domain.User{ID: 1, Username: "admin", Role: "admin", IsActive: true, CreatedAt: now},
		}),
		"profile_2fa_setup.html": base(map[string]any{
			"Secret":    "JBSWY3DPEHPK3PXP",
			"URI":       "otpauth://totp/Probakgo:admin?secret=JBSWY3DPEHPK3PXP&issuer=Probakgo",
			"QRDataURI": template.URL("data:image/png;base64,test"),
		}),
		"reports_pve.html": base(map[string]any{
			"Server":       pveServer,
			"Days":         30,
			"Reports":      []domain.PVEReport{},
			"Pagination":   pagination,
			"Storages":     []map[string]any{},
			"TotalBackups": 0,
			"ChartData":    []map[string]any{},
		}),
		"reset_settings.html": base(map[string]any{}),
		"server_pbs_detail.html": base(map[string]any{
			"Server":     pbsServer,
			"Stores":     []map[string]any{},
			"Reports":    []domain.PBSReport{},
			"Pagination": pagination,
		}),
		"server_pve_detail.html": base(map[string]any{
			"Server":          pveServer,
			"BackupTasks":     []domain.PVEBackupTask{},
			"BackupRows":      []pveBackupJobRow{},
			"BackupJobStart":  int64(0),
			"Heartbeat":       heartbeatView{Label: "Sin datos", CSSClass: "muted"},
			"MissingVMs":      []map[string]any{},
			"ConfiguredVMIDs": map[string]bool{},
			"VMAlertConfigs":  map[int64]domain.PVEVMAlertConfig{},
			"Storages":        []map[string]any{},
			"JobHistory":      []map[string]any{},
			"Reports":         []domain.PVEReport{},
			"Pagination":      pagination,
		}),
		"server_windows_detail.html": base(map[string]any{
			"Server":    windowsServer,
			"Role":      "admin",
			"Heartbeat": heartbeatView{Seen: true, Online: true, Label: "Online", CSSClass: "ok", LastSeen: now},
			"Disks": []windowsDiskDisplay{{
				WindowsDisk: domain.WindowsDisk{Name: "C:", Label: "System", FileSystem: "NTFS", Total: 1000, Used: 500, Free: 500},
				UsedPct:     50,
				BadgeClass:  "ok",
				BadgeLabel:  "50%",
				Title:       "test",
			}},
			"Reports":       []domain.WindowsReport{{ID: 1, ServerID: 3, ReportedAt: now}},
			"Pagination":    pagination,
			"DiskChartData": []windowsDiskChartPoint{{Label: "17/05 10:00", Disk: "C:", UsedPct: 50, Used: 500, Total: 1000}},
			"DiskChartDays": 30,
			"AlertConfig":   domain.WindowsAlertConfig{ServerID: 3},
			"AlertControls": []windowsAlertControl{{ID: "windows_heartbeat:windows:3", Title: "Conexión", Detail: "Servidor Windows sin heartbeat"}},
			"AllAlertIDs":   "windows_heartbeat:windows:3",
			"BackURL":       "/servers/windows/3",
		}),
		"servers_pbs.html": base(map[string]any{
			"Rows":          []map[string]any{},
			"HealthSummary": serverListHealthSummary{},
		}),
		"servers_pve.html": base(map[string]any{
			"Rows":          []map[string]any{},
			"HealthSummary": serverListHealthSummary{},
		}),
		"servers_windows.html": base(map[string]any{
			"Rows":          []map[string]any{},
			"HealthSummary": serverListHealthSummary{},
		}),
		"settings_hub.html": base(map[string]any{
			"Config":   emailConfig,
			"BanCount": 0,
		}),
		"system_settings.html": base(map[string]any{"Config": emailConfig}),
		"users.html": base(map[string]any{
			"Users":           []domain.User{},
			"CurrentUsername": "admin",
		}),
		"vm_backup_config_form.html": base(map[string]any{
			"ServerName": "pve-1",
			"Action":     "new",
			"VM":         (*domain.VMBackupConfig)(nil),
		}),
	}
}

package webhandlers

import (
	"html"
	"html/template"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/web/csp"
)

func TestTemplatesRenderWithRepresentativeData(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
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

func TestProxmoxServerTablesSortEveryDataColumn(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	fixtures := templateFixtures(time.Now())

	for _, tc := range []struct {
		name     string
		path     string
		template string
		columns  int
	}{
		{name: "PVE", path: "/servers/pve", template: "servers_pve.html", columns: 8},
		{name: "PBS", path: "/servers/pbs", template: "servers_pbs.html", columns: 7},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			tmpl.Render(rr, req, tc.template, fixtures[tc.template])
			body := rr.Body.String()

			if got := strings.Count(body, `<th data-sort="`); got != tc.columns {
				t.Fatalf("sortable columns = %d, want %d", got, tc.columns)
			}
			if !strings.Contains(body, "dataset.sortValue") || !strings.Contains(body, "data-sort-type=\"number\"") {
				t.Fatal("table is missing typed stable sort values")
			}
		})
	}
}

func TestServerListsExposeExcelReports(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	fixtures := templateFixtures(time.Now())

	for _, tc := range []struct {
		path     string
		template string
		href     string
	}{
		{path: "/servers/pve", template: "servers_pve.html", href: `/servers/pve.xlsx`},
		{path: "/servers/pbs", template: "servers_pbs.html", href: `/servers/pbs.xlsx`},
		{path: "/servers/windows", template: "servers_windows.html", href: `/servers/windows.xlsx`},
	} {
		req := httptest.NewRequest(http.MethodGet, tc.path, nil)
		rr := httptest.NewRecorder()
		tmpl.Render(rr, req, tc.template, fixtures[tc.template])
		if body := rr.Body.String(); !strings.Contains(body, `href="`+tc.href+`"`) || !strings.Contains(body, "Informe Excel") {
			t.Fatalf("%s is missing its Excel report action", tc.path)
		}
	}
}

func TestTemplatesRenderFlashFromQuery(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
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

func TestTemplatesApplyRequestCSPNonce(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	req, nonce, err := csp.WithNonce(req)
	if err != nil {
		t.Fatalf("WithNonce: %v", err)
	}
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "about.html", templateFixtures(time.Now())["about.html"])

	body := html.UnescapeString(rr.Body.String())
	if !strings.Contains(body, `nonce="`+nonce+`"`) {
		t.Fatal("rendered scripts do not carry the request CSP nonce")
	}
}

func TestTemplateDebugDataRedactsSecrets(t *testing.T) {
	got := safeTemplateDebugData(map[string]any{
		"Key":         "pbk-secret",
		"GitHubToken": "github-secret",
		"Config": domain.EmailConfig{
			SMTPHost: "smtp.example.test",
			SMTPPass: "smtp-secret",
		},
		"Safe": "visible",
	})

	for _, secret := range []string{"pbk-secret", "github-secret", "smtp-secret"} {
		if strings.Contains(got, secret) {
			t.Fatalf("debug data contains secret %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, `"Safe": "visible"`) || !strings.Contains(got, "[REDACTED]") {
		t.Fatalf("debug data lost safe fields or redaction marker: %s", got)
	}
}

func TestAboutUpdateSkipsSensitiveTOTPPrompt(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "about.html", templateFixtures(time.Now())["about.html"])

	body := rr.Body.String()
	if !strings.Contains(body, `action="/about/update"`) {
		t.Fatalf("update form is missing:\n%s", body)
	}
	if !strings.Contains(body, `action="/about/update" data-totp-skip`) {
		t.Fatalf("update form does not bypass the sensitive TOTP prompt:\n%s", body)
	}
}

func TestAlertSuppressionSkipsSensitiveTOTPPrompt(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "alerts.html", templateFixtures(time.Now())["alerts.html"])

	body := rr.Body.String()
	if !strings.Contains(body, `action="/alerts/suppress" data-totp-skip`) {
		t.Fatalf("suppression form is missing:\n%s", body)
	}
	if !strings.Contains(body, `<option value="12" selected>12 horas</option>`) {
		t.Fatalf("12-hour suppression is not the default:\n%s", body)
	}
	if !strings.Contains(body, "Suprimir todas las alertas de este servidor") {
		t.Fatalf("server-wide suppression action is missing:\n%s", body)
	}
}

func TestTelegramProfilePairingSkipsSensitiveTOTPPromptAndShowsMobileQR(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/profile", nil)
	rr := httptest.NewRecorder()
	tmpl.Render(rr, req, "profile.html", templateFixtures(time.Now())["profile.html"])

	body := rr.Body.String()
	for _, action := range []string{"/profile/telegram/pair", "/profile/telegram/test", "/profile/telegram/delete"} {
		if strings.Contains(body, `action="`+action+`"`) && !strings.Contains(body, `action="`+action+`" data-totp-skip`) {
			t.Fatalf("Telegram form %s does not bypass the sensitive TOTP prompt:\n%s", action, body)
		}
	}
	if !strings.Contains(body, `alt="QR para abrir el bot de Telegram en el movil"`) {
		t.Fatalf("Telegram mobile pairing QR is missing:\n%s", body)
	}
}

func TestProfileAndSettingsUseConsistentUXStructure(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	fixtures := templateFixtures(time.Now())

	profileReq := httptest.NewRequest(http.MethodGet, "/profile", nil)
	profileRR := httptest.NewRecorder()
	tmpl.Render(profileRR, profileReq, "profile.html", fixtures["profile.html"])
	profileBody := profileRR.Body.String()
	for _, want := range []string{"Cuenta", "Seguridad", "Mis notificaciones", "profile-channel-heading"} {
		if !strings.Contains(profileBody, want) {
			t.Fatalf("profile is missing UX block %q:\n%s", want, profileBody)
		}
	}

	settingsReq := httptest.NewRequest(http.MethodGet, "/settings", nil)
	settingsRR := httptest.NewRecorder()
	tmpl.Render(settingsRR, settingsReq, "settings_hub.html", fixtures["settings_hub.html"])
	settingsBody := settingsRR.Body.String()
	for _, want := range []string{"General y comunicaciones", "Operación y control", "Zona peligrosa"} {
		if !strings.Contains(settingsBody, want) {
			t.Fatalf("settings hub is missing group %q:\n%s", want, settingsBody)
		}
	}
	for className, want := range map[string]int{
		"settings-hub-card-header": 8,
		"settings-hub-card-body":   8,
		"settings-hub-card-footer": 8,
	} {
		if got := strings.Count(settingsBody, className); got != want {
			t.Fatalf("settings hub %s count = %d, want %d", className, got, want)
		}
	}

	alertsReq := httptest.NewRequest(http.MethodGet, "/settings/alerts", nil)
	alertsRR := httptest.NewRecorder()
	tmpl.Render(alertsRR, alertsReq, "alerts_settings.html", fixtures["alerts_settings.html"])
	alertsBody := alertsRR.Body.String()
	for _, want := range []string{"Almacenamiento y backups", "Conexión y reportes PVE"} {
		if !strings.Contains(alertsBody, want) {
			t.Fatalf("alerts settings is missing card %q:\n%s", want, alertsBody)
		}
	}
	if got := strings.Count(alertsBody, "alert-settings-card"); got != 2 {
		t.Fatalf("alerts settings card count = %d, want 2", got)
	}
}

func TestPageLevelDestructiveActionsUseDangerZones(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	fixtures := templateFixtures(time.Now())

	for _, tc := range []struct {
		name       string
		path       string
		template   string
		zoneID     string
		actionPath string
	}{
		{name: "Telegram configuration", path: "/settings/telegram", template: "telegram_settings.html", zoneID: "telegramDangerZoneTitle", actionPath: "/settings/telegram/delete"},
		{name: "user account", path: "/users/1/edit", template: "user_edit.html", zoneID: "userDangerZoneTitle", actionPath: "/users/1/delete"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.path, nil)
			rr := httptest.NewRecorder()
			tmpl.Render(rr, req, tc.template, fixtures[tc.template])

			body := rr.Body.String()
			zoneAt := strings.Index(body, `id="`+tc.zoneID+`"`)
			actionAt := strings.Index(body, `action="`+tc.actionPath+`"`)
			if zoneAt < 0 || actionAt < 0 || zoneAt > actionAt {
				t.Fatalf("%s destructive action is not inside a preceding danger zone:\n%s", tc.name, body)
			}
		})
	}
}

func TestSensitiveTOTPPromptChecksExpiryAtSubmitTime(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	rr := httptest.NewRecorder()
	tmpl.Render(rr, req, "servers_pve.html", templateFixtures(time.Now())["servers_pve.html"])

	body := rr.Body.String()
	if !strings.Contains(body, "const sensitiveTOTPValidUntil =") || !strings.Contains(body, "Date.now() < sensitiveTOTPValidUntil") {
		t.Fatalf("sensitive-action prompt does not check live expiry:\n%s", body)
	}
	if strings.Contains(body, "if (true) return;") {
		t.Fatalf("sensitive-action prompt still relies on a stale render-time boolean:\n%s", body)
	}
}

func TestAlertNotificationsLinkDirectlyToServer(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	req := httptest.NewRequest(http.MethodGet, "/about", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "about.html", templateFixtures(time.Now())["about.html"])

	body := rr.Body.String()
	for _, want := range []string{
		`function alertServerURL(alert)`,
		`el.querySelector('.alert-toast-server').href = alertServerURL(alert)`,
		`window.location.href = alertServerURL(alert)`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("alert notification missing direct server navigation %q", want)
		}
	}
}

func TestAlertsShowExternalProxmoxLink(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	req := httptest.NewRequest(http.MethodGet, "/alerts", nil)
	rr := httptest.NewRecorder()

	tmpl.Render(rr, req, "alerts.html", templateFixtures(time.Now())["alerts.html"])

	body := rr.Body.String()
	if !strings.Contains(body, `href="https://pve.example.test:8006"`) || !strings.Contains(body, "Abrir Proxmox") {
		t.Fatalf("alerts page missing external Proxmox link:\n%s", body)
	}
}

func TestProfile2FASetupAllowsQRDataURI(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
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

func TestAPIKeysRevealRequestsTOTPWhenRequired(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	rr := httptest.NewRecorder()
	data := templateFixtures(time.Now())["api_keys.html"]
	data["UserTOTPEnabled"] = true
	data["SensitiveTOTPFresh"] = false

	tmpl.Render(rr, req, "api_keys.html", data)

	body := rr.Body.String()
	if !strings.Contains(body, `id="revealTOTP"`) {
		t.Fatalf("API key reveal does not request the TOTP code:\n%s", body)
	}
	if !strings.Contains(body, `fd.append('totp_code', totpInput.value.trim())`) {
		t.Fatalf("API key reveal does not submit the TOTP code:\n%s", body)
	}
}

func TestAPIKeysRevealExplainsMissingTOTP(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return true, false })
	req := httptest.NewRequest(http.MethodGet, "/api-keys", nil)
	rr := httptest.NewRecorder()
	data := templateFixtures(time.Now())["api_keys.html"]
	data["UserTOTPEnabled"] = false
	data["SensitiveTOTPFresh"] = false

	tmpl.Render(rr, req, "api_keys.html", data)

	body := rr.Body.String()
	if !strings.Contains(body, `activar 2FA en tu perfil`) {
		t.Fatalf("API key reveal does not explain how to enable TOTP:\n%s", body)
	}
	if !strings.Contains(body, `id="revealSubmit"`) || !strings.Contains(body, `disabled`) {
		t.Fatalf("API key reveal action remains enabled without TOTP:\n%s", body)
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
		SMTPHost:                   "smtp.example.test",
		SMTPPort:                   587,
		SMTPUser:                   "admin@example.test",
		Recipients:                 "ops@example.test",
		IsEnabled:                  true,
		SendTime:                   "09:00",
		RetentionMonths:            6,
		RetentionEnabled:           true,
		AlertDiskPct:               85,
		AlertWindowsDiskPct:        90,
		AlertBackupErr:             true,
		AlertPVEExpectedFinishTime: "08:55",
		AlertPBSStaleHours:         36,
		AlertPVEHeartbeatMinutes:   15,
	}
	telegramConfig := domain.TelegramConfig{
		BotUsername: "probakgo_test_bot",
		IsEnabled:   true,
	}
	telegramDestinations := []domain.TelegramDestination{
		{ID: 1, UserID: 1, Username: "admin", UserIsActive: true, ChatID: "123456789", ChatTitle: "Nestor", ChatType: "private"},
		{ID: 2, UserID: 2, Username: "editor", UserIsActive: false, ChatID: "987654321", ChatTitle: "Operaciones", ChatType: "private"},
	}
	productionChecklist := productionChecklistView{
		Items: []productionChecklistItem{
			{
				Title:    "HTTPS detectado",
				Detail:   "test",
				Status:   "OK",
				CSSClass: "ok",
				Icon:     "bi-shield-check",
			},
			{
				Title:       "SESSION_SECURE=false",
				Detail:      "test",
				Status:      "Accion",
				CSSClass:    "bad",
				Icon:        "bi-cookie",
				ActionPost:  "/settings/system/session-secure",
				ActionLabel: "Activar",
			},
		},
		OK:    1,
		Bad:   1,
		Ready: false,
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
			"AlertGroups": []alertGroup{{
				ServerName: "pve-1",
				ServerType: "pve",
				ServerID:   1,
				ServerURL:  "https://pve.example.test:8006",
				Warning:    1,
				Alerts: []domain.Alert{{
					ID:         "disk:pve:1:local",
					ServerName: "pve-1",
					ServerType: "pve",
					ServerID:   1,
					Severity:   domain.AlertSeverityWarning,
					Title:      "Disco casi lleno",
					Message:    "90% usado",
				}},
			}},
			"Suppressed": []struct {
				Alert domain.Alert
				Until time.Time
			}{},
			"SuppressedGroups":  []suppressedAlertGroup{},
			"ServerNames":       []string{"pve-1"},
			"FilterSeverity":    "",
			"FilterServer":      "",
			"AlertEvents":       []domain.AlertStateEvent{{EventType: "appeared", ServerName: "pve-1", ServerType: "pve", ServerID: 1, Title: "Sin reporte", Message: "test", CreatedAt: now}},
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
		"telegram_settings.html": base(map[string]any{
			"Config":                 telegramConfig,
			"Destinations":           telegramDestinations,
			"ActiveDestinationCount": 1,
			"Status":                 &domain.TelegramDeliveryStatus{LastSuccessAt: &now},
			"TokenPresent":           true,
		}),
		"api_key_created.html": base(map[string]any{
			"Name":        "cliente-pve",
			"Key":         "pbk-1234567890abcdef",
			"APIURL":      "http://probakgo.test:36748",
			"GitHubToken": "",
		}),
		"api_key_edit.html": base(map[string]any{
			"Key": domain.APIKey{ID: 1, Name: "cliente-pve", Key: "pbk-1234567890abcdef", ServerName: "pve-1", ServerURL: "https://10.0.0.1:8006"},
		}),
		"api_key_new.html": base(map[string]any{}),
		"api_keys.html": base(map[string]any{
			"Keys":               []map[string]any{},
			"UserTOTPEnabled":    true,
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
			"PVEOk":              1,
			"PVEBackupErrors":    0,
			"PVEStale":           0,
			"PBSOk":              1,
			"PBSStale":           0,
			"PBSMaintenance":     0,
			"WindowsOK":          1,
			"WindowsDiskAlerts":  0,
			"WindowsOffline":     0,
			"WindowsMaintenance": 0,
			"PVEMaintenance":     0,
			"MaintenanceTotal":   0,
			"PVERows":            []map[string]any{},
			"PBSRows":            []map[string]any{},
			"WindowsRows":        []map[string]any{},
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
			"User":                domain.User{ID: 1, Username: "admin", Role: "admin", IsActive: true, CreatedAt: now},
			"TelegramConfig":      telegramConfig,
			"TelegramDestination": (*domain.TelegramDestination)(nil),
			"TelegramPairingURL":  "https://t.me/probakgo_test_bot?start=testcode",
			"TelegramQRDataURI":   template.URL("data:image/png;base64,test"),
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
		"not_found.html": map[string]any{
			"Path":     "/ruta-inexistente",
			"LoggedIn": true,
		},
		"server_pve_detail.html": base(map[string]any{
			"Server":          pveServer,
			"ServerURL":       "https://pve.example.test:8006",
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
			"Config":                   emailConfig,
			"TelegramConfig":           telegramConfig,
			"TelegramDestinationCount": len(telegramDestinations),
			"TelegramStatus":           &domain.TelegramDeliveryStatus{LastSuccessAt: &now},
			"BanCount":                 0,
			"ProductionChecklist":      productionChecklist,
		}),
		"system_settings.html": base(map[string]any{
			"Config":              emailConfig,
			"ProductionChecklist": productionChecklist,
		}),
		"user_edit.html": base(map[string]any{
			"User":                &domain.User{ID: 1, Username: "editor", Role: "editor", IsActive: true, CreatedAt: now},
			"CurrentUsername":     "admin",
			"TelegramDestination": &telegramDestinations[1],
		}),
		"user_new.html": base(map[string]any{}),
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

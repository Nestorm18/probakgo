package webhandlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

func TestPVEServersRendersServerWithoutReport(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	st := openAlertsHandlerDB(t)
	if _, err := st.UpsertPVEServer(t.Context(), "pve-new", "10.0.0.10", "", "test", "mid-new"); err != nil {
		t.Fatalf("UpsertPVEServer: %v", err)
	}

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	h := New(st, tmpl, service.NewReport(st, time.UTC))
	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	loginRR := httptest.NewRecorder()
	if err := session.SetUser(loginRR, loginReq, 1, "admin", "admin"); err != nil {
		t.Fatalf("session.SetUser: %v", err)
	}
	for _, cookie := range loginRR.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	h.PVEServers(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body:\n%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if strings.Contains(body, "Error renderizando plantilla") {
		t.Fatalf("template error rendered:\n%s", body)
	}
	if !strings.Contains(body, "pve-new") {
		t.Fatalf("server missing from response:\n%s", body)
	}
}

func TestPVEServersShowsBackupsNotRequiredWhenNoVMsConfigured(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	st := openAlertsHandlerDB(t)
	serverID, err := st.UpsertPVEServer(t.Context(), "pve-empty", "10.0.0.10", "", "test", "mid-empty")
	if err != nil {
		t.Fatalf("UpsertPVEServer: %v", err)
	}
	if err := st.SetPVEBackupInventory(t.Context(), serverID, false); err != nil {
		t.Fatalf("confirm empty inventory: %v", err)
	}

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	h := New(st, tmpl, service.NewReport(st, time.UTC))
	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	loginRR := httptest.NewRecorder()
	if err := session.SetUser(loginRR, loginReq, 1, "admin", "admin"); err != nil {
		t.Fatalf("session.SetUser: %v", err)
	}
	for _, cookie := range loginRR.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h.PVEServers(rr, req)

	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "No requerido") {
		t.Fatalf("empty backup config should render as not required (status %d):\n%s", rr.Code, body)
	}
	if strings.Contains(body, `status-pill bad">Sin reporte`) {
		t.Fatalf("server without VMs still rendered as missing report:\n%s", body)
	}
}

func TestPVEServersShowsBackupsNotRequiredWhenAllVMsExcluded(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	st := openAlertsHandlerDB(t)
	serverID, err := st.UpsertPVEServer(t.Context(), "pve-no-backups", "10.0.0.10", "", "test", "mid-no-backups")
	if err != nil {
		t.Fatalf("UpsertPVEServer: %v", err)
	}
	if _, err := st.CreateVMBackupConfigForServer(t.Context(), "pve", serverID, "pve-no-backups", domain.CreateVMBackupConfigRequest{
		VMID: "100", VMName: "vm", Monday: true,
	}); err != nil {
		t.Fatalf("create backup config: %v", err)
	}
	if err := st.ToggleVMExcludeForServer(t.Context(), "pve", serverID, "100"); err != nil {
		t.Fatalf("exclude VM: %v", err)
	}
	status := &domain.BackupStatus{Status: json.RawMessage(`"ERROR"`)}
	if _, err := st.InsertPVEReport(t.Context(), serverID, status); err != nil {
		t.Fatalf("InsertPVEReport: %v", err)
	}

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	h := New(st, tmpl, service.NewReport(st, time.UTC))
	req := httptest.NewRequest(http.MethodGet, "/servers/pve", nil)
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	loginRR := httptest.NewRecorder()
	if err := session.SetUser(loginRR, loginReq, 1, "admin", "admin"); err != nil {
		t.Fatalf("session.SetUser: %v", err)
	}
	for _, cookie := range loginRR.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h.PVEServers(rr, req)

	body := rr.Body.String()
	if rr.Code != http.StatusOK || !strings.Contains(body, "No requerido") {
		t.Fatalf("backups-not-required state missing (status %d):\n%s", rr.Code, body)
	}
	if strings.Contains(body, `status-pill bad">ERROR`) {
		t.Fatalf("excluded backups still render as an error:\n%s", body)
	}
}

func TestPVEServerDetailLinksConfiguredProxmoxURL(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	st := openAlertsHandlerDB(t)
	key, err := st.CreateAPIKey(t.Context(), "pve-new", "pve-new", "https://pve.example.test:8006")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	serverID, err := st.UpsertPVEServerForAPIKey(t.Context(), key.ID, "pve-new", "10.0.0.10", "", "test", "mid-new")
	if err != nil {
		t.Fatalf("UpsertPVEServerForAPIKey: %v", err)
	}

	tmpl := NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	h := New(st, tmpl, service.NewReport(st, time.UTC))
	req := httptest.NewRequest(http.MethodGet, "/servers/pve/"+strconv.FormatInt(serverID, 10), nil)
	routeCtx := chi.NewRouteContext()
	routeCtx.URLParams.Add("id", strconv.FormatInt(serverID, 10))
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, routeCtx))
	loginReq := httptest.NewRequest(http.MethodGet, "/", nil)
	loginRR := httptest.NewRecorder()
	if err := session.SetUser(loginRR, loginReq, 1, "admin", "admin"); err != nil {
		t.Fatalf("session.SetUser: %v", err)
	}
	for _, cookie := range loginRR.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()

	h.PVEServerDetail(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status: got %d, body:\n%s", rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	if !strings.Contains(body, `href="https://pve.example.test:8006"`) || !strings.Contains(body, "Abrir Proxmox") {
		t.Fatalf("configured Proxmox link missing from response:\n%s", body)
	}
}

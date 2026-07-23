package webhandlers

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

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
	if err := session.SetUser(loginRR, loginReq, "admin", "admin"); err != nil {
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

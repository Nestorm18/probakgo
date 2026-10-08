package webhandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

func TestNASSettingsDoNotReuseSavedPasswordForAnotherHost(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	st, err := store.NewEncrypted(database, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}
	ctx := context.Background()
	saved := domain.NASBackupConfig{Enabled: true, Host: "nas.local", Port: 22, Username: "backup", Password: "saved-secret", Directory: "/backups", SendTime: "02:00"}
	if err := st.SaveNASBackupConfig(ctx, saved); err != nil {
		t.Fatalf("save NAS config: %v", err)
	}
	madrid := time.FixedZone("Europe/Madrid", 2*60*60)
	h := New(st, NewTemplates(os.DirFS("../../.."), "test", madrid, false, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false }), nil)

	post := func(host string) *httptest.ResponseRecorder {
		form := url.Values{
			"action": {"save"}, "nas_enabled": {"on"}, "nas_host": {host}, "nas_port": {"22"},
			"nas_username": {"backup"}, "nas_directory": {"/backups"}, "nas_time": {"02:00"},
		}
		req := httptest.NewRequest(http.MethodPost, "/settings/maintenance/nas", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		sessionRR := httptest.NewRecorder()
		if err := session.SetUser(sessionRR, req, 1, "admin", "admin"); err != nil {
			t.Fatalf("set admin session: %v", err)
		}
		for _, cookie := range sessionRR.Result().Cookies() {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		h.NASBackupSettingsPost(rr, req)
		return rr
	}

	rr := post("attacker.example")
	if body := rr.Body.String(); !strings.Contains(body, "Introduce la contraseña SFTP") || !strings.Contains(body, "Europe/Madrid") {
		t.Fatalf("host change without password was not rejected:\n%s", body)
	}
	got, err := st.GetNASBackupConfig(ctx)
	if err != nil || got.Host != "nas.local" || got.Password != "saved-secret" {
		t.Fatalf("saved NAS config changed: %+v %v", got, err)
	}

	if rr := post("nas.local"); rr.Code != http.StatusSeeOther {
		t.Fatalf("same destination without password should keep the saved one: %d %s", rr.Code, rr.Body.String())
	}
	if got, err := st.GetNASBackupConfig(ctx); err != nil || got.Password != "saved-secret" {
		t.Fatalf("saved password lost: %+v %v", got, err)
	}
}

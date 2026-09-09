package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

func TestOldCookieCannotAuthenticateRecreatedUsername(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	uid, err := st.CreateUser(ctx, "reused-name", "old-hash", "reader")
	if err != nil {
		t.Fatal(err)
	}
	session.Init("audit-session-key-with-at-least-32-bytes", false)
	saved := httptest.NewRecorder()
	if err := session.SetUserWithVersion(saved, httptest.NewRequest("GET", "/", nil), uid, "reused-name", "reader", 1); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteUser(ctx, uid); err != nil {
		t.Fatal(err)
	}
	newID, err := st.CreateUser(ctx, "reused-name", "different-password", "admin")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "/users", nil)
	for _, cookie := range saved.Result().Cookies() {
		req.AddCookie(cookie)
	}
	accepted := false
	handler := RequireLogin(st)(RequireAdmin(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted = true })))
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)
	t.Logf("old reader user=%d new admin user=%d old cookie accepted on admin route=%v HTTP=%d", uid, newID, accepted, rr.Code)
	if accepted {
		t.Fatal("unauthorized operation was accepted")
	}
}

func TestSensitiveTOTPLimitsAttemptsAcrossRoutesAndIPs(t *testing.T) {
	dbHandle, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer dbHandle.Close()
	st := store.New(dbHandle)
	ctx := context.Background()
	id, err := st.CreateUser(ctx, "admin", "hash", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnableUserTOTP(ctx, id, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SensitiveActionsRequireTOTP: true}); err != nil {
		t.Fatal(err)
	}
	session.Init("test-key-with-at-least-thirty-two-bytes", false)
	w := httptest.NewRecorder()
	if err := session.SetUserWithVersion(w, httptest.NewRequest("GET", "/", nil), id, "admin", "admin", 2); err != nil {
		t.Fatal(err)
	}
	protected := RequireTOTPForSensitiveAction(st)
	for attempt := 0; attempt < 6; attempt++ {
		r := httptest.NewRequest("POST", "/keys/reveal", strings.NewReader("totp_code=invalid"))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.RemoteAddr = []string{"192.0.2.1:1234", "192.0.2.2:1234"}[attempt%2]
		for _, c := range w.Result().Cookies() {
			r.AddCookie(c)
		}
		rr := httptest.NewRecorder()
		protected(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Error("invalid code accepted") })).ServeHTTP(rr, r)
		want := http.StatusForbidden
		if attempt == 5 {
			want = http.StatusTooManyRequests
		}
		if rr.Code != want {
			t.Fatalf("attempt %d: HTTP %d, want %d", attempt, rr.Code, want)
		}
	}
}

func TestSensitiveTOTPDeniesOnConfigurationReadError(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "audit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	if _, err := st.CreateUser(ctx, "audit-admin", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SensitiveActionsRequireTOTP: true}); err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec("DROP TABLE email_config"); err != nil {
		t.Fatal(err)
	}
	session.Init("audit-session-key-with-at-least-32-bytes", false)
	saved := httptest.NewRecorder()
	if err := session.SetUser(saved, httptest.NewRequest("GET", "/", nil), 1, "audit-admin", "admin"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "/settings/system", nil)
	for _, cookie := range saved.Result().Cookies() {
		req.AddCookie(cookie)
	}
	accepted := false
	handler := RequireTOTPForSensitiveAction(st)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted = true }))
	handler.ServeHTTP(httptest.NewRecorder(), req)
	t.Logf("sensitive operation without TOTP allowed after configuration read failure=%v", accepted)
	if accepted {
		t.Fatal("unauthorized operation was accepted")
	}
}

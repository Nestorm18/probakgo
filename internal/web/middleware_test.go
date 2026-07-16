package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

func TestSensitiveTOTPRecentSessionNeverAcceptsExplicitWrongCode(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "probakgo-test.db"))
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	st := store.New(sqlDB)

	ctx := context.Background()
	userID, err := st.CreateUser(ctx, "admin", "hash", "admin")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if err := st.EnableUserTOTP(ctx, userID, "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"); err != nil {
		t.Fatalf("enable TOTP: %v", err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SensitiveActionsRequireTOTP: true}); err != nil {
		t.Fatalf("enable sensitive-action TOTP: %v", err)
	}
	user, err := st.GetUser(ctx, userID)
	if err != nil {
		t.Fatalf("get user: %v", err)
	}

	session.Init("test-session-key-32-bytes-long!!", false)
	authReq := httptest.NewRequest(http.MethodGet, "/", nil)
	authRR := httptest.NewRecorder()
	if err := session.SetUserWithVersion(authRR, authReq, user.Username, user.Role, user.SessionVersion); err != nil {
		t.Fatalf("set user session: %v", err)
	}

	freshReq := httptest.NewRequest(http.MethodGet, "/", nil)
	for _, cookie := range authRR.Result().Cookies() {
		freshReq.AddCookie(cookie)
	}
	freshRR := httptest.NewRecorder()
	if err := session.SetSensitiveTOTPFresh(freshRR, freshReq, time.Now().Add(5*time.Minute)); err != nil {
		t.Fatalf("set recent TOTP: %v", err)
	}
	freshCookies := freshRR.Result().Cookies()

	run := func(code string) (called bool, status int) {
		form := url.Values{}
		if code != "" {
			form.Set("totp_code", code)
		}
		req := httptest.NewRequest(http.MethodPost, "/settings/system", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range freshCookies {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		handler := RequireTOTPForSensitiveAction(st)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			called = true
			w.WriteHeader(http.StatusNoContent)
		}))
		handler.ServeHTTP(rr, req)
		return called, rr.Code
	}

	if called, status := run(""); !called || status != http.StatusNoContent {
		t.Fatalf("recent session without code: called=%t status=%d", called, status)
	}
	if called, status := run("000000"); called || status != http.StatusSeeOther {
		t.Fatalf("recent session with wrong code: called=%t status=%d", called, status)
	}
}

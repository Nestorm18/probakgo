package web

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/session"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
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

func TestTOTPEnforcementNeverDisablesLastActiveAdmin(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "lockout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SendTime: "08:00", EnforceTOTPNonReaders: true}); err != nil {
		t.Fatal(err)
	}
	adminID, err := st.CreateUser(ctx, "root-admin", "hash", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sqlDB.Exec(`UPDATE users SET totp_grace_started_at=? WHERE id=?`, time.Now().Add(-96*time.Hour), adminID); err != nil {
		t.Fatal(err)
	}
	session.Init("lockout-session-key-with-at-least-32-bytes", false)

	request := func() (bool, int) {
		user, err := st.GetUser(ctx, adminID)
		if err != nil {
			t.Fatal(err)
		}
		saved := httptest.NewRecorder()
		if err := session.SetUserWithVersion(saved, httptest.NewRequest("GET", "/", nil), adminID, user.Username, user.Role, user.SessionVersion); err != nil {
			t.Fatal(err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range saved.Result().Cookies() {
			req.AddCookie(cookie)
		}
		accepted := false
		rr := httptest.NewRecorder()
		RequireLogin(st)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted = true })).ServeHTTP(rr, req)
		return accepted, rr.Code
	}

	if accepted, code := request(); !accepted {
		t.Fatalf("last active admin was rejected after the 2FA grace period: HTTP %d", code)
	}
	if user, err := st.GetUser(ctx, adminID); err != nil || !user.IsActive {
		t.Fatalf("last active admin was disabled: %+v %v", user, err)
	}

	if _, err := st.CreateUser(ctx, "second-admin", "hash", "admin"); err != nil {
		t.Fatal(err)
	}
	if accepted, _ := request(); accepted {
		t.Fatal("admin without 2FA was accepted although another admin is active")
	}
	if user, err := st.GetUser(ctx, adminID); err != nil || user.IsActive {
		t.Fatalf("admin without 2FA should be disabled when another admin exists: %+v %v", user, err)
	}
}

func TestOwnPasswordChangeKeepsCurrentSessionAndRevokesOthers(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "profile.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("old-password-123"), bcrypt.MinCost)
	uid, err := st.CreateUser(ctx, "alice", string(hash), "editor")
	if err != nil {
		t.Fatal(err)
	}
	session.Init("profile-session-key-with-at-least-32-bytes", false)
	user, _ := st.GetUser(ctx, uid)
	saved := httptest.NewRecorder()
	if err := session.SetUserWithVersion(saved, httptest.NewRequest("GET", "/", nil), uid, user.Username, user.Role, user.SessionVersion); err != nil {
		t.Fatal(err)
	}
	oldCookies := saved.Result().Cookies()
	h := webhandlers.New(st, nil, nil)

	post := func(password string) *httptest.ResponseRecorder {
		form := url.Values{"current_password": {password}, "new_password": {"new-password-456"}, "password_confirm": {"new-password-456"}}
		req := httptest.NewRequest(http.MethodPost, "/profile", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range oldCookies {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		h.ProfilePost(rr, req)
		return rr
	}
	authorized := func(cookies []*http.Cookie) bool {
		req := httptest.NewRequest("GET", "/profile", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		accepted := false
		RequireLogin(st)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted = true })).ServeHTTP(httptest.NewRecorder(), req)
		return accepted
	}

	rr := post("old-password-123")
	if location := rr.Header().Get("Location"); rr.Code != http.StatusSeeOther || !strings.HasPrefix(location, "/profile?") || !strings.Contains(location, "ok=1") {
		t.Fatalf("password change redirect: %d %q", rr.Code, location)
	}
	if !authorized(rr.Result().Cookies()) {
		t.Fatal("the session that changed the password was signed out")
	}
	if authorized(oldCookies) {
		t.Fatal("a session issued before the password change is still valid")
	}
}

func TestCurrentPasswordChecksAreLimitedPerUser(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "limit.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	hash, _ := bcrypt.GenerateFromPassword([]byte("right-password-1"), bcrypt.MinCost)
	uid, err := st.CreateUser(ctx, "bob", string(hash), "editor")
	if err != nil {
		t.Fatal(err)
	}
	session.Init("limit-session-key-with-at-least-32-bytes", false)
	saved := httptest.NewRecorder()
	if err := session.SetUserWithVersion(saved, httptest.NewRequest("GET", "/", nil), uid, "bob", "editor", 1); err != nil {
		t.Fatal(err)
	}
	h := webhandlers.New(st, nil, nil)
	disable2FA := func(password string) string {
		form := url.Values{"current_password": {password}}
		req := httptest.NewRequest(http.MethodPost, "/profile/2fa/disable", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range saved.Result().Cookies() {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		h.Profile2FADisable(rr, req)
		location, _ := url.QueryUnescape(rr.Header().Get("Location"))
		return location
	}

	for range 5 {
		if location := disable2FA("wrong-password"); !strings.Contains(location, "Contraseña actual incorrecta") {
			t.Fatalf("wrong password response: %q", location)
		}
	}
	if location := disable2FA("right-password-1"); !strings.Contains(location, "Demasiados intentos") {
		t.Fatalf("correct password accepted after repeated failures: %q", location)
	}
}

func TestSensitiveTOTPCodeCannotBeReplayed(t *testing.T) {
	dbHandle, err := db.Open(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer dbHandle.Close()
	st := store.New(dbHandle)
	ctx := context.Background()
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	id, err := st.CreateUser(ctx, "admin", "hash", "admin")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.EnableUserTOTP(ctx, id, secret); err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertEmailConfig(ctx, domain.EmailConfig{SensitiveActionsRequireTOTP: true}); err != nil {
		t.Fatal(err)
	}
	user, _ := st.GetUser(ctx, id)
	session.Init("replay-session-key-with-at-least-32-bytes", false)
	saved := httptest.NewRecorder()
	if err := session.SetUserWithVersion(saved, httptest.NewRequest("GET", "/", nil), id, "admin", "admin", user.SessionVersion); err != nil {
		t.Fatal(err)
	}

	var counter [8]byte
	binary.BigEndian.PutUint64(counter[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, []byte("12345678901234567890"))
	mac.Write(counter[:])
	sum := mac.Sum(nil)
	code := fmt.Sprintf("%06d", (binary.BigEndian.Uint32(sum[sum[len(sum)-1]&15:])&0x7fffffff)%1000000)

	accepted := 0
	protected := RequireTOTPForSensitiveAction(st)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { accepted++ }))
	for range 2 {
		// Each request starts from the original cookie, as a captured request would.
		r := httptest.NewRequest("POST", "/api-keys/1/reveal", strings.NewReader("totp_code="+code))
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, c := range saved.Result().Cookies() {
			r.AddCookie(c)
		}
		protected.ServeHTTP(httptest.NewRecorder(), r)
	}
	if accepted != 1 {
		t.Fatalf("the same TOTP code was accepted %d times, want once", accepted)
	}
}

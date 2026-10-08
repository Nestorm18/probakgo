package webhandlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/bcrypt"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/ratelimit"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

type capturedSecurityNotification struct {
	text string
	link string
}

type captureAdminSecurityNotifier struct {
	notifications chan capturedSecurityNotification
}

func (n *captureAdminSecurityNotifier) SendAdminSecurityNotification(_ context.Context, text, linkURL string) error {
	n.notifications <- capturedSecurityNotification{text: text, link: linkURL}
	return nil
}

func newSecurityNotificationHandler(t *testing.T, withTemplates bool) (*WebH, *store.Store, *captureAdminSecurityNotifier) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { database.Close() })
	st := store.New(database)
	var tmpl *Templates
	if withTemplates {
		tmpl = NewTemplates(os.DirFS("../../.."), "test", time.UTC, true, func() (int, int) { return 0, 0 }, func() (bool, bool) { return false, false })
	}
	h := New(st, tmpl, nil)
	notifier := &captureAdminSecurityNotifier{notifications: make(chan capturedSecurityNotification, 1)}
	h.telegram = notifier
	return h, st, notifier
}

func waitSecurityNotification(t *testing.T, notifier *captureAdminSecurityNotifier) capturedSecurityNotification {
	t.Helper()
	select {
	case notification := <-notifier.notifications:
		return notification
	case <-time.After(2 * time.Second):
		t.Fatal("Telegram security notification was not sent")
		return capturedSecurityNotification{}
	}
}

func TestLoginPostNotifiesAdminsOnSuccess(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	h, st, notifier := newSecurityNotificationHandler(t, false)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if _, err := st.CreateUser(context.Background(), "alice", string(hash), "reader"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	form := url.Values{"username": {"alice"}, "password": {"correct-password"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.LoginPost(rr, req)

	notification := waitSecurityNotification(t, notifier)
	if !strings.Contains(notification.text, "inicio de sesión") || !strings.Contains(notification.text, "Usuario: alice") || notification.link != "/settings/ip-bans" {
		t.Fatalf("unexpected login notification: %+v", notification)
	}
}

func TestLoginPostNotifiesAdminsOnInvalidPassword(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	h, st, notifier := newSecurityNotificationHandler(t, true)
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if _, err := st.CreateUser(context.Background(), "alice", string(hash), "reader"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	form := url.Values{"username": {"alice"}, "password": {"wrong-password"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.LoginPost(rr, req)

	notification := waitSecurityNotification(t, notifier)
	if !strings.Contains(notification.text, "intento de acceso fallido") || !strings.Contains(notification.text, "Credenciales no válidas") {
		t.Fatalf("unexpected failed-login notification: %+v", notification)
	}
}

func TestCreateUserPostNotifiesAdmins(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	h, _, notifier := newSecurityNotificationHandler(t, false)
	form := url.Values{"username": {"bob"}, "password": {"secret123456"}, "role": {"editor"}}
	req := httptest.NewRequest(http.MethodPost, "/users", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	sessionRR := httptest.NewRecorder()
	if err := session.SetUser(sessionRR, req, 1, "admin", "admin"); err != nil {
		t.Fatalf("set admin session: %v", err)
	}
	for _, cookie := range sessionRR.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	h.CreateUserPost(rr, req)

	notification := waitSecurityNotification(t, notifier)
	if !strings.Contains(notification.text, "usuario creado") || !strings.Contains(notification.text, "Usuario: bob") || !strings.Contains(notification.text, "Creado por: admin") || notification.link == "" {
		t.Fatalf("unexpected user-created notification: %+v", notification)
	}
}

func TestLoginNoticeThrottleLimitsPerIPAndGlobally(t *testing.T) {
	var throttle loginNoticeThrottle
	now := time.Now()

	if ok, skipped := throttle.allow("10.0.0.1", now); !ok || skipped != 0 {
		t.Fatalf("first notice: ok=%v skipped=%d", ok, skipped)
	}
	if ok, _ := throttle.allow("10.0.0.1", now.Add(time.Minute)); ok {
		t.Fatal("second notice for the same IP inside the interval was allowed")
	}
	if ok, skipped := throttle.allow("10.0.0.2", now.Add(2*time.Minute)); !ok || skipped != 1 {
		t.Fatalf("notice for another IP: ok=%v skipped=%d, want skipped=1", ok, skipped)
	}
	if ok, _ := throttle.allow("10.0.0.1", now.Add(failedLoginNoticeInterval+time.Second)); !ok {
		t.Fatal("notice after the per-IP interval was not allowed")
	}

	var global loginNoticeThrottle
	for i := range failedLoginNoticeLimit {
		if ok, _ := global.allow(fmt.Sprintf("10.1.0.%d", i), now); !ok {
			t.Fatalf("notice %d below the global limit was rejected", i)
		}
	}
	if ok, _ := global.allow("10.2.0.1", now); ok {
		t.Fatal("notice above the global limit was allowed")
	}
	if ok, skipped := global.allow("10.2.0.1", now.Add(failedLoginNoticeWindow+time.Second)); !ok || skipped != 1 {
		t.Fatalf("notice after the global window: ok=%v skipped=%d", ok, skipped)
	}
}

func TestLoginPostNotifiesBanOnceAndIgnoresBlockedAttempts(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	h, st, notifier := newSecurityNotificationHandler(t, true)
	h.SetBanhammer(ratelimit.NewBanhammer(3, time.Hour, nil, time.Hour))
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if _, err := st.CreateUser(context.Background(), "alice", string(hash), "reader"); err != nil {
		t.Fatalf("create user: %v", err)
	}
	login := func() {
		form := url.Values{"username": {"alice"}, "password": {"wrong-password"}}
		req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		h.LoginPost(httptest.NewRecorder(), req)
	}

	login()
	if first := waitSecurityNotification(t, notifier); strings.Contains(first.text, "IP bloqueada") {
		t.Fatalf("first failure reported a ban: %+v", first)
	}
	login()
	login()
	if ban := waitSecurityNotification(t, notifier); !strings.Contains(ban.text, "IP bloqueada") || !strings.Contains(ban.text, "Avisos omitidos desde el anterior: 1") {
		t.Fatalf("ban notification: %+v", ban)
	}
	login()
	select {
	case extra := <-notifier.notifications:
		t.Fatalf("blocked attempt sent a notification: %+v", extra)
	case <-time.After(200 * time.Millisecond):
	}
}

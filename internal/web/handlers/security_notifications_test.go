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

	"golang.org/x/crypto/bcrypt"

	dbpkg "probakgo/internal/db"
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
	if err := session.SetUser(sessionRR, req, "admin", "admin"); err != nil {
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

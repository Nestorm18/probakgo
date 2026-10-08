package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"probakgo/internal/db"
	"probakgo/internal/session"
	"probakgo/internal/store"
	webhandlers "probakgo/internal/web/handlers"
)

func TestLogoutRevokesCopiedCookieButNotOtherSessions(t *testing.T) {
	sqlDB, err := db.Open(filepath.Join(t.TempDir(), "logout.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer sqlDB.Close()
	st := store.New(sqlDB)
	ctx := context.Background()
	uid, err := st.CreateUser(ctx, "alice", "hash", "editor")
	if err != nil {
		t.Fatal(err)
	}
	session.Init("logout-session-key-with-at-least-32-bytes", false)
	user, _ := st.GetUser(ctx, uid)
	login := func() []*http.Cookie {
		rr := httptest.NewRecorder()
		if err := session.SetUserWithVersion(rr, httptest.NewRequest("GET", "/", nil), uid, user.Username, user.Role, user.SessionVersion); err != nil {
			t.Fatal(err)
		}
		return rr.Result().Cookies()
	}
	authorized := func(cookies []*http.Cookie) bool {
		req := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range cookies {
			req.AddCookie(cookie)
		}
		accepted := false
		RequireLogin(st)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { accepted = true })).ServeHTTP(httptest.NewRecorder(), req)
		return accepted
	}

	laptop, phone := login(), login()
	req := httptest.NewRequest(http.MethodPost, "/logout", nil)
	for _, cookie := range laptop {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	webhandlers.New(st, nil, nil).Logout(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/login" {
		t.Fatalf("logout response: %d %q", rr.Code, rr.Header().Get("Location"))
	}

	if authorized(laptop) {
		t.Fatal("a copy of the logged-out cookie is still accepted")
	}
	if !authorized(phone) {
		t.Fatal("logout also closed another session of the same user")
	}
}

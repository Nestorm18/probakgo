package session

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSessionIDIsNewPerLoginAndKeptOnRoleRefresh(t *testing.T) {
	Init("sid-session-key-32-bytes-long!!!", false)
	login := func() *http.Request {
		rr := httptest.NewRecorder()
		if err := SetUserWithVersion(rr, httptest.NewRequest("GET", "/", nil), 1, "admin", "admin", 1); err != nil {
			t.Fatalf("SetUserWithVersion: %v", err)
		}
		req := httptest.NewRequest("GET", "/", nil)
		for _, c := range rr.Result().Cookies() {
			req.AddCookie(c)
		}
		return req
	}

	first, second := login(), login()
	firstID, ok := ID(first)
	if !ok {
		t.Fatal("login did not assign a session ID")
	}
	if secondID, _ := ID(second); secondID == firstID {
		t.Fatal("two logins share a session ID")
	}

	rr := httptest.NewRecorder()
	if err := RefreshRole(rr, first, "reader", 2); err != nil {
		t.Fatalf("RefreshRole: %v", err)
	}
	refreshed := httptest.NewRequest("GET", "/", nil)
	for _, c := range rr.Result().Cookies() {
		refreshed.AddCookie(c)
	}
	if id, _ := ID(refreshed); id != firstID {
		t.Fatalf("RefreshRole changed the session ID: got %q, want %q", id, firstID)
	}
	if _, role, _ := GetUser(refreshed); role != "reader" {
		t.Fatalf("role after refresh: got %q, want reader", role)
	}
}

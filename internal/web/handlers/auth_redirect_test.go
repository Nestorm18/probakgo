package webhandlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/session"
)

func TestSafeLocalPathRejectsBrowserNormalisedHosts(t *testing.T) {
	for _, raw := range []string{
		"", "servers", "//evil.example", "/\\evil.example", "/\t/evil.example", "/\n/evil.example",
		"https://evil.example/", "/a\r\nLocation: //evil.example",
	} {
		if got := SafeLocalPath(raw); got != "" {
			t.Errorf("SafeLocalPath(%q) = %q, want rejection", raw, got)
		}
	}
	for _, raw := range []string{"/", "/servers/pve/3", "/alerts?filter=critical&page=2"} {
		if got := SafeLocalPath(raw); got != raw {
			t.Errorf("SafeLocalPath(%q) = %q, want unchanged", raw, got)
		}
	}
	for _, raw := range []string{"/login", "/login/2fa?x=1", "/logout", "/\\evil.example"} {
		if got := safeNext(raw); got != "/" {
			t.Errorf("safeNext(%q) = %q, want /", raw, got)
		}
	}
}

func TestLoginKeepsRequestedPage(t *testing.T) {
	session.Init("test-session-key-32-bytes-long!!", false)
	h, st, _ := newSecurityNotificationHandler(t, true)
	h.telegram = nil
	hash, _ := bcrypt.GenerateFromPassword([]byte("correct-password"), bcrypt.MinCost)
	if _, err := st.CreateUser(context.Background(), "alice", string(hash), "reader"); err != nil {
		t.Fatalf("create user: %v", err)
	}

	page := httptest.NewRecorder()
	h.LoginPage(page, httptest.NewRequest(http.MethodGet, "/login?next="+url.QueryEscape("/servers/pve/3?days=7"), nil))
	if !strings.Contains(page.Body.String(), `name="next" value="/servers/pve/3?days=7"`) {
		t.Fatalf("login page does not keep the requested page:\n%s", page.Body.String())
	}

	form := url.Values{"username": {"alice"}, "password": {"correct-password"}, "next": {"/servers/pve/3?days=7"}}
	req := httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr := httptest.NewRecorder()
	h.LoginPost(rr, req)
	if rr.Code != http.StatusSeeOther || rr.Header().Get("Location") != "/servers/pve/3?days=7" {
		t.Fatalf("login redirect: %d %q", rr.Code, rr.Header().Get("Location"))
	}

	form.Set("next", "/\\evil.example")
	req = httptest.NewRequest(http.MethodPost, "/login", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	h.LoginPost(rr, req)
	if rr.Header().Get("Location") != "/" {
		t.Fatalf("unsafe next was followed: %q", rr.Header().Get("Location"))
	}
}

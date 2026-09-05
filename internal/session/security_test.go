package session

import (
	"bytes"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestTOTPSeedEncryptedInCookie(t *testing.T) {
	Init("audit-session-key-with-at-least-32-bytes", false)
	const seed = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	rr := httptest.NewRecorder()
	if err := SetPendingTOTPSetup(rr, httptest.NewRequest("GET", "/", nil), seed); err != nil {
		t.Fatal(err)
	}
	outer, err := base64.URLEncoding.DecodeString(rr.Result().Cookies()[0].Value)
	if err != nil {
		t.Fatal(err)
	}
	parts := bytes.SplitN(outer, []byte("|"), 3)
	if len(parts) != 3 {
		t.Fatal("unexpected encoding")
	}
	payload, err := base64.URLEncoding.DecodeString(string(parts[1]))
	if err != nil {
		t.Fatal(err)
	}
	readable := bytes.Contains(payload, []byte(seed))
	t.Logf("TOTP seed recovered from cookie using only base64: %v", readable)
	if readable {
		t.Fatal("TOTP secret is readable without the session key")
	}
}

func pendingRequest(t *testing.T) *http.Request {
	t.Helper()
	w := httptest.NewRecorder()
	if err := SetPending2FA(w, httptest.NewRequest("GET", "/", nil), 42, "/profile", 3); err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("POST", "/login/2fa", nil)
	for _, c := range w.Result().Cookies() {
		r.AddCookie(c)
	}
	return r
}

func TestPending2FAExpiresAndBindsVersion(t *testing.T) {
	Init("test-key-with-at-least-thirty-two-bytes", false)
	r := pendingRequest(t)
	if id, next, version, ok := GetPending2FA(r); !ok || id != 42 || next != "/profile" || version != 3 {
		t.Fatal("pending login not available")
	}
	if ConsumePending2FA(r, 42, 4) {
		t.Fatal("accepted different credential version")
	}
	r = pendingRequest(t)
	token := challengeToken(r)
	challenges.Lock()
	c := challenges.items[token]
	c.expires = time.Now().Add(-time.Second)
	challenges.items[token] = c
	challenges.Unlock()
	if _, _, _, ok := GetPending2FA(r); ok {
		t.Fatal("expired challenge accepted")
	}
	if ConsumePending2FA(r, 42, 3) {
		t.Fatal("expired challenge consumed")
	}
	r = pendingRequest(t)
	resetChallenges()
	if _, _, _, ok := GetPending2FA(r); ok {
		t.Fatal("restart retained challenge")
	}
}

func TestPending2FACanBeConsumedOnlyOnce(t *testing.T) {
	Init("test-key-with-at-least-thirty-two-bytes", false)
	r := pendingRequest(t)
	cookies := r.Cookies()
	var accepted atomic.Int32
	var wg sync.WaitGroup
	for range 16 {
		wg.Go(func() {
			req := httptest.NewRequest("POST", "/login/2fa", nil)
			for _, cookie := range cookies {
				req.AddCookie(cookie)
			}
			if ConsumePending2FA(req, 42, 3) {
				accepted.Add(1)
			}
		})
	}
	wg.Wait()
	if accepted.Load() != 1 {
		t.Fatalf("challenge accepted %d times", accepted.Load())
	}
}

func TestPending2FAAttemptLimitIsIndependentOfIP(t *testing.T) {
	Init("test-key-with-at-least-thirty-two-bytes", false)
	r := pendingRequest(t)
	for range 5 {
		if !AllowPending2FAAttempt(r) {
			t.Fatal("challenge exhausted too early")
		}
	}
	if AllowPending2FAAttempt(r) {
		t.Fatal("allowed more than five attempts")
	}
	if ConsumePending2FA(r, 42, 3) {
		t.Fatal("exhausted challenge still authenticates")
	}
}

func TestTOTPSetupExpiresAndNewLoginClearsIt(t *testing.T) {
	Init("test-key-with-at-least-thirty-two-bytes", false)
	r := httptest.NewRequest("GET", "/profile", nil)
	w := httptest.NewRecorder()
	if err := SetPendingTOTPSetup(w, r, "secret"); err != nil {
		t.Fatal(err)
	}
	if _, ok := GetPendingTOTPSetup(r); !ok {
		t.Fatal("new setup missing")
	}
	sess, _ := getSession(r)
	sess.Values["pending_totp_expires"] = time.Now().Add(-time.Second).Unix()
	if _, ok := GetPendingTOTPSetup(r); ok {
		t.Fatal("expired setup accepted")
	}
	if err := SetUserWithVersion(w, r, 9, "another-user", "reader", 1); err != nil {
		t.Fatal(err)
	}
	if _, exists := sess.Values["pending_totp_secret"]; exists {
		t.Fatal("login retained prior user's setup seed")
	}
}

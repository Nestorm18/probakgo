package webhandlers_test

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"probakgo/internal/session"
	webhandlers "probakgo/internal/web/handlers"
)

func TestPending2FAInvalidatedByPasswordReset(t *testing.T) {
	st := openHandlerDB(t)
	ctx := context.Background()
	uid, err := st.CreateUser(ctx, "audit-login", "old-hash", "reader")
	if err != nil {
		t.Fatal(err)
	}
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if err := st.EnableUserTOTP(ctx, uid, secret); err != nil {
		t.Fatal(err)
	}
	pending := httptest.NewRecorder()
	if err := session.SetPending2FA(pending, httptest.NewRequest("GET", "/", nil), uid, "/", 2); err != nil {
		t.Fatal(err)
	}
	if err := st.UpdateUserPassword(ctx, uid, "new-password-hash"); err != nil {
		t.Fatal(err)
	}
	form := url.Values{"code": {testCurrentTOTP(secret)}}
	req := httptest.NewRequest("POST", "/login/2fa", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	for _, cookie := range pending.Result().Cookies() {
		req.AddCookie(cookie)
	}
	rr := httptest.NewRecorder()
	webhandlers.New(st, nil, nil).Login2FAPost(rr, req)
	follow := httptest.NewRequest("GET", "/", nil)
	for _, cookie := range rr.Result().Cookies() {
		follow.AddCookie(cookie)
	}
	name, _, ok := session.GetUser(follow)
	t.Logf("password reset followed by old pending cookie: status=%d authenticated=%v user=%s", rr.Code, ok, name)
	if rr.Code != http.StatusSeeOther || ok {
		t.Fatal("stale pending challenge authenticated after password reset")
	}
}

func testCurrentTOTP(secret string) string {
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(time.Now().Unix()/30))
	mac := hmac.New(sha1.New, key)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	offset := sum[len(sum)-1] & 15
	n := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", n%1000000)
}

func TestSuccessful2FAChallengeCannotBeReplayed(t *testing.T) {
	st := openHandlerDB(t)
	ctx := context.Background()
	uid, err := st.CreateUser(ctx, "once", "hash", "reader")
	if err != nil {
		t.Fatal(err)
	}
	const secret = "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	if err := st.EnableUserTOTP(ctx, uid, secret); err != nil {
		t.Fatal(err)
	}
	pending := httptest.NewRecorder()
	if err := session.SetPending2FA(pending, httptest.NewRequest("GET", "/", nil), uid, "/profile", 2); err != nil {
		t.Fatal(err)
	}
	h := webhandlers.New(st, nil, nil)
	for attempt := 0; attempt < 2; attempt++ {
		form := url.Values{"code": {testCurrentTOTP(secret)}}
		req := httptest.NewRequest("POST", "/login/2fa", strings.NewReader(form.Encode()))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		for _, cookie := range pending.Result().Cookies() {
			req.AddCookie(cookie)
		}
		rr := httptest.NewRecorder()
		h.Login2FAPost(rr, req)
		follow := httptest.NewRequest("GET", "/", nil)
		for _, cookie := range rr.Result().Cookies() {
			follow.AddCookie(cookie)
		}
		id, ok := session.UserID(follow)
		if attempt == 0 && (!ok || id != uid || rr.Header().Get("Location") != "/profile") {
			t.Fatalf("valid login failed: %d %s", rr.Code, rr.Header().Get("Location"))
		}
		if attempt == 1 && (ok || rr.Header().Get("Location") != "/login") {
			t.Fatal("replayed password challenge authenticated")
		}
	}
}

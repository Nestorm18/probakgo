package session

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestSensitiveTOTPFresh(t *testing.T) {
	Init("test-session-key-32-bytes-long!!", false)
	now := time.Date(2026, 6, 29, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()

	if err := SetSensitiveTOTPFresh(rr, req, now.Add(5*time.Minute)); err != nil {
		t.Fatalf("SetSensitiveTOTPFresh: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	for _, c := range rr.Result().Cookies() {
		req2.AddCookie(c)
	}
	if !SensitiveTOTPFresh(req2, now.Add(4*time.Minute)) {
		t.Fatal("fresh TOTP window should be valid before expiry")
	}
	if SensitiveTOTPFresh(req2, now.Add(6*time.Minute)) {
		t.Fatal("fresh TOTP window should expire")
	}
}

func TestUserVersion(t *testing.T) {
	Init("test-session-key-32-bytes-long!!", false)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	if err := SetUserWithVersion(rr, req, "admin", "admin", 4); err != nil {
		t.Fatalf("SetUserWithVersion: %v", err)
	}

	req2 := httptest.NewRequest("GET", "/", nil)
	for _, c := range rr.Result().Cookies() {
		req2.AddCookie(c)
	}
	version, ok := UserVersion(req2)
	if !ok || version != 4 {
		t.Fatalf("UserVersion: got (%d, %t), want (4, true)", version, ok)
	}
}

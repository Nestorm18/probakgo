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
	until, ok := SensitiveTOTPUntil(req2)
	if !ok || !until.Equal(now.Add(5*time.Minute)) {
		t.Fatalf("SensitiveTOTPUntil: got (%v, %t)", until, ok)
	}
	if SensitiveTOTPFresh(req2, now.Add(6*time.Minute)) {
		t.Fatal("fresh TOTP window should expire")
	}
}

func TestTelegramPairingExpires(t *testing.T) {
	Init("test-session-key-32-bytes-long!!", false)
	now := time.Date(2026, 8, 18, 12, 0, 0, 0, time.UTC)
	req := httptest.NewRequest("GET", "/profile", nil)
	rr := httptest.NewRecorder()
	if err := SetTelegramPairing(rr, req, 42, "pair-code", now.Add(10*time.Minute)); err != nil {
		t.Fatalf("SetTelegramPairing: %v", err)
	}
	pairedReq := httptest.NewRequest("POST", "/profile/telegram/pair", nil)
	for _, cookie := range rr.Result().Cookies() {
		pairedReq.AddCookie(cookie)
	}
	if code, ok := GetTelegramPairing(pairedReq, 42, now.Add(9*time.Minute)); !ok || code != "pair-code" {
		t.Fatalf("pairing before expiry: code=%q ok=%t", code, ok)
	}
	if _, ok := GetTelegramPairing(pairedReq, 7, now.Add(9*time.Minute)); ok {
		t.Fatal("Telegram pairing was accepted for another user")
	}
	if _, ok := GetTelegramPairing(pairedReq, 42, now.Add(11*time.Minute)); ok {
		t.Fatal("expired Telegram pairing remained valid")
	}
}

func TestUserVersion(t *testing.T) {
	Init("test-session-key-32-bytes-long!!", false)
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	if err := SetUserWithVersion(rr, req, 1, "admin", "admin", 4); err != nil {
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

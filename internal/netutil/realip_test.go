package netutil

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestTrustedProxyRealIPIgnoresHeadersFromDirectClient(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "198.51.100.10:1234"
	req.Header.Set("X-Forwarded-For", "203.0.113.20")
	req.Header.Set("X-Forwarded-Proto", "https")

	called := false
	h := TrustedProxyRealIP([]string{"127.0.0.1/32"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		if r.RemoteAddr != "198.51.100.10:1234" {
			t.Fatalf("RemoteAddr = %q", r.RemoteAddr)
		}
		if r.Header.Get("X-Forwarded-For") != "" || r.Header.Get("X-Forwarded-Proto") != "" {
			t.Fatal("forwarded headers were not cleared")
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
	if !called {
		t.Fatal("next handler was not called")
	}
}

func TestTrustedProxyRealIPUsesLastUntrustedForwardedAddress(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "127.0.0.1:1234"
	req.Header.Set("X-Forwarded-For", "198.51.100.20, 10.0.0.2")

	h := TrustedProxyRealIP([]string{"127.0.0.1/32", "10.0.0.0/8"})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.RemoteAddr != "198.51.100.20:0" {
			t.Fatalf("RemoteAddr = %q", r.RemoteAddr)
		}
	}))
	h.ServeHTTP(httptest.NewRecorder(), req)
}

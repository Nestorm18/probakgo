package web

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCrossOriginProtectionRequiresExactTrustedOrigin(t *testing.T) {
	protection, err := newCrossOriginProtection([]string{"https://monitor.example"})
	if err != nil {
		t.Fatalf("newCrossOriginProtection: %v", err)
	}
	h := protection.Handler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	for _, tt := range []struct {
		origin string
		want   int
	}{
		{origin: "https://monitor.example", want: http.StatusNoContent},
		{origin: "http://monitor.example", want: http.StatusForbidden},
	} {
		req := httptest.NewRequest(http.MethodPost, "/settings", nil)
		req.Host = "probakgo.example"
		req.Header.Set("Origin", tt.origin)
		req.Header.Set("Sec-Fetch-Site", "cross-site")
		rr := httptest.NewRecorder()
		h.ServeHTTP(rr, req)
		if rr.Code != tt.want {
			t.Errorf("origin %q: got %d, want %d", tt.origin, rr.Code, tt.want)
		}
	}
}

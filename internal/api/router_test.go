package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestLimitRequestBodyRejectsOversizedRequest(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("next handler must not receive an oversized request")
	})
	h := limitRequestBody(8)(next)
	req := httptest.NewRequest(http.MethodPost, "/report/pve", strings.NewReader(strings.Repeat("x", 9)))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status: got %d, want %d", rr.Code, http.StatusRequestEntityTooLarge)
	}
}

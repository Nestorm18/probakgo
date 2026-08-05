package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func TestPWAStaticAssets(t *testing.T) {
	staticFS := fstest.MapFS{
		"sw.js":                {Data: []byte("self.skipWaiting();")},
		"manifest.webmanifest": {Data: []byte(`{"name":"Probakgo"}`)},
	}
	tests := []struct {
		name        string
		handler     http.HandlerFunc
		contentType string
		body        string
	}{
		{"service worker", serveServiceWorker(staticFS), "text/javascript", "self.skipWaiting();"},
		{"manifest", serveManifest(staticFS), "application/manifest+json", `{"name":"Probakgo"}`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder := httptest.NewRecorder()
			tt.handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/", nil))
			if recorder.Code != http.StatusOK {
				t.Fatalf("status: got %d, want 200", recorder.Code)
			}
			if got := recorder.Header().Get("Content-Type"); !strings.HasPrefix(got, tt.contentType) {
				t.Fatalf("content type: got %q, want prefix %q", got, tt.contentType)
			}
			if recorder.Body.String() != tt.body {
				t.Fatalf("body: got %q, want %q", recorder.Body.String(), tt.body)
			}
		})
	}
}

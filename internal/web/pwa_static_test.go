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

func TestStaticFilesDoNotListDirectories(t *testing.T) {
	staticFS := fstest.MapFS{"css/style.css": {Data: []byte("body{}")}}
	handler := http.StripPrefix("/static/", staticFiles(staticFS))
	for path, want := range map[string]int{
		"/static/css/style.css": http.StatusOK,
		"/static/css/":          http.StatusNotFound,
		"/static/css":           http.StatusNotFound,
		"/static/":              http.StatusNotFound,
		"/static/missing.js":    http.StatusNotFound,
	} {
		recorder := httptest.NewRecorder()
		handler.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code != want {
			t.Errorf("%s: got %d, want %d", path, recorder.Code, want)
		}
	}
}

func TestStrictTransportSecurityOnlyForSecureHTTPS(t *testing.T) {
	next := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	for _, tt := range []struct {
		secure bool
		proto  string
		want   bool
	}{
		{secure: true, proto: "https", want: true},
		{secure: true, proto: "", want: false},
		{secure: false, proto: "https", want: false},
	} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		if tt.proto != "" {
			req.Header.Set("X-Forwarded-Proto", tt.proto)
		}
		recorder := httptest.NewRecorder()
		strictTransportSecurity(tt.secure)(next).ServeHTTP(recorder, req)
		if got := recorder.Header().Get("Strict-Transport-Security") != ""; got != tt.want {
			t.Errorf("secure=%v proto=%q: HSTS=%v, want %v", tt.secure, tt.proto, got, tt.want)
		}
	}
}

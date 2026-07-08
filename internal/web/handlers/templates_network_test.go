package webhandlers

import (
	"net/http/httptest"
	"testing"
)

func TestRequestLooksPublicHTTP(t *testing.T) {
	tests := []struct {
		name string
		host string
		want bool
	}{
		{"localhost", "localhost:36748", false},
		{"private ip", "192.168.1.10:36748", false},
		{"netbird cgnat", "100.64.2.10:36748", false},
		{"public ip", "8.8.8.8:36748", true},
		{"public dns", "probakgo.example.com", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("GET", "http://"+tt.host+"/", nil)
			if got := requestLooksPublicHTTP(r); got != tt.want {
				t.Fatalf("requestLooksPublicHTTP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRequestLooksPublicHTTPS(t *testing.T) {
	r := httptest.NewRequest("GET", "http://probakgo.example.com/", nil)
	r.Header.Set("X-Forwarded-Proto", "https")
	if !requestLooksPublicHTTPS(r) {
		t.Fatal("requestLooksPublicHTTPS() = false, want true")
	}
}

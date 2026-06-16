package netutil

import "testing"

func TestHostLooksPublic(t *testing.T) {
	tests := []struct {
		host string
		want bool
	}{
		{"localhost", false},
		{"127.0.0.1:36748", false},
		{"192.168.10.222:36748", false},
		{"10.12.0.5", false},
		{"172.20.1.5", false},
		{"100.64.2.10:36748", false},
		{"100.127.255.254", false},
		{"100.128.0.1", true},
		{"probakgo", false},
		{"probakgo.example.info", true},
		{"8.8.8.8", true},
	}
	for _, tt := range tests {
		if got := HostLooksPublic(tt.host); got != tt.want {
			t.Errorf("HostLooksPublic(%q): want %v, got %v", tt.host, tt.want, got)
		}
	}
}

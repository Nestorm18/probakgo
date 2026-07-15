package webhandlers

import "testing"

func TestIsClientVersionOutdated(t *testing.T) {
	tests := []struct {
		name    string
		release string
		client  string
		want    bool
	}{
		{name: "older", release: "0.0.168", client: "0.0.138", want: true},
		{name: "current", release: "0.0.168", client: "0.0.168", want: false},
		{name: "newer client", release: "0.0.168", client: "0.0.169", want: false},
		{name: "unknown client", release: "0.0.168", client: "", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isClientVersionOutdated(tt.release, tt.client); got != tt.want {
				t.Fatalf("isClientVersionOutdated(%q, %q) = %t, want %t", tt.release, tt.client, got, tt.want)
			}
		})
	}
}

func TestIsClientVersionCurrent(t *testing.T) {
	tests := []struct {
		release string
		client  string
		want    bool
	}{
		{release: "0.0.169", client: "0.0.169", want: true},
		{release: "0.0.169", client: "0.0.170", want: true},
		{release: "0.0.169", client: "0.0.138", want: false},
		{release: "0.0.169", client: "unknown", want: false},
	}

	for _, tt := range tests {
		if got := isClientVersionCurrent(tt.release, tt.client); got != tt.want {
			t.Errorf("isClientVersionCurrent(%q, %q) = %t, want %t", tt.release, tt.client, got, tt.want)
		}
	}
}

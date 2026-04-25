package main

import "testing"

func TestServerTypeFromContent(t *testing.T) {
	tests := []struct {
		content string
		want    string
	}{
		{"Proxmox Virtual Environment 8.2\n", "pve"},
		{"PROXMOX VIRTUAL ENVIRONMENT", "pve"},
		{"Proxmox Backup Server 3.1\n", "pbs"},
		{"proxmox backup server 2.4", "pbs"},
		{"Debian GNU/Linux 12 \\n \\l\n", "unknown"},
		{"Ubuntu 22.04", "unknown"},
		{"", "unknown"},
	}
	for _, tt := range tests {
		got := serverTypeFromContent(tt.content)
		if got != tt.want {
			t.Errorf("serverTypeFromContent(%q) = %q, want %q", tt.content, got, tt.want)
		}
	}
}

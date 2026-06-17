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

func TestParseSwapInfo(t *testing.T) {
	info := parseSwapInfo("MemTotal: 1024 kB\nSwapTotal: 2097152 kB\nSwapFree: 1572864 kB\n")
	if !info.Enabled {
		t.Fatal("expected swap enabled")
	}
	if info.Total != 2097152*1024 {
		t.Fatalf("total = %d, want %d", info.Total, int64(2097152*1024))
	}
	if info.Used != 524288*1024 {
		t.Fatalf("used = %d, want %d", info.Used, int64(524288*1024))
	}
}

func TestParseSwapInfoDisabled(t *testing.T) {
	info := parseSwapInfo("MemTotal: 1024 kB\nSwapTotal: 0 kB\nSwapFree: 0 kB\n")
	if info.Enabled || info.Total != 0 || info.Used != 0 {
		t.Fatalf("unexpected swap info: %+v", info)
	}
}

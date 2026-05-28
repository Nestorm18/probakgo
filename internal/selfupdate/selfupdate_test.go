package selfupdate

import "testing"

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		name    string
		remote  string
		current string
		want    int
		wantOK  bool
	}{
		{"newer patch", "0.0.51", "0.0.50", 1, true},
		{"remote with v prefix", "v0.0.51", "0.0.50", 1, true},
		{"same with prefix", "v0.0.50", "0.0.50", 0, true},
		{"older remote", "0.0.49", "0.0.50", -1, true},
		{"local has extra zero", "0.0.50", "0.0.50.0", 0, true},
		{"pre release suffix ignored", "0.0.51-beta.1", "0.0.50", 1, true},
		{"invalid remote", "latest", "0.0.50", 0, false},
		{"invalid current", "0.0.51", "local", 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := compareVersions(tt.remote, tt.current)
			if ok != tt.wantOK || got != tt.want {
				t.Fatalf("compareVersions(%q, %q) = %d, %v; want %d, %v", tt.remote, tt.current, got, ok, tt.want, tt.wantOK)
			}
		})
	}
}

func TestIsNewer(t *testing.T) {
	newer, ok := IsNewer("v0.0.59", "0.0.58")
	if !ok || !newer {
		t.Fatalf("IsNewer newer = %v, %v; want true, true", newer, ok)
	}

	newer, ok = IsNewer("v0.0.58", "0.0.58")
	if !ok || newer {
		t.Fatalf("IsNewer same = %v, %v; want false, true", newer, ok)
	}
}

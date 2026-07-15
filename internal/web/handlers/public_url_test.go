package webhandlers

import (
	"net/http/httptest"
	"testing"
)

func TestNormalizePublicAPIURL(t *testing.T) {
	tests := []struct {
		input string
		want  string
		ok    bool
	}{
		{input: "https://probakgo.example/", want: "https://probakgo.example", ok: true},
		{input: "http://192.168.1.10:36748", want: "http://192.168.1.10:36748", ok: true},
		{input: "https://user:pass@probakgo.example", ok: false},
		{input: "https://probakgo.example/path", ok: false},
		{input: "https://probakgo.example?next=evil", ok: false},
		{input: "javascript:alert(1)", ok: false},
	}
	for _, tt := range tests {
		got, err := normalizePublicAPIURL(tt.input)
		if (err == nil) != tt.ok || got != tt.want {
			t.Errorf("normalizePublicAPIURL(%q) = %q, %v; want %q, ok=%t", tt.input, got, err, tt.want, tt.ok)
		}
	}
}

func TestInstallerAPIURL(t *testing.T) {
	privateReq := httptest.NewRequest("POST", "http://192.168.1.10:36748/api-keys", nil)
	if got, err := installerAPIURL(privateReq, ""); err != nil || got != "http://192.168.1.10:36748" {
		t.Fatalf("private installer URL = %q, %v", got, err)
	}

	publicReq := httptest.NewRequest("POST", "https://attacker.example/api-keys", nil)
	if _, err := installerAPIURL(publicReq, ""); err == nil {
		t.Fatal("expected public request host without configured URL to be rejected")
	}
	if got, err := installerAPIURL(publicReq, "https://probakgo.example/"); err != nil || got != "https://probakgo.example" {
		t.Fatalf("configured installer URL = %q, %v", got, err)
	}
}

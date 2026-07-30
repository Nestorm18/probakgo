package main

import (
	"strings"
	"testing"
)

func TestMergeEnvContentUpdatesProvidedValues(t *testing.T) {
	input := strings.Join([]string{
		"# probakgo client configuration",
		"API_KEY=pbk-old",
		"API_URL=http://old.example",
		"PROXMOX_TOKEN=root@pam!probakgo-client",
		"PROXMOX_SECRET=keep-me",
		"",
	}, "\n")

	got, changed := mergeEnvContent(input, map[string]string{
		"API_KEY":        "pbk-new",
		"API_URL":        "https://probakgo.example",
		"PROXMOX_TOKEN":  "",
		"PROXMOX_SECRET": "",
	})

	if !changed {
		t.Fatal("expected env content to change")
	}
	if !strings.Contains(got, "API_KEY=pbk-new") {
		t.Fatalf("API_KEY was not updated:\n%s", got)
	}
	if !strings.Contains(got, "API_URL=https://probakgo.example") {
		t.Fatalf("API_URL was not updated:\n%s", got)
	}
	if !strings.Contains(got, "PROXMOX_SECRET=keep-me") {
		t.Fatalf("existing Proxmox secret should be preserved:\n%s", got)
	}
}

func TestMergeEnvContentAddsMissingProvidedValue(t *testing.T) {
	got, changed := mergeEnvContent("API_URL=http://old.example\n", map[string]string{
		"API_KEY": "pbk-new",
	})

	if !changed {
		t.Fatal("expected env content to change")
	}
	if !strings.Contains(got, "API_KEY=pbk-new") {
		t.Fatalf("API_KEY was not added:\n%s", got)
	}
}

func TestParsePBSGenerateTokenOutput(t *testing.T) {
	const secret = "d63e505a-e3ec-449a-9bc7-1da610d4ccde"
	tests := []string{
		`{"value":"` + secret + `"}`,
		"Result: {\n  \"tokenid\": \"root@pam!probakgo-client\",\n  \"value\": \"" + secret + "\"\n}\n",
	}
	for _, output := range tests {
		if got := parsePBSGenerateTokenOutput([]byte(output)); got != secret {
			t.Fatalf("parsePBSGenerateTokenOutput() = %q, want %q for %q", got, secret, output)
		}
	}
}

func TestHookScriptDefersReportUntilAfterJobEnd(t *testing.T) {
	if !strings.Contains(hookScript, "systemd-run") || !strings.Contains(hookScript, "--on-active=5s") {
		t.Fatal("hook must schedule the report outside the vzdump task scope")
	}
	if !strings.Contains(hookScript, `"run-report"`) {
		t.Fatal("hook must provide a detached report entry point")
	}
	if !strings.Contains(hookScript, `2>&1 &`) {
		t.Fatal("hook must retain a background fallback when systemd scheduling is unavailable")
	}
	pendingWrite := strings.Index(hookScript, `echo "$(date '+%Y-%m-%d %H:%M:%S')" > "$PENDING_FILE"`)
	schedule := strings.Index(hookScript, "systemd-run --quiet")
	if pendingWrite < 0 || schedule < 0 || pendingWrite > schedule {
		t.Fatal("hook must persist the pending marker before scheduling the detached report")
	}
}

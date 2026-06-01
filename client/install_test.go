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

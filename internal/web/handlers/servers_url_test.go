package webhandlers

import (
	"testing"

	"probakgo/internal/domain"
)

func TestServerURLForDoesNotUseAnotherKeysURL(t *testing.T) {
	urls := buildServerURLMap([]domain.APIKey{
		{ID: 1, ServerName: "pbs", ServerURL: ""},
		{ID: 2, ServerName: "pbs", ServerURL: "https://other-pbs.example:8007"},
	}, nil)

	if got := serverURLFor(1, "pbs", urls); got != "" {
		t.Fatalf("server URL = %q, want empty URL from linked API key", got)
	}
}

func TestServerURLForKeepsLegacyNameFallback(t *testing.T) {
	urls := buildServerURLMap([]domain.APIKey{
		{ID: 2, ServerName: "pbs", ServerURL: "https://pbs.example:8007"},
	}, nil)

	if got := serverURLFor(0, "pbs", urls); got != "https://pbs.example:8007" {
		t.Fatalf("legacy server URL = %q, want name fallback", got)
	}
}

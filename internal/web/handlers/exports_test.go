package webhandlers

import "testing"

func TestEscapeCSVFormula(t *testing.T) {
	for _, input := range []string{"=SUM(A1:A2)", "+cmd", "-1+1", "@HYPERLINK(\"https://example.test\")", "  =SUM(A1:A2)"} {
		if got := escapeCSVFormula(input); got != "'"+input {
			t.Errorf("escapeCSVFormula(%q) = %q", input, got)
		}
	}
	if got := escapeCSVFormula("server-01"); got != "server-01" {
		t.Errorf("safe value changed: %q", got)
	}
}

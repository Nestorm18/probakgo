package webhandlers

import (
	"mime"
	"testing"
)

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

func TestAttachmentDispositionEncodesClientHostnames(t *testing.T) {
	for _, name := range []string{`reportes_pve"; x=1_20261007.csv`, "reportes_nodo-ñ_20261007.csv", "probakgo_data_20261007_101010.db"} {
		header := attachmentDisposition(name)
		disposition, params, err := mime.ParseMediaType(header)
		if err != nil || disposition != "attachment" || params["filename"] != name {
			t.Fatalf("%q: header %q parsed as %q %v %v", name, header, disposition, params, err)
		}
	}
}

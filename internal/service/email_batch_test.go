package service

import (
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestRenderAlertDigestEmail(t *testing.T) {
	active := []domain.Alert{{ServerName: "Active server", Title: "Disk full", Message: "<script>bad</script>"}}
	resolved := []domain.Alert{{ServerName: "Recovered server", Title: "Backup recovered"}}
	html := renderAlertDigestEmail(active, resolved, time.Now())
	for _, want := range []string{"Active server", "Recovered server", "Disk full", "Backup recovered", "&lt;script&gt;"} {
		if !strings.Contains(html, want) {
			t.Errorf("missing %q", want)
		}
	}
	if strings.Count(html, "<body") != 1 || strings.Count(html, "</html>") != 1 || strings.Contains(html, "<script>") {
		t.Fatal("invalid or unsafe digest document")
	}
}

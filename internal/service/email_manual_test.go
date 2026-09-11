package service

import (
	"strings"
	"testing"
	"time"
)

func TestManualReportDoesNotSkipDisabledSchedule(t *testing.T) {
	_, st := openTestStore(t)
	cfg, err := st.GetEmailConfig(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	cfg.IsEnabled = false
	cfg.AlertEmailBatchMinutes = 5
	cfg.SMTPUser = ""
	cfg.SMTPPass = ""
	if err := st.UpsertEmailConfig(t.Context(), *cfg); err != nil {
		t.Fatal(err)
	}
	rep := NewReport(st, time.UTC)
	if err := SendDailyReport(st, rep); err != nil {
		t.Fatalf("disabled scheduled report should skip: %v", err)
	}
	// A manual send must attempt delivery and report missing credentials,
	// rather than return success without sending or wait for the alert batch.
	if err := SendDailyReportTest(st, rep); err == nil || !strings.Contains(err.Error(), "SMTP credentials not configured") {
		t.Fatalf("manual report did not attempt immediate sending: %v", err)
	}
}

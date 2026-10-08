package service

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestSMTPBackoffGrowsAndResets(t *testing.T) {
	var b smtpBackoff
	now := time.Now()
	failure := errors.New("535 authentication failed")

	b.record(failure, now)
	if !b.active(now.Add(59*time.Second)) || b.active(now.Add(61*time.Second)) {
		t.Fatalf("first failure should wait one minute, got until %s", b.until.Sub(now))
	}
	for range 10 {
		b.record(failure, now)
	}
	if b.delay != smtpBackoffMax {
		t.Fatalf("delay should be capped at %s, got %s", smtpBackoffMax, b.delay)
	}
	b.record(nil, now)
	if b.active(now) || b.delay != 0 {
		t.Fatal("a successful delivery should clear the backoff")
	}
}

func TestImmediateEmailWaitsAfterSMTPFailure(t *testing.T) {
	// Start from a clean shared backoff and leave it clean for other tests.
	immediateEmailBackoff.record(nil, time.Now())
	t.Cleanup(func() { immediateEmailBackoff.record(nil, time.Now()) })

	// A closed port makes every delivery attempt fail quickly.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	ctx := context.Background()
	_, st := openTestStore(t)
	alert := domain.Alert{ID: "pve_heartbeat:pve:1", Type: domain.AlertTypePVEHeartbeat, Severity: domain.AlertSeverityCritical, Title: "Servidor offline"}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatal(err)
	}
	cfg := &domain.EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: port, SMTPUser: "alerts@example.com", SMTPPass: "secret"}

	if err := dispatchImmediateEmail(ctx, st, cfg, []string{"ops@example.com"}, []domain.Alert{alert}, nil, time.Now()); err == nil {
		t.Fatal("expected the first delivery attempt to fail")
	}
	if !immediateEmailBackoff.active(time.Now()) {
		t.Fatal("an SMTP failure should start the backoff")
	}
	if err := dispatchImmediateEmail(ctx, st, cfg, []string{"ops@example.com"}, []domain.Alert{alert}, nil, time.Now()); err != nil {
		t.Fatalf("delivery during backoff should be skipped, got %v", err)
	}
	sent, err := st.ListCriticalEmailSentAlertIDs(ctx, []string{alert.ID})
	if err != nil || sent[alert.ID] {
		t.Fatalf("a skipped alert must stay pending: sent=%v err=%v", sent, err)
	}
}

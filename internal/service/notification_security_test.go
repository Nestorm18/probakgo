package service

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func TestSMTPGreetingCannotBlockPastDeadline(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		var b [1]byte
		_, _ = conn.Read(b[:]) // no SMTP greeting; returns when the client times out
	}()
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
	defer cancel()
	start := time.Now()
	err = sendSMTP(ctx, &domain.EmailConfig{SMTPHost: "127.0.0.1", SMTPPort: listener.Addr().(*net.TCPAddr).Port, SMTPUser: "sender@example.com"}, []string{"reader@example.com"}, "test", "test")
	if err == nil || time.Since(start) > 2*time.Second {
		t.Fatalf("SMTP did not respect deadline: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("SMTP connection was not closed")
	}
}

func offlineServer(t *testing.T, st *store.Store) (int64, string) {
	t.Helper()
	ctx := context.Background()
	id, err := st.UpsertPVEServer(ctx, "silent-node", "192.0.2.1", "", "test", "silent-machine")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{ServerType: "pve", ServerID: id, Hostname: "silent-node", LastSeenAt: time.Now().Add(-24 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	return id, fmt.Sprintf("pve_heartbeat:pve:%d", id)
}

func TestSchedulerDetectsOfflineServerWithoutIncomingRequests(t *testing.T) {
	st := newPushTestStore(t)
	oldPush, oldTelegram := GetPushSender(), GetTelegramSender()
	SetPushSender(nil)
	SetTelegramSender(nil)
	defer SetPushSender(oldPush)
	defer SetTelegramSender(oldTelegram)
	_, alertID := offlineServer(t, st)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { defer close(done); runAlertScheduler(ctx, st, NewReport(st, time.UTC), 10*time.Millisecond) }()
	defer func() { cancel(); <-done }()
	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-deadline:
			t.Fatal("periodic evaluator never persisted offline alert")
		case <-ticker.C:
			alerts, err := st.ListPresentAlerts(ctx)
			if err != nil {
				t.Fatal(err)
			}
			for _, a := range alerts {
				if a.ID == alertID {
					return
				}
			}
		}
	}
}

func TestConcurrentNotificationsDoNotDuplicatePush(t *testing.T) {
	st := newPushTestStore(t)
	ctx := context.Background()
	_, alertID := offlineServer(t, st)
	uid, err := st.CreateUser(ctx, "reader", "hash", "reader")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.AddPushSubscription(ctx, store.PushSubscription{UserID: uid, Endpoint: "https://example.com/push", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatal(err)
	}
	sender := NewPushSender(st)
	if _, _, err := sender.EnsureVAPIDKeys(ctx, "mailto:test@example.com"); err != nil {
		t.Fatal(err)
	}
	started, release := make(chan struct{}, 2), make(chan struct{})
	var attempts atomic.Int32
	sender.deliverFn = func(ctx context.Context, _ *store.PushConfig, _ store.PushSubscription, _ []byte) error {
		attempts.Add(1)
		started <- struct{}{}
		select {
		case <-release:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	oldPush, oldTelegram := GetPushSender(), GetTelegramSender()
	SetPushSender(sender)
	SetTelegramSender(nil)
	defer SetPushSender(oldPush)
	defer SetTelegramSender(oldTelegram)
	rep := NewReport(st, time.UTC)
	done := make(chan error, 1)
	go func() { done <- SendImmediateCriticalAlerts(st, rep) }()
	defer func() {
		select {
		case <-release:
		default:
			close(release)
		}
	}()
	select {
	case <-started:
	case <-time.After(2 * time.Second):
		t.Fatal("push was not attempted")
	}
	if err := SendImmediateCriticalAlerts(st, rep); err != nil {
		t.Fatal(err)
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if err := SendImmediateCriticalAlerts(st, rep); err != nil {
		t.Fatal(err)
	}
	if attempts.Load() != 1 {
		t.Fatalf("push sent %d times", attempts.Load())
	}
	sent, err := st.ListCriticalPushSentAlertIDs(ctx, []string{alertID})
	if err != nil || !sent[alertID] {
		t.Fatalf("delivery state not persisted: %v %v", sent, err)
	}
}

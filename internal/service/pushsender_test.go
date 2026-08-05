package service

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"sync"
	"testing"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
	"probakgo/internal/store"
)

func newPushTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test database: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	st, err := store.NewEncrypted(db, "test-master-key-needs-to-be-32+bytes")
	if err != nil {
		t.Fatalf("new encrypted test store: %v", err)
	}
	return st
}

// I can't easily run webpush.SendNotification in unit tests (it would hit a
// real push service), so the push sender exposes a `deliver` hook the tests
// can swap. This is much simpler than refactoring the sender behind an
// interface and keeps production behaviour untouched.

func TestBuildPushPayload_SingleAlert(t *testing.T) {
	payload := buildPushPayload([]domain.Alert{{
		ID:         "pve_heartbeat:pve:42",
		Title:      "Servidor offline",
		Message:    "No se recibe senal desde hace 22m",
		ServerName: "pve-edge-1",
		Severity:   domain.AlertSeverityCritical,
	}}, "/alerts", false)

	var p PushNotification
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Title != "pve-edge-1" {
		t.Fatalf("title: want server name for single alert, got %q", p.Title)
	}
	if p.Body != "No se recibe senal desde hace 22m" {
		t.Fatalf("body: got %q", p.Body)
	}
	if p.Tag != "pve_heartbeat:pve:42" {
		t.Fatalf("tag should be the alert id so toasts replace each other, got %q", p.Tag)
	}
	if p.Severity != domain.AlertSeverityCritical {
		t.Fatalf("severity: got %q", p.Severity)
	}
	if p.URL != "/alerts" {
		t.Fatalf("url: got %q", p.URL)
	}
}

func TestBuildPushPayload_MultipleAlerts(t *testing.T) {
	payload := buildPushPayload([]domain.Alert{
		{ID: "a", Title: "t1", Message: "m1", Severity: domain.AlertSeverityWarning},
		{ID: "b", Title: "t2", Message: "m2", Severity: domain.AlertSeverityCritical},
	}, "/alerts", false)
	var p PushNotification
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !strings.Contains(p.Title, "critica") {
		t.Fatalf("expected title to mention critical count, got %q", p.Title)
	}
	if !strings.Contains(p.Body, "2 alerta(s)") {
		t.Fatalf("body should mention 2 alerts, got %q", p.Body)
	}
	if !strings.HasPrefix(p.Tag, "batch:") {
		t.Fatalf("batch tag should start with 'batch:', got %q", p.Tag)
	}
}

func TestRedactEndpoint(t *testing.T) {
	cases := map[string]string{
		"https://fcm.googleapis.com/fcm/send/abc123?token=secret": "https://fcm.googleapis.com/fcm/send/abc123",
		"https://updates.push.services.mozilla.com/wpush/v2/abc":  "https://updates.push.services.mozilla.com/wpush/v2/abc",
		strings.Repeat("x", 80):                                   strings.Repeat("x", 64) + "...",
	}
	for in, want := range cases {
		if got := redactEndpoint(in); got != want {
			t.Fatalf("redactEndpoint(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestEnsureVAPIDKeys_GeneratesOnce(t *testing.T) {
	ctx := context.Background()
	st := newPushTestStore(t)
	sender := NewPushSender(st)

	pub1, priv1, err := sender.EnsureVAPIDKeys(ctx, "mailto:alerts@example.com")
	if err != nil {
		t.Fatalf("first ensure: %v", err)
	}
	if pub1 == "" || priv1 == "" {
		t.Fatalf("expected non-empty keys, got pub=%q priv=%q", pub1, priv1)
	}

	pub2, priv2, err := sender.EnsureVAPIDKeys(ctx, "mailto:alerts@example.com")
	if err != nil {
		t.Fatalf("second ensure: %v", err)
	}
	if pub1 != pub2 || priv1 != priv2 {
		t.Fatalf("expected stable keys, got new pub=%q priv=%q", pub2, priv2)
	}
}

func TestEnsureVAPIDKeys_ConcurrentCallsUseSamePair(t *testing.T) {
	st := newPushTestStore(t)
	sender := NewPushSender(st)
	type pair struct{ public, private string }
	results := make(chan pair, 12)
	errs := make(chan error, 12)
	var wg sync.WaitGroup
	for range 12 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			public, private, err := sender.EnsureVAPIDKeys(context.Background(), "mailto:test@example.com")
			if err != nil {
				errs <- err
				return
			}
			results <- pair{public, private}
		}()
	}
	wg.Wait()
	close(results)
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent ensure: %v", err)
	}
	var first pair
	for result := range results {
		if first.public == "" {
			first = result
			continue
		}
		if result != first {
			t.Fatalf("concurrent calls generated different VAPID pairs")
		}
	}
}

func TestBuildPushPayload_ResolutionIsNotCritical(t *testing.T) {
	payload := buildPushPayload([]domain.Alert{{
		ID: "disk:pve:42", Title: "Disco lleno", ServerName: "pve-1", Severity: domain.AlertSeverityCritical,
	}}, "/alerts", true)
	var p PushNotification
	if err := json.Unmarshal(payload, &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Severity != "info" || !strings.Contains(p.Body, "resuelta") {
		t.Fatalf("resolution payload should clear critical presentation: %+v", p)
	}
	if p.Tag != "disk:pve:42" {
		t.Fatalf("resolution must replace the active notification, tag=%q", p.Tag)
	}
}

func TestSendAlerts_SkipsWhenNoVAPID(t *testing.T) {
	ctx := context.Background()
	st := newPushTestStore(t)
	uid, err := st.CreateUser(ctx, "alice", "hash", "reader")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	if _, err := st.AddPushSubscription(ctx, store.PushSubscription{
		UserID: uid, Endpoint: "https://example.com/1", P256DH: "x", Auth: "y",
	}); err != nil {
		t.Fatalf("add sub: %v", err)
	}
	sender := NewPushSender(st)
	// No VAPID keys yet, no delivery attempt should be made and no panic.
	sender.SendAlerts(ctx, []domain.Alert{{ID: "x", Title: "t"}}, "/alerts")
}

func TestSendAlerts_PrunesExpiredEndpoints(t *testing.T) {
	ctx := context.Background()
	st := newPushTestStore(t)
	uid, err := st.CreateUser(ctx, "alice", "hash", "reader")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	// Pre-create VAPID keys so the sender is "configured".
	sender := NewPushSender(st)
	if _, _, err := sender.EnsureVAPIDKeys(ctx, "mailto:test@example.com"); err != nil {
		t.Fatalf("vapid: %v", err)
	}
	subID, err := st.AddPushSubscription(ctx, store.PushSubscription{
		UserID: uid, Endpoint: "https://push.example.com/expired", P256DH: "x", Auth: "y",
	})
	if err != nil {
		t.Fatalf("add sub: %v", err)
	}

	// Override deliver to simulate the push service responding 410 Gone.
	sender.deliverFn = func(_ context.Context, _ *store.PushConfig, _ store.PushSubscription, _ []byte) error {
		return pushHTTPError{status: http.StatusGone}
	}
	sender.SendAlerts(ctx, []domain.Alert{{ID: "x", Title: "t"}}, "/alerts")

	subs, _ := st.ListPushSubscriptionsByUser(ctx, uid)
	if len(subs) != 0 {
		t.Fatalf("expected subscription %d to be pruned after 410, still have %d", subID, len(subs))
	}
}

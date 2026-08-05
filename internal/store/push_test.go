package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	dbpkg "probakgo/internal/db"
	"probakgo/internal/domain"
)

func newEncryptedTestDB(t *testing.T) *Store {
	t.Helper()
	db, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	st, err := NewEncrypted(db, "this-is-a-32byte-master-key-test")
	if err != nil {
		t.Fatalf("encrypted store: %v", err)
	}
	return st
}

func mustCreateUser(t *testing.T, st *Store, username string) int64 {
	t.Helper()
	id, err := st.CreateUser(context.Background(), username, "$2a$10$fakehash", "reader")
	if err != nil {
		t.Fatalf("CreateUser(%s): %v", username, err)
	}
	return id
}

func TestPushConfig_RoundTripEncrypted(t *testing.T) {
	ctx := context.Background()
	st := newEncryptedTestDB(t)

	cfg, err := st.GetPushConfig(ctx)
	if err != nil {
		t.Fatalf("initial GetPushConfig: %v", err)
	}
	if cfg.PublicKey != "" || cfg.PrivateKey != "" {
		t.Fatalf("expected empty initial config, got %+v", cfg)
	}

	original := PushConfig{
		PublicKey:  "BPubKey",
		PrivateKey: "super-secret-vapid",
		Subject:    "mailto:alerts@example.com",
	}
	if err := st.UpsertPushConfig(ctx, original); err != nil {
		t.Fatalf("UpsertPushConfig: %v", err)
	}

	got, err := st.GetPushConfig(ctx)
	if err != nil {
		t.Fatalf("GetPushConfig: %v", err)
	}
	if got.PublicKey != original.PublicKey {
		t.Fatalf("public key roundtrip: got %q want %q", got.PublicKey, original.PublicKey)
	}
	if got.PrivateKey != original.PrivateKey {
		t.Fatalf("private key roundtrip: got %q want %q", got.PrivateKey, original.PrivateKey)
	}
	if got.Subject != original.Subject {
		t.Fatalf("subject roundtrip: got %q want %q", got.Subject, original.Subject)
	}

	// Ensure the on-disk private key is actually encrypted (not the plaintext).
	var stored string
	if err := st.db.QueryRowContext(ctx, `SELECT private_key FROM push_config WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("read raw: %v", err)
	}
	if stored == original.PrivateKey {
		t.Fatalf("private key was stored in plaintext: %q", stored)
	}
}

func TestAddPushSubscription_RefreshesSameEndpoint(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	uid := mustCreateUser(t, st, "alice")

	first, err := st.AddPushSubscription(ctx, PushSubscription{
		UserID: uid, Endpoint: "https://push.example.com/abc",
		P256DH: "p1", Auth: "a1", UserAgent: "Firefox/120",
	})
	if err != nil {
		t.Fatalf("add first: %v", err)
	}
	second, err := st.AddPushSubscription(ctx, PushSubscription{
		UserID: uid, Endpoint: "https://push.example.com/abc",
		P256DH: "p2", Auth: "a2", UserAgent: "Firefox/121",
	})
	if err != nil {
		t.Fatalf("add second: %v", err)
	}
	if first != second {
		t.Fatalf("expected same row id for duplicate endpoint, got first=%d second=%d", first, second)
	}

	subs, err := st.ListPushSubscriptionsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 1 {
		t.Fatalf("want 1 subscription after upsert, got %d", len(subs))
	}
	if subs[0].P256DH != "p2" || subs[0].Auth != "a2" || subs[0].UserAgent != "Firefox/121" {
		t.Fatalf("subscription not refreshed: %+v", subs[0])
	}
}

func TestAddPushSubscription_RejectsUnsafeEndpoint(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	uid := mustCreateUser(t, st, "alice")
	for _, endpoint := range []string{
		"http://push.example.com/x",
		"https://localhost/x",
		"https://127.0.0.1/x",
		"https://10.0.0.1/x",
		"https://user:pass@push.example.com/x",
	} {
		_, err := st.AddPushSubscription(ctx, PushSubscription{
			UserID: uid, Endpoint: endpoint, P256DH: "p", Auth: "a",
		})
		if !errors.Is(err, ErrInvalidPushSubscription) {
			t.Errorf("endpoint %q: got %v, want ErrInvalidPushSubscription", endpoint, err)
		}
	}
}

func TestAddPushSubscription_LimitsDevicesPerUser(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	uid := mustCreateUser(t, st, "alice")
	for i := 0; i < maxPushSubscriptionsPerUser+3; i++ {
		if _, err := st.AddPushSubscription(ctx, PushSubscription{
			UserID: uid, Endpoint: fmt.Sprintf("https://push.example.com/%d", i), P256DH: "p", Auth: "a",
		}); err != nil {
			t.Fatalf("add subscription %d: %v", i, err)
		}
	}
	subs, err := st.ListPushSubscriptionsByUser(ctx, uid)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != maxPushSubscriptionsPerUser {
		t.Fatalf("got %d subscriptions, want cap of %d", len(subs), maxPushSubscriptionsPerUser)
	}
}

func TestRemovePushSubscription_OnlyOwner(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alice := mustCreateUser(t, st, "alice")
	bob := mustCreateUser(t, st, "bob")

	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: alice, Endpoint: "https://push.example.com/x", P256DH: "p", Auth: "a"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := st.RemovePushSubscription(ctx, bob, "https://push.example.com/x"); err != nil {
		t.Fatalf("remove by non-owner: %v", err)
	}
	subs, _ := st.ListPushSubscriptionsByUser(ctx, alice)
	if len(subs) != 1 {
		t.Fatalf("non-owner should not be able to delete; subs=%d", len(subs))
	}
	if err := st.RemovePushSubscription(ctx, alice, "https://push.example.com/x"); err != nil {
		t.Fatalf("remove by owner: %v", err)
	}
	subs, _ = st.ListPushSubscriptionsByUser(ctx, alice)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subs after owner delete, got %d", len(subs))
	}
}

func TestListAllPushSubscriptions_FansOut(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alice := mustCreateUser(t, st, "alice")
	bob := mustCreateUser(t, st, "bob")
	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: alice, Endpoint: "https://a/1", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatalf("add alice: %v", err)
	}
	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: bob, Endpoint: "https://b/1", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatalf("add bob: %v", err)
	}
	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: bob, Endpoint: "https://b/2", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatalf("add bob 2: %v", err)
	}

	all, err := st.ListAllPushSubscriptions(ctx)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("want 3 subs, got %d", len(all))
	}

	n, err := st.CountPushSubscriptions(ctx)
	if err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("count: want 3, got %d", n)
	}
}

func TestDeletePushSubscriptionsByUser(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alice := mustCreateUser(t, st, "alice")
	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: alice, Endpoint: "https://a/1", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := st.DeletePushSubscriptionsByUser(ctx, alice); err != nil {
		t.Fatalf("delete: %v", err)
	}
	subs, _ := st.ListPushSubscriptionsByUser(ctx, alice)
	if len(subs) != 0 {
		t.Fatalf("expected 0 subs after delete, got %d", len(subs))
	}
}

func TestDisablingUserDeletesPushSubscriptions(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alice := mustCreateUser(t, st, "alice")
	if _, err := st.AddPushSubscription(ctx, PushSubscription{
		UserID: alice, Endpoint: "https://push.example.com/alice", P256DH: "x", Auth: "y",
	}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := st.SetUserActive(ctx, alice, false); err != nil {
		t.Fatalf("disable user: %v", err)
	}
	subs, err := st.ListPushSubscriptionsByUser(ctx, alice)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(subs) != 0 {
		t.Fatalf("disabled user retained %d push subscription(s)", len(subs))
	}
}

func TestPushDeliveryStateIsIndependentFromEmail(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alert := domain.Alert{
		ID: "disk:pve:42", Severity: domain.AlertSeverityCritical, Title: "Disk full", ServerType: "pve", ServerID: 42,
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatalf("sync active alert: %v", err)
	}
	if err := st.MarkAlertCriticalPushesSent(ctx, []string{alert.ID}, time.Now()); err != nil {
		t.Fatalf("mark push sent: %v", err)
	}
	emailSent, err := st.ListCriticalEmailSentAlertIDs(ctx, []string{alert.ID})
	if err != nil {
		t.Fatalf("list email state: %v", err)
	}
	if emailSent[alert.ID] {
		t.Fatal("marking push sent must not mark email sent")
	}
	if err := st.SyncAlertStates(ctx, nil); err != nil {
		t.Fatalf("sync resolved alert: %v", err)
	}
	pushResolved, err := st.ListPendingAlertResolutionPushes(ctx)
	if err != nil {
		t.Fatalf("list push resolutions: %v", err)
	}
	emailResolved, err := st.ListPendingAlertResolutionEmails(ctx)
	if err != nil {
		t.Fatalf("list email resolutions: %v", err)
	}
	if len(pushResolved) != 1 || len(emailResolved) != 0 {
		t.Fatalf("resolution state crossed channels: push=%d email=%d", len(pushResolved), len(emailResolved))
	}
}

// TestDeleteUser_CascadesPushSubscriptions proves the ON DELETE CASCADE on
// push_subscriptions.user_id actually fires when a user row is dropped.
// Without the cascade, deleting a user would leave orphan endpoints behind
// and the next ListAllPushSubscriptions would re-try every dead device.
func TestDeleteUser_CascadesPushSubscriptions(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	alice := mustCreateUser(t, st, "alice")
	if _, err := st.AddPushSubscription(ctx, PushSubscription{UserID: alice, Endpoint: "https://a/1", P256DH: "x", Auth: "y"}); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := st.DeleteUser(ctx, alice); err != nil {
		t.Fatalf("delete user: %v", err)
	}
	all, err := st.ListAllPushSubscriptions(ctx)
	if err != nil {
		t.Fatalf("list all: %v", err)
	}
	if len(all) != 0 {
		t.Fatalf("expected cascade to drop subscriptions, still have %d", len(all))
	}
}

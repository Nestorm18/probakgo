package service

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"probakgo/internal/store"
)

func TestPushRedirectCannotReachLoopback(t *testing.T) {
	var reached atomic.Bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reached.Store(true)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	start := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/internal", http.StatusFound)
	}))
	defer start.Close()
	// The test trusts only its local HTTPS fixture. Host routing below simulates
	// a public push endpoint; the redirect target uses the ordinary dialer.
	transport := start.Client().Transport.(*http.Transport).Clone()
	defer transport.CloseIdleConnections()
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		if addr == "example.com:443" {
			addr = start.Listener.Addr().String()
		}
		return (&net.Dialer{}).DialContext(ctx, network, addr)
	}
	st := newPushTestStore(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	uid, err := st.CreateUser(ctx, "audit-push", "hash", "reader")
	if err != nil {
		t.Fatal(err)
	}
	key, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	sub := store.PushSubscription{UserID: uid, Endpoint: "https://example.com/send", P256DH: base64.RawURLEncoding.EncodeToString(key.PublicKey().Bytes()), Auth: base64.RawURLEncoding.EncodeToString(make([]byte, 16))}
	if _, err := st.AddPushSubscription(ctx, sub); err != nil {
		t.Fatalf("endpoint rejected: %v", err)
	}
	sender := NewPushSender(st)
	sender.client.Transport = transport
	if _, _, err := sender.EnsureVAPIDKeys(ctx, "mailto:audit@example.invalid"); err != nil {
		t.Fatal(err)
	}
	err = sender.SendTest(ctx, sub, "/alerts")
	t.Logf("accepted subscription redirected HTTPS -> internal HTTP: reached=%v delivery_error=%v", reached.Load(), err)
	if reached.Load() || err == nil {
		t.Fatal("private redirect was followed or treated as delivered")
	}
}

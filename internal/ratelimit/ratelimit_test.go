package ratelimit

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAllowKeyLimitsIndependently(t *testing.T) {
	l := New(2, time.Minute)

	if !l.AllowKey("key-a") || !l.AllowKey("key-a") {
		t.Fatal("first two requests for key-a should be allowed")
	}
	if l.AllowKey("key-a") {
		t.Fatal("third request for key-a should be limited")
	}
	if !l.AllowKey("key-b") {
		t.Fatal("key-b should have a separate bucket")
	}
}

func TestLimiterMiddlewareResponses(t *testing.T) {
	t.Run("plain text", func(t *testing.T) {
		limiter := New(1, time.Minute)
		handler := limiter.Middleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))

		first := httptest.NewRecorder()
		handler.ServeHTTP(first, httptest.NewRequest(http.MethodGet, "/", nil))
		if first.Code != http.StatusNoContent {
			t.Fatalf("first status = %d", first.Code)
		}

		second := httptest.NewRecorder()
		handler.ServeHTTP(second, httptest.NewRequest(http.MethodGet, "/", nil))
		if second.Code != http.StatusTooManyRequests {
			t.Fatalf("second status = %d", second.Code)
		}
		if second.Header().Get("Retry-After") != "60" {
			t.Fatalf("Retry-After = %q", second.Header().Get("Retry-After"))
		}
	})

	t.Run("json", func(t *testing.T) {
		limiter := New(1, 30*time.Second)
		handler := limiter.JSONMiddleware(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		}))
		handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
		rr := httptest.NewRecorder()
		handler.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/", nil))
		if rr.Code != http.StatusTooManyRequests {
			t.Fatalf("status = %d", rr.Code)
		}
		if rr.Header().Get("Content-Type") != "application/json" || !strings.Contains(rr.Body.String(), "too many requests") {
			t.Fatalf("unexpected JSON response: headers=%v body=%q", rr.Header(), rr.Body.String())
		}
	})
}

func TestLimiterWindowAndExtractIP(t *testing.T) {
	limiter := New(1, time.Hour)
	if !limiter.AllowKey("host") || limiter.AllowKey("host") {
		t.Fatal("limiter did not enforce the initial window")
	}
	limiter.mu.Lock()
	limiter.buckets["host"].reset = time.Now().Add(-time.Second)
	limiter.mu.Unlock()
	if !limiter.AllowKey("host") {
		t.Fatal("expired window was not reset")
	}

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.RemoteAddr = "192.0.2.10:4321"
	if got := ExtractIP(req); got != "192.0.2.10" {
		t.Fatalf("ExtractIP = %q", got)
	}
	req.RemoteAddr = "local-socket"
	if got := ExtractIP(req); got != "local-socket" {
		t.Fatalf("ExtractIP without port = %q", got)
	}
}

type memoryBanStore struct {
	bans      []IPBan
	listErr   error
	upserted  []IPBan
	deletedIP string
}

func (s *memoryBanStore) ListIPBans(context.Context) ([]IPBan, error) {
	return s.bans, s.listErr
}

func (s *memoryBanStore) UpsertIPBan(_ context.Context, ban IPBan) error {
	s.upserted = append(s.upserted, ban)
	return nil
}

func (s *memoryBanStore) DeleteIPBan(_ context.Context, ip string) error {
	s.deletedIP = ip
	return nil
}

func TestBanhammerEscalatesPersistsAndUnbans(t *testing.T) {
	persistence := &memoryBanStore{}
	banhammer := NewBanhammer(2, time.Minute, persistence, time.Hour, 0)
	ip := "192.0.2.20"

	if banhammer.RecordFailure(ip) {
		t.Fatal("first failure unexpectedly banned the IP")
	}
	if !banhammer.RecordFailure(ip) {
		t.Fatal("second failure did not ban the IP")
	}
	if banned, remaining := banhammer.IsBanned(ip); !banned || remaining <= 0 {
		t.Fatalf("temporary ban = (%t, %v)", banned, remaining)
	}
	if len(persistence.upserted) != 1 || persistence.upserted[0].BanExpiry == nil {
		t.Fatalf("temporary ban was not persisted: %#v", persistence.upserted)
	}
	if bans := banhammer.ListBanned(); len(bans) != 1 || bans[0].IP != ip {
		t.Fatalf("ListBanned = %#v", bans)
	}
	if banhammer.RecordFailure(ip) {
		t.Fatal("failure during an active ban changed the state")
	}

	banhammer.mu.Lock()
	banhammer.states[ip].banExpiry = time.Now().Add(-time.Second)
	banhammer.mu.Unlock()
	if banned, _ := banhammer.IsBanned(ip); banned {
		t.Fatal("expired ban remains active")
	}
	banhammer.RecordFailure(ip)
	if !banhammer.RecordFailure(ip) {
		t.Fatal("repeat offense did not create a permanent ban")
	}
	if banned, remaining := banhammer.IsBanned(ip); !banned || remaining != -1 {
		t.Fatalf("permanent ban = (%t, %v)", banned, remaining)
	}
	if got := persistence.upserted[len(persistence.upserted)-1]; got.BanExpiry != nil || got.BanCount != 2 {
		t.Fatalf("permanent ban persistence = %#v", got)
	}

	banhammer.UnbanIP(ip)
	if banned, _ := banhammer.IsBanned(ip); banned {
		t.Fatal("UnbanIP left the IP banned")
	}
	if persistence.deletedIP != ip {
		t.Fatalf("deleted IP = %q", persistence.deletedIP)
	}
}

func TestBanhammerLoadAndClearFailures(t *testing.T) {
	now := time.Now()
	future := now.Add(time.Hour)
	expired := now.Add(-time.Hour)
	persistence := &memoryBanStore{bans: []IPBan{
		{IP: "temporary", BanCount: 1, BanExpiry: &future, BannedAt: now},
		{IP: "expired", BanCount: 2, BanExpiry: &expired, BannedAt: now},
		{IP: "permanent", BanCount: 3, BanExpiry: nil, BannedAt: now},
	}}
	banhammer := NewBanhammer(2, time.Minute, persistence)
	if err := banhammer.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	if banned, _ := banhammer.IsBanned("temporary"); !banned {
		t.Fatal("temporary persisted ban not loaded")
	}
	if banned, _ := banhammer.IsBanned("expired"); banned {
		t.Fatal("expired persisted ban loaded as active")
	}
	if banned, remaining := banhammer.IsBanned("permanent"); !banned || remaining != -1 {
		t.Fatal("permanent persisted ban not loaded")
	}

	banhammer.RecordFailure("clear")
	banhammer.ClearFailures("clear")
	banhammer.RecordFailure("clear")
	if banned, _ := banhammer.IsBanned("clear"); banned {
		t.Fatal("ClearFailures did not reset the failure count")
	}

	if err := NewBanhammer(2, time.Minute, nil).Load(); err != nil {
		t.Fatalf("memory-only Load: %v", err)
	}
	wantErr := errors.New("database unavailable")
	if err := NewBanhammer(2, time.Minute, &memoryBanStore{listErr: wantErr}).Load(); !errors.Is(err, wantErr) {
		t.Fatalf("Load error = %v", err)
	}
}

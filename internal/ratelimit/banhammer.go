package ratelimit

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// permanentExpiry is an in-memory sentinel that means "banned forever".
var permanentExpiry = time.Date(9999, 1, 1, 0, 0, 0, 0, time.UTC)

// IPBan is the persistent record stored in the DB and returned to callers.
type IPBan struct {
	IP        string
	BanCount  int
	BanExpiry *time.Time // nil = permanent
	BannedAt  time.Time
}

// BanStore persists bans across server restarts. Implemented by *store.Store.
type BanStore interface {
	ListIPBans(ctx context.Context) ([]IPBan, error)
	UpsertIPBan(ctx context.Context, b IPBan) error
	DeleteIPBan(ctx context.Context, ip string) error
}

type ipState struct {
	failures  []time.Time
	banExpiry time.Time // zero=not banned, permanentExpiry=permanent
	banCount  int       // total bans imposed (drives escalation)
	bannedAt  time.Time
}

// Banhammer applies progressive bans per IP.
// banDurs[0] is the first ban duration, banDurs[1] the second, etc.
// A zero duration means permanent.
type Banhammer struct {
	mu       sync.Mutex
	states   map[string]*ipState
	store    BanStore // nil = memory-only (used in tests)
	maxFails int
	window   time.Duration
	banDurs  []time.Duration
}

// NewBanhammer creates the banhammer. Call Load() right after to restore bans from DB.
func NewBanhammer(maxFails int, window time.Duration, store BanStore, banDurs ...time.Duration) *Banhammer {
	if len(banDurs) == 0 {
		banDurs = []time.Duration{24 * time.Hour}
	}
	b := &Banhammer{
		states:   make(map[string]*ipState),
		store:    store,
		maxFails: maxFails,
		window:   window,
		banDurs:  banDurs,
	}
	go b.cleanup()
	return b
}

// Load reads all bans from the DB into memory. Call once at startup.
func (b *Banhammer) Load() error {
	if b.store == nil {
		return nil
	}
	bans, err := b.store.ListIPBans(context.Background())
	if err != nil {
		return err
	}
	now := time.Now()
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, ban := range bans {
		s := &ipState{banCount: ban.BanCount, bannedAt: ban.BannedAt}
		switch {
		case ban.BanExpiry == nil:
			s.banExpiry = permanentExpiry
		case ban.BanExpiry.After(now):
			s.banExpiry = *ban.BanExpiry
		// expired: keep banCount for escalation, banExpiry stays zero
		}
		b.states[ban.IP] = s
	}
	return nil
}

func (b *Banhammer) banDurFor(count int) time.Duration {
	if count >= len(b.banDurs) {
		return b.banDurs[len(b.banDurs)-1]
	}
	return b.banDurs[count]
}

func (b *Banhammer) getOrCreate(ip string) *ipState {
	s, ok := b.states[ip]
	if !ok {
		s = &ipState{}
		b.states[ip] = s
	}
	return s
}

// cleanup runs every hour and evicts states that have no active ban and no recent failures.
func (b *Banhammer) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		cutoff := now.Add(-b.window)
		b.mu.Lock()
		for ip, s := range b.states {
			if !s.banExpiry.IsZero() && (s.banExpiry.Equal(permanentExpiry) || now.Before(s.banExpiry)) {
				continue // actively banned
			}
			hasRecent := false
			for _, t := range s.failures {
				if t.After(cutoff) {
					hasRecent = true
					break
				}
			}
			if !hasRecent {
				delete(b.states, ip)
			}
		}
		b.mu.Unlock()
	}
}

// IsBanned reports whether ip is currently banned.
// remaining == -1 signals a permanent ban.
func (b *Banhammer) IsBanned(ip string) (banned bool, remaining time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	s, ok := b.states[ip]
	if !ok || s.banExpiry.IsZero() {
		return false, 0
	}
	if s.banExpiry.Equal(permanentExpiry) {
		return true, -1
	}
	remaining = time.Until(s.banExpiry)
	if remaining <= 0 {
		s.banExpiry = time.Time{} // expired; keep banCount for next offense
		s.failures = nil
		return false, 0
	}
	return true, remaining
}

// RecordFailure registers a failed login for ip.
// Returns true if the IP just got banned.
func (b *Banhammer) RecordFailure(ip string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	s := b.getOrCreate(ip)

	// Never accumulate failures while already banned.
	if !s.banExpiry.IsZero() && (s.banExpiry.Equal(permanentExpiry) || now.Before(s.banExpiry)) {
		return false
	}

	cutoff := now.Add(-b.window)
	valid := s.failures[:0]
	for _, t := range s.failures {
		if t.After(cutoff) {
			valid = append(valid, t)
		}
	}
	valid = append(valid, now)
	s.failures = valid

	if len(valid) < b.maxFails {
		slog.Info("failed login", "ip", ip, "failures", len(valid), "max", b.maxFails)
		return false
	}

	dur := b.banDurFor(s.banCount)
	s.banCount++
	s.bannedAt = now
	s.failures = nil
	if dur == 0 {
		s.banExpiry = permanentExpiry
	} else {
		s.banExpiry = now.Add(dur)
	}
	slog.Warn("login ip banned", "ip", ip, "offense", s.banCount, "duration", dur.String())

	if b.store != nil {
		var expiry *time.Time
		if dur != 0 {
			e := s.banExpiry
			expiry = &e
		}
		_ = b.store.UpsertIPBan(context.Background(), IPBan{
			IP: ip, BanCount: s.banCount,
			BanExpiry: expiry, BannedAt: now,
		})
	}
	return true
}

// ClearFailures resets the failure counter for ip after a successful login.
// banCount is kept so escalation applies to future offenses.
func (b *Banhammer) ClearFailures(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if s, ok := b.states[ip]; ok {
		s.failures = nil
	}
}

// UnbanIP lifts the ban for ip immediately (admin action).
func (b *Banhammer) UnbanIP(ip string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.states, ip)
	if b.store != nil {
		_ = b.store.DeleteIPBan(context.Background(), ip)
	}
	slog.Info("ip unbanned by admin", "ip", ip)
}

// ListBanned returns all currently active bans (temporary and permanent).
func (b *Banhammer) ListBanned() []IPBan {
	b.mu.Lock()
	defer b.mu.Unlock()
	now := time.Now()
	var out []IPBan
	for ip, s := range b.states {
		if s.banExpiry.IsZero() {
			continue
		}
		if !s.banExpiry.Equal(permanentExpiry) && now.After(s.banExpiry) {
			continue
		}
		ban := IPBan{IP: ip, BanCount: s.banCount, BannedAt: s.bannedAt}
		if !s.banExpiry.Equal(permanentExpiry) {
			e := s.banExpiry
			ban.BanExpiry = &e
		}
		out = append(out, ban)
	}
	return out
}

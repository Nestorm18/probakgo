package ratelimit

import (
	"context"
	"log/slog"
	"net/netip"
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
	trusted  []netip.Prefix // never banned; set once before serving
	proxies  []netip.Prefix // reverse proxies; never exempt even when inside trusted
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

// SetTrustedNetworks exempts trusted networks from bans, for example an office
// or VPN shared by several administrators. Reverse proxies stay bannable even
// inside a trusted network: without forwarded headers every client would look
// like the proxy. Call it before serving requests.
func (b *Banhammer) SetTrustedNetworks(trusted, proxies []netip.Prefix) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.trusted = trusted
	b.proxies = proxies
}

// isTrusted must be called with b.mu held.
func (b *Banhammer) isTrusted(ip string) bool {
	addr, err := netip.ParseAddr(ip)
	if err != nil {
		return false
	}
	addr = addr.Unmap()
	for _, prefix := range b.proxies {
		if prefix.Contains(addr) {
			return false
		}
	}
	for _, prefix := range b.trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
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

// BanKey is the key bans are tracked under. IPv6 clients are grouped by /64,
// the prefix a single host usually controls, so rotating addresses inside it
// cannot evade a ban. IPv4 addresses and unparsable values are used as-is.
func BanKey(ip string) string {
	addr, err := netip.ParseAddr(ip)
	if err != nil || !addr.Is6() || addr.Is4In6() {
		return ip
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return ip
	}
	return prefix.String()
}

func (b *Banhammer) getOrCreate(ip string) *ipState {
	s, ok := b.states[ip]
	if !ok {
		s = &ipState{}
		b.states[ip] = s
	}
	return s
}

// cleanup evicts unused states while preserving prior bans for escalation.
func (b *Banhammer) cleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for range ticker.C {
		b.cleanupAt(time.Now())
	}
}

func (b *Banhammer) cleanupAt(now time.Time) {
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
		if !hasRecent && s.banCount == 0 {
			delete(b.states, ip)
		} else if !hasRecent {
			s.failures = nil
		}
	}
	b.mu.Unlock()
}

// IsBanned reports whether ip is currently banned.
// remaining == -1 signals a permanent ban.
func (b *Banhammer) IsBanned(ip string) (banned bool, remaining time.Duration) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.isTrusted(ip) {
		return false, 0
	}
	ip = BanKey(ip)
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
	if b.isTrusted(ip) {
		slog.Info("failed login from trusted network; not counted for bans", "ip", ip)
		return false
	}
	ip = BanKey(ip)
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
	ip = BanKey(ip)
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
	// Lift both the IPv6 /64 key and a ban recorded for the exact address
	// before bans were grouped by prefix.
	for _, key := range UnbanKeys(ip) {
		delete(b.states, key)
		if b.store != nil {
			_ = b.store.DeleteIPBan(context.Background(), key)
		}
	}
	slog.Info("ip unbanned by admin", "ip", ip)
}

// UnbanKeys lists the ban keys that may hold a ban for ip.
func UnbanKeys(ip string) []string {
	if key := BanKey(ip); key != ip {
		return []string{ip, key}
	}
	return []string{ip}
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

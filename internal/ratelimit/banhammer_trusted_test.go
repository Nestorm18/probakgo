package ratelimit

import (
	"net/netip"
	"testing"
	"time"
)

func TestTrustedNetworksAreNeverBanned(t *testing.T) {
	b := NewBanhammer(1, time.Minute, nil, 0)
	b.SetTrustedNetworks([]netip.Prefix{
		netip.MustParsePrefix("203.0.113.0/24"),
		netip.MustParsePrefix("2001:db8:1::/48"),
	}, []netip.Prefix{netip.MustParsePrefix("203.0.113.20/32")})

	for _, ip := range []string{"203.0.113.7", "::ffff:203.0.113.8", "2001:db8:1:2::9"} {
		if b.RecordFailure(ip) {
			t.Errorf("trusted %s was banned", ip)
		}
		if banned, _ := b.IsBanned(ip); banned {
			t.Errorf("trusted %s is reported as banned", ip)
		}
	}
	if !b.RecordFailure("198.51.100.1") {
		t.Fatal("an untrusted IP was not banned")
	}
	// Without forwarded headers every client looks like the proxy.
	if !b.RecordFailure("203.0.113.20") {
		t.Fatal("the reverse proxy inside a trusted network was exempted from bans")
	}

	// A ban recorded before the network became trusted no longer blocks it.
	b.states["203.0.113.50"] = &ipState{banCount: 3, banExpiry: permanentExpiry}
	if banned, _ := b.IsBanned("203.0.113.50"); banned {
		t.Fatal("an existing ban still blocks a trusted IP")
	}
}

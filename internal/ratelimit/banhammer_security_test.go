package ratelimit

import (
	"testing"
	"time"
)

func TestCleanupPreservesEscalationForExpiredBan(t *testing.T) {
	b := NewBanhammer(1, time.Minute, nil, time.Hour, 24*time.Hour, 0)
	b.states["repeat"] = &ipState{banCount: 1, banExpiry: time.Now().Add(-time.Hour)}
	b.states["unused"] = &ipState{failures: []time.Time{time.Now().Add(-time.Hour)}}
	b.cleanupAt(time.Now())
	if _, ok := b.states["unused"]; ok {
		t.Fatal("unused IP state was retained")
	}
	if !b.RecordFailure("repeat") {
		t.Fatal("repeat offender was not banned")
	}
	bans := b.ListBanned()
	if len(bans) != 1 || bans[0].BanCount != 2 || bans[0].BanExpiry == nil || time.Until(*bans[0].BanExpiry) < 23*time.Hour {
		t.Fatalf("ban escalation reset: %+v", bans)
	}
}

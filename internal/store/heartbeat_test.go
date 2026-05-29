package store

import (
	"context"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestServerHeartbeat_UpsertAndList(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()
	serverID, err := st.UpsertPVEServer(ctx, "pve-heartbeat", "10.0.0.10", "", "0.0.65", "mid-1")
	if err != nil {
		t.Fatalf("UpsertPVEServer: %v", err)
	}
	firstSeen := time.Now().Add(-5 * time.Minute).UTC().Truncate(time.Second)
	if err := st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{
		ServerType:    "pve",
		ServerID:      serverID,
		Hostname:      "pve-heartbeat",
		IP:            "10.0.0.10",
		ClientVersion: "0.0.65",
		MachineID:     "mid-1",
		LastSeenAt:    firstSeen,
	}); err != nil {
		t.Fatalf("UpsertServerHeartbeat first: %v", err)
	}
	secondSeen := time.Now().UTC().Truncate(time.Second)
	if err := st.UpsertServerHeartbeat(ctx, domain.ServerHeartbeat{
		ServerType:    "pve",
		ServerID:      serverID,
		Hostname:      "pve-heartbeat",
		IP:            "10.0.0.11",
		ClientVersion: "0.0.65",
		MachineID:     "mid-1",
		LastSeenAt:    secondSeen,
	}); err != nil {
		t.Fatalf("UpsertServerHeartbeat second: %v", err)
	}

	hb, err := st.GetServerHeartbeat(ctx, "pve", serverID)
	if err != nil {
		t.Fatalf("GetServerHeartbeat: %v", err)
	}
	if hb.IP != "10.0.0.11" {
		t.Fatalf("IP: want updated IP, got %q", hb.IP)
	}
	if !hb.LastSeenAt.Equal(secondSeen) {
		t.Fatalf("LastSeenAt: want %v, got %v", secondSeen, hb.LastSeenAt)
	}

	all, err := st.ListServerHeartbeatsByType(ctx, "pve")
	if err != nil {
		t.Fatalf("ListServerHeartbeatsByType: %v", err)
	}
	if _, ok := all[serverID]; !ok {
		t.Fatalf("heartbeat for server %d not listed", serverID)
	}
}

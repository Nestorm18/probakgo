package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"
)

func TestServerMaintenanceLifecycle(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	until := time.Now().Add(2 * time.Hour).Truncate(time.Second)
	if err := st.UpsertServerMaintenance(ctx, "pve", 42, until, "reparacion"); err != nil {
		t.Fatalf("UpsertServerMaintenance: %v", err)
	}

	got, err := st.GetServerMaintenance(ctx, "pve", 42)
	if err != nil {
		t.Fatalf("GetServerMaintenance: %v", err)
	}
	if got.ServerType != "pve" || got.ServerID != 42 || got.Reason != "reparacion" || !got.Active {
		t.Fatalf("unexpected maintenance: %+v", got)
	}
	if got.Until.Unix() != until.Unix() {
		t.Fatalf("until mismatch: got %v want %v", got.Until, until)
	}

	active, err := st.GetActiveServerMaintenances(ctx)
	if err != nil {
		t.Fatalf("GetActiveServerMaintenances: %v", err)
	}
	if _, ok := active[ServerMaintenanceKey("pve", 42)]; !ok {
		t.Fatalf("active maintenance not found: %+v", active)
	}

	if err := st.DeleteServerMaintenance(ctx, "pve", 42); err != nil {
		t.Fatalf("DeleteServerMaintenance: %v", err)
	}
	if _, err := st.GetServerMaintenance(ctx, "pve", 42); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("expected sql.ErrNoRows after delete, got %v", err)
	}
}

func TestGetActiveServerMaintenancesIgnoresExpired(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	if err := st.UpsertServerMaintenance(ctx, "pbs", 7, time.Now().Add(-time.Hour), "old"); err != nil {
		t.Fatalf("UpsertServerMaintenance: %v", err)
	}
	active, err := st.GetActiveServerMaintenances(ctx)
	if err != nil {
		t.Fatalf("GetActiveServerMaintenances: %v", err)
	}
	if _, ok := active[ServerMaintenanceKey("pbs", 7)]; ok {
		t.Fatalf("expired maintenance should not be active: %+v", active)
	}
}

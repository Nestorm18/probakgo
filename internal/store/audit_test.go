package store

import (
	"context"
	"testing"

	"probakgo/internal/domain"
)

func TestAuditLogInsertAndListNewestFirst(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	if err := st.InsertAuditLog(ctx, domain.AuditLog{
		ActorUsername: "admin",
		ActorRole:     "admin",
		ActorIP:       "10.0.0.1",
		Action:        "user.create",
		TargetType:    "user",
		TargetID:      "1",
		TargetName:    "alice",
		Metadata:      `{"role":"reader"}`,
	}); err != nil {
		t.Fatalf("insert first audit log: %v", err)
	}
	if err := st.InsertAuditLog(ctx, domain.AuditLog{
		ActorUsername: "admin",
		ActorRole:     "admin",
		ActorIP:       "10.0.0.1",
		Action:        "api_key.delete",
		TargetType:    "api_key",
		TargetID:      "2",
		TargetName:    "node-key",
		Metadata:      `{}`,
	}); err != nil {
		t.Fatalf("insert second audit log: %v", err)
	}

	rows, err := st.ListAuditLogs(ctx, 10)
	if err != nil {
		t.Fatalf("list audit logs: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows: got %d, want 2", len(rows))
	}
	if rows[0].Action != "api_key.delete" {
		t.Fatalf("newest action: got %q, want api_key.delete", rows[0].Action)
	}
	if rows[1].Metadata != `{"role":"reader"}` {
		t.Fatalf("metadata: got %q", rows[1].Metadata)
	}
}

func TestAuditLogListPage(t *testing.T) {
	st := openTestDB(t)
	ctx := context.Background()

	for _, action := range []string{"first", "second", "third"} {
		if err := st.InsertAuditLog(ctx, domain.AuditLog{
			ActorUsername: "admin",
			ActorRole:     "admin",
			Action:        action,
			TargetType:    "system",
			Metadata:      `{}`,
		}); err != nil {
			t.Fatalf("insert audit log %q: %v", action, err)
		}
	}

	page1, err := st.ListAuditLogsPage(ctx, 2, 0)
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("first page rows: got %d, want 2", len(page1))
	}
	if page1[0].Action != "third" || page1[1].Action != "second" {
		t.Fatalf("first page actions: got %q, %q", page1[0].Action, page1[1].Action)
	}

	page2, err := st.ListAuditLogsPage(ctx, 2, 2)
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(page2) != 1 {
		t.Fatalf("second page rows: got %d, want 1", len(page2))
	}
	if page2[0].Action != "first" {
		t.Fatalf("second page action: got %q, want first", page2[0].Action)
	}
}

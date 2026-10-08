package store

import (
	"context"
	"database/sql"
	"errors"
	"testing"
)

func TestGetAPIKeyByValueRejectsStoredCiphertext(t *testing.T) {
	ctx := context.Background()
	plain := openTestDB(t)
	st, err := NewEncrypted(plain.db, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}
	k, err := st.CreateAPIKey(ctx, "node", "pve-node", "")
	if err != nil {
		t.Fatalf("create API key: %v", err)
	}
	var stored string
	if err := st.db.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id=?`, k.ID).Scan(&stored); err != nil {
		t.Fatalf("read stored key: %v", err)
	}

	for name, candidate := range map[string]*Store{"encrypted store": st, "store without key": plain} {
		if _, err := candidate.GetAPIKeyByValue(ctx, stored); !errors.Is(err, sql.ErrNoRows) {
			t.Fatalf("%s: lookup with stored ciphertext: got %v, want sql.ErrNoRows", name, err)
		}
	}
	got, err := st.GetAPIKeyByValue(ctx, k.Key)
	if err != nil || got.ID != k.ID {
		t.Fatalf("lookup with real key after ciphertext attempt: got %+v, %v", got, err)
	}
}

func TestBindAPIKeyMachineIDAndServerNameOnlyOnce(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	k, err := st.CreateAPIKey(ctx, "node", "", "")
	if err != nil {
		t.Fatalf("create API key: %v", err)
	}

	if bound, err := st.BindAPIKeyMachineID(ctx, k.ID, "machine-a"); err != nil || !bound {
		t.Fatalf("first machine bind: bound=%v err=%v", bound, err)
	}
	if bound, err := st.BindAPIKeyMachineID(ctx, k.ID, "machine-b"); err != nil || bound {
		t.Fatalf("second machine bind: bound=%v err=%v", bound, err)
	}
	if bound, err := st.BindAPIKeyServerName(ctx, k.ID, "pve-a"); err != nil || !bound {
		t.Fatalf("first server bind: bound=%v err=%v", bound, err)
	}
	if bound, err := st.BindAPIKeyServerName(ctx, k.ID, "pve-b"); err != nil || bound {
		t.Fatalf("second server bind: bound=%v err=%v", bound, err)
	}

	got, err := st.GetAPIKey(ctx, k.ID)
	if err != nil {
		t.Fatalf("get API key: %v", err)
	}
	if got.MachineID != "machine-a" || got.ServerName != "pve-a" {
		t.Fatalf("binding changed: machine=%q server=%q", got.MachineID, got.ServerName)
	}

	if err := st.UnbindAPIKeyServer(ctx, k.ID); err != nil {
		t.Fatalf("unbind: %v", err)
	}
	if bound, err := st.BindAPIKeyMachineID(ctx, k.ID, "machine-b"); err != nil || !bound {
		t.Fatalf("bind after unbind: bound=%v err=%v", bound, err)
	}
}

func TestListAPIKeysPageSearchAndPaging(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	if _, err := st.CreateAPIKey(ctx, "alpha", "pve-alpha", "https://10.0.0.1:8006"); err != nil {
		t.Fatalf("create alpha key: %v", err)
	}
	if _, err := st.CreateAPIKey(ctx, "beta", "pbs-beta", "https://10.0.0.2:8007"); err != nil {
		t.Fatalf("create beta key: %v", err)
	}
	if _, err := st.CreateAPIKey(ctx, "gamma", "win-gamma", ""); err != nil {
		t.Fatalf("create gamma key: %v", err)
	}

	page1, err := st.ListAPIKeysPage(ctx, 2, 0, "")
	if err != nil {
		t.Fatalf("list first page: %v", err)
	}
	if len(page1) != 2 {
		t.Fatalf("first page rows: got %d, want 2", len(page1))
	}
	if page1[0].Name != "gamma" || page1[1].Name != "beta" {
		t.Fatalf("first page names: got %q, %q", page1[0].Name, page1[1].Name)
	}

	page2, err := st.ListAPIKeysPage(ctx, 2, 2, "")
	if err != nil {
		t.Fatalf("list second page: %v", err)
	}
	if len(page2) != 1 || page2[0].Name != "alpha" {
		t.Fatalf("second page: got %+v", page2)
	}

	found, err := st.ListAPIKeysPage(ctx, 10, 0, "10.0.0.2")
	if err != nil {
		t.Fatalf("search keys: %v", err)
	}
	if len(found) != 1 || found[0].Name != "beta" {
		t.Fatalf("search result: got %+v", found)
	}
}

func TestListAPIKeysPageSearchIgnoresCiphertextAndWildcards(t *testing.T) {
	ctx := context.Background()
	plain := openTestDB(t)
	st, err := NewEncrypted(plain.db, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}
	k, err := st.CreateAPIKey(ctx, "alpha", "pve-alpha", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if _, err := st.CreateAPIKey(ctx, "beta_1", "pbs-beta", ""); err != nil {
		t.Fatalf("create key: %v", err)
	}

	// "enc" is in every stored ciphertext but in no visible field.
	if found, err := st.ListAPIKeysPage(ctx, 10, 0, "enc"); err != nil || len(found) != 0 {
		t.Fatalf("search matched ciphertext: %v %+v", err, found)
	}
	if found, err := st.ListAPIKeysPage(ctx, 10, 0, "_"); err != nil || len(found) != 1 || found[0].Name != "beta_1" {
		t.Fatalf("underscore must match literally: %v %+v", err, found)
	}
	if found, err := st.ListAPIKeysPage(ctx, 10, 0, k.Key); err != nil || len(found) != 1 || found[0].ID != k.ID {
		t.Fatalf("exact key search: %v %+v", err, found)
	}
}

func TestUpdateAPIKeyLastUsedSkipsRecentWrites(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	k, err := st.CreateAPIKey(ctx, "node", "pve-node", "")
	if err != nil {
		t.Fatalf("create key: %v", err)
	}
	if err := st.UpdateAPIKeyLastUsed(ctx, k.ID); err != nil {
		t.Fatalf("first update: %v", err)
	}
	first, _ := st.GetAPIKey(ctx, k.ID)
	if first.LastUsed == nil {
		t.Fatal("last_used was not set")
	}
	if err := st.UpdateAPIKeyLastUsed(ctx, k.ID); err != nil {
		t.Fatalf("second update: %v", err)
	}
	second, _ := st.GetAPIKey(ctx, k.ID)
	if !second.LastUsed.Equal(*first.LastUsed) {
		t.Fatalf("last_used rewritten within a minute: %v -> %v", first.LastUsed, second.LastUsed)
	}
}

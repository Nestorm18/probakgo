package store

import (
	"context"
	"probakgo/internal/domain"
	"probakgo/internal/secretbox"
	"testing"
)

func TestNASBackupEncryptedConfigAndClaims(t *testing.T) {
	plain := openTestDB(t)
	st, err := NewEncrypted(plain.db, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	c, err := st.GetNASBackupConfig(ctx)
	if err != nil || c.Enabled || c.Port != 22 || c.SendTime != "03:00" {
		t.Fatalf("defaults: %+v, %v", c, err)
	}
	c.Enabled, c.Password = true, "nas-password"
	if err := plain.SaveNASBackupConfig(ctx, *c); err == nil {
		t.Fatal("accepted plaintext credentials")
	}
	if err := st.SaveNASBackupConfig(ctx, *c); err != nil {
		t.Fatal(err)
	}
	var raw string
	if err := plain.db.QueryRow(`SELECT password FROM nas_backup_config`).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if !secretbox.IsEncrypted(raw) {
		t.Fatal("password not encrypted")
	}
	got, err := st.GetNASBackupConfig(ctx)
	if err != nil || got.Password != c.Password {
		t.Fatal("encrypted round trip failed", err)
	}
	if protected, err := st.ValidateProtectedSecrets(ctx); err != nil || !protected {
		t.Fatal("secret validation failed", err)
	}
	if _, err := plain.ValidateProtectedSecrets(ctx); err == nil {
		t.Fatal("missing key accepted")
	}
	attempt := "2026-09-09T03:00:00Z"
	if ok, err := st.ClaimNASBackup(ctx, "", attempt); err != nil || !ok {
		t.Fatal("claim failed", err)
	}
	if ok, err := st.ClaimNASBackup(ctx, "", attempt); err != nil || ok {
		t.Fatal("duplicate claimed", err)
	}
	if err := st.FinishNASBackup(ctx, attempt, ""); err != nil {
		t.Fatal(err)
	}
	if err := st.SaveNASBackupConfig(ctx, *c); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetNASBackupConfig(ctx)
	if err != nil || got.LastSuccess != attempt || got.LastAttempt != attempt || got.LastScheduledAttempt != attempt {
		t.Fatal("saving erased run state", err)
	}
	if err := st.FinishNASBackup(ctx, attempt, "failed"); err != nil {
		t.Fatal(err)
	}
	got, _ = st.GetNASBackupConfig(ctx)
	if got.LastSuccess != attempt || got.LastError != "failed" {
		t.Fatal("failure erased successful backup")
	}
	if ok, err := st.ClaimManualNASBackup(ctx, attempt, "2026-09-09T10:00:00Z"); err != nil || !ok {
		t.Fatal("manual claim failed", err)
	}
	got, err = st.GetNASBackupConfig(ctx)
	if err != nil || got.LastScheduledAttempt != attempt {
		t.Fatal("manual backup changed schedule", err)
	}
	if err := st.ResetAllData(ctx); err != nil {
		t.Fatal(err)
	}
	got, err = st.GetNASBackupConfig(ctx)
	if err != nil || got.Enabled || got.Password != "" || got.LastAttempt != "" || got.LastScheduledAttempt != "" || got.Port != 22 {
		t.Fatalf("NAS config not reset: %+v, %v", got, err)
	}
	if err := st.SaveNASBackupConfig(ctx, domain.NASBackupConfig{}); err != nil {
		t.Fatal(err)
	}
	if ok, err := st.ClaimNASBackup(ctx, attempt, "next"); err != nil || ok {
		t.Fatal("disabled backup claimed", err)
	}
}

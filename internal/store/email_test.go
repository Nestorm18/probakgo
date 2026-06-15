package store

import (
	"context"
	"testing"

	"probakgo/internal/domain"
)

func TestGetEmailConfig_Defaults(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if cfg.SendTime != "08:00" {
		t.Errorf("SendTime: want 08:00, got %s", cfg.SendTime)
	}
	if cfg.RetentionMonths != 3 {
		t.Errorf("RetentionMonths: want 3, got %d", cfg.RetentionMonths)
	}
	if !cfg.RetentionEnabled {
		t.Error("RetentionEnabled: want true")
	}
	if cfg.AlertDiskPct != 85 {
		t.Errorf("AlertDiskPct: want 85, got %d", cfg.AlertDiskPct)
	}
	if !cfg.AlertBackupErr {
		t.Error("AlertBackupErr: want true")
	}
	if cfg.AlertPVEHeartbeatMinutes != 15 {
		t.Errorf("AlertPVEHeartbeatMinutes: want 15, got %d", cfg.AlertPVEHeartbeatMinutes)
	}
	if cfg.CriticalAlertsEnabled {
		t.Error("CriticalAlertsEnabled: want false")
	}
}

func TestUpsertEmailConfig_RoundTrip(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	want := domain.EmailConfig{
		SMTPHost:                 "smtp.example.com",
		SMTPPort:                 465,
		SMTPUser:                 "user@example.com",
		SMTPPass:                 "s3cr3t",
		Recipients:               "admin@example.com,ops@example.com",
		IsEnabled:                true,
		SendTime:                 "09:30",
		RetentionMonths:          12,
		RetentionEnabled:         false,
		AlertDiskPct:             90,
		AlertBackupErr:           false,
		PublicAPIURL:             "https://probakgo.example.com",
		AlertPVEHeartbeatMinutes: 10,
		CriticalAlertsEnabled:    true,
	}

	if err := st.UpsertEmailConfig(ctx, want); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	got, err := st.GetEmailConfig(ctx)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"SMTPHost", got.SMTPHost, want.SMTPHost},
		{"SMTPPort", got.SMTPPort, want.SMTPPort},
		{"SMTPUser", got.SMTPUser, want.SMTPUser},
		{"SMTPPass", got.SMTPPass, want.SMTPPass},
		{"Recipients", got.Recipients, want.Recipients},
		{"IsEnabled", got.IsEnabled, want.IsEnabled},
		{"SendTime", got.SendTime, want.SendTime},
		{"RetentionMonths", got.RetentionMonths, want.RetentionMonths},
		{"RetentionEnabled", got.RetentionEnabled, want.RetentionEnabled},
		{"AlertDiskPct", got.AlertDiskPct, want.AlertDiskPct},
		{"AlertBackupErr", got.AlertBackupErr, want.AlertBackupErr},
		{"PublicAPIURL", got.PublicAPIURL, want.PublicAPIURL},
		{"AlertPVEHeartbeatMinutes", got.AlertPVEHeartbeatMinutes, want.AlertPVEHeartbeatMinutes},
		{"CriticalAlertsEnabled", got.CriticalAlertsEnabled, want.CriticalAlertsEnabled},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: want %v, got %v", c.name, c.want, c.got)
		}
	}
}

func TestUpsertEmailConfig_UpdateInPlace(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)

	first := domain.EmailConfig{SMTPHost: "smtp1.example.com", SMTPPort: 587, SendTime: "08:00", RetentionMonths: 3}
	second := domain.EmailConfig{SMTPHost: "smtp2.example.com", SMTPPort: 587, SendTime: "10:00", RetentionMonths: 6}

	if err := st.UpsertEmailConfig(ctx, first); err != nil {
		t.Fatalf("first upsert: %v", err)
	}
	if err := st.UpsertEmailConfig(ctx, second); err != nil {
		t.Fatalf("second upsert: %v", err)
	}

	var count int
	if err := st.db.QueryRow("SELECT COUNT(*) FROM email_config").Scan(&count); err != nil {
		t.Fatalf("count rows: %v", err)
	}
	if count != 1 {
		t.Errorf("want 1 row in email_config, got %d", count)
	}

	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		t.Fatalf("get config: %v", err)
	}
	if cfg.SMTPHost != "smtp2.example.com" {
		t.Errorf("SMTPHost: want smtp2.example.com, got %s", cfg.SMTPHost)
	}
	if cfg.RetentionMonths != 6 {
		t.Errorf("RetentionMonths: want 6, got %d", cfg.RetentionMonths)
	}
}

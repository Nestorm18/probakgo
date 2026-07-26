package store

import (
	"context"
	"strings"
	"testing"
)

func TestProtectLegacySecretsAndEncryptedRoundTrip(t *testing.T) {
	plain := openTestDB(t)
	ctx := context.Background()

	if _, err := plain.db.ExecContext(ctx, `
		INSERT INTO api_keys (key, name, key_type, server_name)
		VALUES ('pbk-legacy', 'legacy', 'server', 'pve-legacy')`); err != nil {
		t.Fatalf("insert API key: %v", err)
	}
	if _, err := plain.db.ExecContext(ctx, `
		INSERT INTO email_config (id, smtp_host, smtp_user, smtp_password, recipients)
		VALUES (1, '', '', 'smtp-legacy', '')`); err != nil {
		t.Fatalf("insert email config: %v", err)
	}
	if _, err := plain.db.ExecContext(ctx, `
		INSERT INTO users (username, password_hash, role, totp_enabled, totp_secret)
		VALUES ('admin', 'hash', 'admin', 1, 'totp-legacy')`); err != nil {
		t.Fatalf("insert user: %v", err)
	}

	encrypted, err := NewEncrypted(plain.db, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}
	if err := encrypted.ProtectLegacySecrets(ctx); err != nil {
		t.Fatalf("ProtectLegacySecrets: %v", err)
	}

	var storedKey, keyHash, storedSMTP, storedTOTP string
	if err := plain.db.QueryRowContext(ctx, `SELECT key, key_hash FROM api_keys`).Scan(&storedKey, &keyHash); err != nil {
		t.Fatalf("read stored API key: %v", err)
	}
	if err := plain.db.QueryRowContext(ctx, `SELECT smtp_password FROM email_config`).Scan(&storedSMTP); err != nil {
		t.Fatalf("read stored SMTP password: %v", err)
	}
	if err := plain.db.QueryRowContext(ctx, `SELECT totp_secret FROM users`).Scan(&storedTOTP); err != nil {
		t.Fatalf("read stored TOTP secret: %v", err)
	}
	for name, value := range map[string]string{
		"API key":       storedKey,
		"SMTP password": storedSMTP,
		"TOTP secret":   storedTOTP,
	} {
		if !strings.HasPrefix(value, "enc:v1:") {
			t.Errorf("%s remains plaintext: %q", name, value)
		}
	}
	if keyHash == "" {
		t.Fatal("API key lookup hash is empty")
	}

	apiKey, err := encrypted.GetAPIKeyByValue(ctx, "pbk-legacy")
	if err != nil {
		t.Fatalf("GetAPIKeyByValue: %v", err)
	}
	if apiKey.Key != "pbk-legacy" {
		t.Fatalf("API key: got %q", apiKey.Key)
	}
	emailConfig, err := encrypted.GetEmailConfig(ctx)
	if err != nil {
		t.Fatalf("GetEmailConfig: %v", err)
	}
	if emailConfig.SMTPPass != "smtp-legacy" {
		t.Fatalf("SMTP password: got %q", emailConfig.SMTPPass)
	}
	user, err := encrypted.GetUserByUsername(ctx, "admin")
	if err != nil {
		t.Fatalf("GetUserByUsername: %v", err)
	}
	if user.TOTPSecret != "totp-legacy" {
		t.Fatalf("TOTP secret: got %q", user.TOTPSecret)
	}

	created, err := encrypted.CreateAPIKey(ctx, "new", "pve-new", "")
	if err != nil {
		t.Fatalf("CreateAPIKey: %v", err)
	}
	var storedCreated string
	if err := plain.db.QueryRowContext(ctx, `SELECT key FROM api_keys WHERE id=?`, created.ID).Scan(&storedCreated); err != nil {
		t.Fatalf("read created API key: %v", err)
	}
	if storedCreated == created.Key || !strings.HasPrefix(storedCreated, "enc:v1:") {
		t.Fatalf("new API key not encrypted: stored=%q plain=%q", storedCreated, created.Key)
	}

	protected, err := encrypted.ValidateProtectedSecrets(ctx)
	if err != nil {
		t.Fatalf("ValidateProtectedSecrets: %v", err)
	}
	if !protected {
		t.Fatal("ValidateProtectedSecrets did not detect encrypted values")
	}
	wrongKey, err := NewEncrypted(plain.db, "abcdef0123456789abcdef0123456789")
	if err != nil {
		t.Fatalf("NewEncrypted with wrong key: %v", err)
	}
	if _, err := wrongKey.ValidateProtectedSecrets(ctx); err == nil {
		t.Fatal("ValidateProtectedSecrets accepted the wrong key")
	}
}

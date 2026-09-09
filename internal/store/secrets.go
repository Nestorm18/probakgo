package store

import (
	"context"
	"fmt"

	"probakgo/internal/secretbox"
)

// ValidateProtectedSecrets verifies every encrypted value and API-key lookup
// hash without changing the database. The returned bool reports whether any
// protected value was found.
func (s *Store) ValidateProtectedSecrets(ctx context.Context) (bool, error) {
	protected := false

	rows, err := s.db.QueryContext(ctx, `SELECT id, key, key_hash FROM api_keys`)
	if err != nil {
		return false, err
	}
	for rows.Next() {
		var id int64
		var value, keyHash string
		if err := rows.Scan(&id, &value, &keyHash); err != nil {
			rows.Close()
			return false, err
		}
		if !secretbox.IsEncrypted(value) {
			continue
		}
		protected = true
		if s.secrets == nil {
			rows.Close()
			return true, fmt.Errorf("API key %d is encrypted but DATA_ENCRYPTION_KEY is unavailable", id)
		}
		plain, err := s.secrets.Decrypt(value)
		if err != nil {
			rows.Close()
			return true, fmt.Errorf("decrypt API key %d: %w", id, err)
		}
		if keyHash == "" || keyHash != s.secrets.LookupHash(plain) {
			rows.Close()
			return true, fmt.Errorf("API key %d has an invalid lookup hash", id)
		}
	}
	if err := rows.Close(); err != nil {
		return protected, err
	}
	if err := rows.Err(); err != nil {
		return protected, err
	}

	for _, source := range []struct {
		name  string
		query string
	}{
		{"SMTP password", `SELECT id, smtp_password FROM email_config WHERE smtp_password <> ''`},
		{"NAS password", `SELECT id, password FROM nas_backup_config WHERE password <> ''`},
		{"Telegram bot token", `SELECT id, bot_token FROM telegram_config WHERE bot_token <> ''`},
		{"TOTP secret", `SELECT id, totp_secret FROM users WHERE totp_secret <> ''`},
	} {
		rows, err = s.db.QueryContext(ctx, source.query)
		if err != nil {
			return protected, err
		}
		for rows.Next() {
			var id int64
			var value string
			if err := rows.Scan(&id, &value); err != nil {
				rows.Close()
				return protected, err
			}
			if !secretbox.IsEncrypted(value) {
				continue
			}
			protected = true
			if s.secrets == nil {
				rows.Close()
				return true, fmt.Errorf("%s %d is encrypted but DATA_ENCRYPTION_KEY is unavailable", source.name, id)
			}
			if _, err := s.secrets.Decrypt(value); err != nil {
				rows.Close()
				return true, fmt.Errorf("decrypt %s %d: %w", source.name, id, err)
			}
		}
		if err := rows.Close(); err != nil {
			return protected, err
		}
		if err := rows.Err(); err != nil {
			return protected, err
		}
	}

	return protected, nil
}

// ProtectLegacySecrets migrates plaintext API keys, SMTP passwords, Telegram
// bot tokens and TOTP secrets in one transaction. It is safe to call on every startup.
func (s *Store) ProtectLegacySecrets(ctx context.Context) error {
	if s.secrets == nil {
		return nil
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin secret migration: %w", err)
	}
	defer tx.Rollback()

	type secretRow struct {
		id    int64
		value string
	}

	var apiKeys []secretRow
	rows, err := tx.QueryContext(ctx, `SELECT id, key FROM api_keys`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row secretRow
		if err := rows.Scan(&row.id, &row.value); err != nil {
			rows.Close()
			return err
		}
		apiKeys = append(apiKeys, row)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range apiKeys {
		plain, err := s.secrets.Decrypt(row.value)
		if err != nil {
			return fmt.Errorf("decrypt API key %d: %w", row.id, err)
		}
		encrypted := row.value
		if !secretbox.IsEncrypted(row.value) {
			encrypted, err = s.secrets.Encrypt(plain)
			if err != nil {
				return err
			}
		}
		if _, err := tx.ExecContext(ctx, `UPDATE api_keys SET key=?, key_hash=? WHERE id=?`,
			encrypted, s.secrets.LookupHash(plain), row.id); err != nil {
			return fmt.Errorf("protect API key %d: %w", row.id, err)
		}
	}

	var emailSecrets []secretRow
	rows, err = tx.QueryContext(ctx, `SELECT id, smtp_password FROM email_config WHERE smtp_password <> ''`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row secretRow
		if err := rows.Scan(&row.id, &row.value); err != nil {
			rows.Close()
			return err
		}
		emailSecrets = append(emailSecrets, row)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range emailSecrets {
		if secretbox.IsEncrypted(row.value) {
			if _, err := s.secrets.Decrypt(row.value); err != nil {
				return fmt.Errorf("decrypt SMTP password %d: %w", row.id, err)
			}
			continue
		}
		encrypted, err := s.secrets.Encrypt(row.value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE email_config SET smtp_password=? WHERE id=?`, encrypted, row.id); err != nil {
			return fmt.Errorf("protect SMTP password %d: %w", row.id, err)
		}
	}

	var telegramSecrets []secretRow
	rows, err = tx.QueryContext(ctx, `SELECT id, bot_token FROM telegram_config WHERE bot_token <> ''`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row secretRow
		if err := rows.Scan(&row.id, &row.value); err != nil {
			rows.Close()
			return err
		}
		telegramSecrets = append(telegramSecrets, row)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range telegramSecrets {
		if secretbox.IsEncrypted(row.value) {
			if _, err := s.secrets.Decrypt(row.value); err != nil {
				return fmt.Errorf("decrypt Telegram bot token %d: %w", row.id, err)
			}
			continue
		}
		encrypted, err := s.secrets.Encrypt(row.value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE telegram_config SET bot_token=? WHERE id=?`, encrypted, row.id); err != nil {
			return fmt.Errorf("protect Telegram bot token %d: %w", row.id, err)
		}
	}

	var totpSecrets []secretRow
	rows, err = tx.QueryContext(ctx, `SELECT id, totp_secret FROM users WHERE totp_secret <> ''`)
	if err != nil {
		return err
	}
	for rows.Next() {
		var row secretRow
		if err := rows.Scan(&row.id, &row.value); err != nil {
			rows.Close()
			return err
		}
		totpSecrets = append(totpSecrets, row)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for _, row := range totpSecrets {
		if secretbox.IsEncrypted(row.value) {
			if _, err := s.secrets.Decrypt(row.value); err != nil {
				return fmt.Errorf("decrypt TOTP secret %d: %w", row.id, err)
			}
			continue
		}
		encrypted, err := s.secrets.Encrypt(row.value)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET totp_secret=? WHERE id=?`, encrypted, row.id); err != nil {
			return fmt.Errorf("protect TOTP secret %d: %w", row.id, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit secret migration: %w", err)
	}
	return nil
}

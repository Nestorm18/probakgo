package store

import (
	"context"
	"database/sql"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) GetPVEAlertConfig(ctx context.Context, serverID int64) (domain.PVEAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT disk_pct, stale_hours, backup_err, expected_finish_time FROM pve_alert_config WHERE server_id = ?`)
	row := s.db.QueryRowContext(ctx,
		`SELECT disk_pct, stale_hours, backup_err, expected_finish_time FROM pve_alert_config WHERE server_id = ?`,
		serverID,
	)
	var cfg domain.PVEAlertConfig
	cfg.ServerID = serverID
	var diskPct, staleHours, backupErr sql.NullInt64
	var expectedFinish sql.NullString
	if err := row.Scan(&diskPct, &staleHours, &backupErr, &expectedFinish); err == sql.ErrNoRows {
		return cfg, nil
	} else if err != nil {
		return cfg, err
	}
	if diskPct.Valid {
		v := int(diskPct.Int64)
		cfg.DiskPct = &v
	}
	if staleHours.Valid {
		v := int(staleHours.Int64)
		cfg.StaleHours = &v
	}
	if backupErr.Valid {
		v := int(backupErr.Int64)
		cfg.BackupErr = &v
	}
	if expectedFinish.Valid {
		v := expectedFinish.String
		cfg.ExpectedFinishTime = &v
	}
	return cfg, nil
}

func (s *Store) ListPVEAlertConfigs(ctx context.Context) (map[int64]domain.PVEAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT server_id, disk_pct, stale_hours, backup_err, expected_finish_time FROM pve_alert_config`)
	rows, err := s.db.QueryContext(ctx,
		`SELECT server_id, disk_pct, stale_hours, backup_err, expected_finish_time FROM pve_alert_config`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configs := make(map[int64]domain.PVEAlertConfig)
	for rows.Next() {
		var cfg domain.PVEAlertConfig
		var diskPct, staleHours, backupErr sql.NullInt64
		var expectedFinish sql.NullString
		if err := rows.Scan(&cfg.ServerID, &diskPct, &staleHours, &backupErr, &expectedFinish); err != nil {
			return nil, err
		}
		if diskPct.Valid {
			v := int(diskPct.Int64)
			cfg.DiskPct = &v
		}
		if staleHours.Valid {
			v := int(staleHours.Int64)
			cfg.StaleHours = &v
		}
		if backupErr.Valid {
			v := int(backupErr.Int64)
			cfg.BackupErr = &v
		}
		if expectedFinish.Valid {
			v := expectedFinish.String
			cfg.ExpectedFinishTime = &v
		}
		configs[cfg.ServerID] = cfg
	}
	return configs, rows.Err()
}

func (s *Store) UpsertPVEAlertConfig(ctx context.Context, cfg domain.PVEAlertConfig) error {
	debug.RecordQuery(ctx, `INSERT INTO pve_alert_config (server_id, disk_pct, stale_hours, backup_err, expected_finish_time, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(server_id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pve_alert_config (server_id, disk_pct, stale_hours, backup_err, expected_finish_time, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(server_id) DO UPDATE SET
			disk_pct=excluded.disk_pct,
			stale_hours=excluded.stale_hours,
			backup_err=excluded.backup_err,
			expected_finish_time=excluded.expected_finish_time,
			updated_at=excluded.updated_at`,
		cfg.ServerID, nullInt(cfg.DiskPct), nullInt(cfg.StaleHours), nullInt(cfg.BackupErr), nullString(cfg.ExpectedFinishTime),
	)
	return err
}

func (s *Store) GetPVEVMAlertConfigs(ctx context.Context, serverID int64) ([]domain.PVEVMAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, vmid, backup_err, min_size_mb FROM pve_vm_alert_config WHERE server_id = ?`)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, server_id, vmid, backup_err, min_size_mb FROM pve_vm_alert_config WHERE server_id = ?`,
		serverID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var configs []domain.PVEVMAlertConfig
	for rows.Next() {
		var c domain.PVEVMAlertConfig
		var backupErr, minSizeMB sql.NullInt64
		if err := rows.Scan(&c.ID, &c.ServerID, &c.VMID, &backupErr, &minSizeMB); err != nil {
			return nil, err
		}
		if backupErr.Valid {
			v := int(backupErr.Int64)
			c.BackupErr = &v
		}
		if minSizeMB.Valid {
			v := int(minSizeMB.Int64)
			c.MinSizeMB = &v
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func (s *Store) ListPVEVMAlertConfigs(ctx context.Context) (map[int64][]domain.PVEVMAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, vmid, backup_err, min_size_mb FROM pve_vm_alert_config ORDER BY server_id, vmid`)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, server_id, vmid, backup_err, min_size_mb FROM pve_vm_alert_config ORDER BY server_id, vmid`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configs := make(map[int64][]domain.PVEVMAlertConfig)
	for rows.Next() {
		var c domain.PVEVMAlertConfig
		var backupErr, minSizeMB sql.NullInt64
		if err := rows.Scan(&c.ID, &c.ServerID, &c.VMID, &backupErr, &minSizeMB); err != nil {
			return nil, err
		}
		if backupErr.Valid {
			v := int(backupErr.Int64)
			c.BackupErr = &v
		}
		if minSizeMB.Valid {
			v := int(minSizeMB.Int64)
			c.MinSizeMB = &v
		}
		configs[c.ServerID] = append(configs[c.ServerID], c)
	}
	return configs, rows.Err()
}

func (s *Store) UpsertPVEVMAlertConfig(ctx context.Context, cfg domain.PVEVMAlertConfig) error {
	debug.RecordQuery(ctx, `INSERT INTO pve_vm_alert_config (server_id, vmid, backup_err, min_size_mb) VALUES (?, ?, ?, ?) ON CONFLICT(server_id, vmid) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pve_vm_alert_config (server_id, vmid, backup_err, min_size_mb)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(server_id, vmid) DO UPDATE SET
			backup_err=excluded.backup_err,
			min_size_mb=excluded.min_size_mb`,
		cfg.ServerID, cfg.VMID, nullInt(cfg.BackupErr), nullInt(cfg.MinSizeMB),
	)
	return err
}

func (s *Store) DeletePVEVMAlertConfig(ctx context.Context, serverID, vmid int64) error {
	debug.RecordQuery(ctx, `DELETE FROM pve_vm_alert_config WHERE server_id = ? AND vmid = ?`)
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM pve_vm_alert_config WHERE server_id = ? AND vmid = ?`,
		serverID, vmid,
	)
	return err
}

func (s *Store) GetPBSAlertConfig(ctx context.Context, serverID int64) (domain.PBSAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT disk_pct, days_until_full, stale_hours, verify_alert FROM pbs_alert_config WHERE server_id = ?`)
	row := s.db.QueryRowContext(ctx,
		`SELECT disk_pct, days_until_full, stale_hours, verify_alert FROM pbs_alert_config WHERE server_id = ?`,
		serverID,
	)
	var cfg domain.PBSAlertConfig
	cfg.ServerID = serverID
	cfg.VerifyAlert = true // default on
	var diskPct, daysUntilFull, staleHours sql.NullInt64
	var verifyAlert int
	if err := row.Scan(&diskPct, &daysUntilFull, &staleHours, &verifyAlert); err == sql.ErrNoRows {
		return cfg, nil
	} else if err != nil {
		return cfg, err
	}
	if diskPct.Valid {
		v := int(diskPct.Int64)
		cfg.DiskPct = &v
	}
	if daysUntilFull.Valid {
		v := int(daysUntilFull.Int64)
		cfg.DaysUntilFull = &v
	}
	if staleHours.Valid {
		v := int(staleHours.Int64)
		cfg.StaleHours = &v
	}
	cfg.VerifyAlert = verifyAlert != 0
	return cfg, nil
}

func (s *Store) ListPBSAlertConfigs(ctx context.Context) (map[int64]domain.PBSAlertConfig, error) {
	debug.RecordQuery(ctx, `SELECT server_id, disk_pct, days_until_full, stale_hours, verify_alert FROM pbs_alert_config`)
	rows, err := s.db.QueryContext(ctx,
		`SELECT server_id, disk_pct, days_until_full, stale_hours, verify_alert FROM pbs_alert_config`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	configs := make(map[int64]domain.PBSAlertConfig)
	for rows.Next() {
		var cfg domain.PBSAlertConfig
		cfg.VerifyAlert = true
		var diskPct, daysUntilFull, staleHours sql.NullInt64
		var verifyAlert int
		if err := rows.Scan(&cfg.ServerID, &diskPct, &daysUntilFull, &staleHours, &verifyAlert); err != nil {
			return nil, err
		}
		if diskPct.Valid {
			v := int(diskPct.Int64)
			cfg.DiskPct = &v
		}
		if daysUntilFull.Valid {
			v := int(daysUntilFull.Int64)
			cfg.DaysUntilFull = &v
		}
		if staleHours.Valid {
			v := int(staleHours.Int64)
			cfg.StaleHours = &v
		}
		cfg.VerifyAlert = verifyAlert != 0
		configs[cfg.ServerID] = cfg
	}
	return configs, rows.Err()
}

func (s *Store) UpsertPBSAlertConfig(ctx context.Context, cfg domain.PBSAlertConfig) error {
	verifyInt := 0
	if cfg.VerifyAlert {
		verifyInt = 1
	}
	debug.RecordQuery(ctx, `INSERT INTO pbs_alert_config (server_id, disk_pct, days_until_full, stale_hours, verify_alert, updated_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP) ON CONFLICT(server_id) DO UPDATE SET ...`)
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO pbs_alert_config (server_id, disk_pct, days_until_full, stale_hours, verify_alert, updated_at)
		VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(server_id) DO UPDATE SET
			disk_pct=excluded.disk_pct,
			days_until_full=excluded.days_until_full,
			stale_hours=excluded.stale_hours,
			verify_alert=excluded.verify_alert,
			updated_at=excluded.updated_at`,
		cfg.ServerID, nullInt(cfg.DiskPct), nullInt(cfg.DaysUntilFull), nullInt(cfg.StaleHours), verifyInt,
	)
	return err
}

// nullInt converts *int to a SQL-compatible value (nil → NULL, &v → v).
func nullInt(p *int) any {
	if p == nil {
		return nil
	}
	return *p
}

func nullString(p *string) any {
	if p == nil {
		return nil
	}
	return *p
}

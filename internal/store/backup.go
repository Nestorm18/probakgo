package store

import (
	"database/sql"
	"time"

	"probakgo/internal/domain"
)

func (s *Store) ListVMBackupConfigs(serverName string) ([]domain.VMBackupConfig, error) {
	rows, err := s.db.Query(`SELECT id, server_name, vm_id, vm_name, monday, tuesday, wednesday,
		thursday, friday, saturday, sunday, is_excluded, is_deleted, deleted_at, created_at
		FROM vm_backup_configs WHERE server_name = ? AND is_deleted = 0 ORDER BY vm_id`, serverName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanVMConfigs(rows)
}

func (s *Store) CreateVMBackupConfig(serverName string, req domain.CreateVMBackupConfigRequest) (int64, error) {
	res, err := s.db.Exec(
		`INSERT INTO vm_backup_configs (server_name, vm_id, vm_name, monday, tuesday, wednesday,
		 thursday, friday, saturday, sunday) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		serverName, req.VMID, req.VMName,
		boolToInt(req.Monday), boolToInt(req.Tuesday), boolToInt(req.Wednesday),
		boolToInt(req.Thursday), boolToInt(req.Friday), boolToInt(req.Saturday),
		boolToInt(req.Sunday),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) UpdateVMBackupConfig(serverName, vmID string, req domain.CreateVMBackupConfigRequest) error {
	_, err := s.db.Exec(
		`UPDATE vm_backup_configs SET vm_name=?, monday=?, tuesday=?, wednesday=?,
		 thursday=?, friday=?, saturday=?, sunday=?
		 WHERE server_name=? AND vm_id=? AND is_deleted=0`,
		req.VMName,
		boolToInt(req.Monday), boolToInt(req.Tuesday), boolToInt(req.Wednesday),
		boolToInt(req.Thursday), boolToInt(req.Friday), boolToInt(req.Saturday),
		boolToInt(req.Sunday), serverName, vmID,
	)
	return err
}

func (s *Store) ToggleVMExclude(serverName, vmID string) error {
	_, err := s.db.Exec(
		`UPDATE vm_backup_configs SET is_excluded = NOT is_excluded WHERE server_name=? AND vm_id=? AND is_deleted=0`,
		serverName, vmID,
	)
	return err
}

func (s *Store) DeleteVMBackupConfig(serverName, vmID string) error {
	_, err := s.db.Exec(
		`UPDATE vm_backup_configs SET is_deleted=1, deleted_at=? WHERE server_name=? AND vm_id=? AND is_deleted=0`,
		time.Now(), serverName, vmID,
	)
	return err
}

func scanVMConfigs(rows *sql.Rows) ([]domain.VMBackupConfig, error) {
	var configs []domain.VMBackupConfig
	for rows.Next() {
		var c domain.VMBackupConfig
		var mon, tue, wed, thu, fri, sat, sun, excl, del int
		var deletedAt sql.NullTime
		if err := rows.Scan(&c.ID, &c.ServerName, &c.VMID, &c.VMName,
			&mon, &tue, &wed, &thu, &fri, &sat, &sun,
			&excl, &del, &deletedAt, &c.CreatedAt); err != nil {
			return nil, err
		}
		c.Monday = mon != 0
		c.Tuesday = tue != 0
		c.Wednesday = wed != 0
		c.Thursday = thu != 0
		c.Friday = fri != 0
		c.Saturday = sat != 0
		c.Sunday = sun != 0
		c.IsExcluded = excl != 0
		c.IsDeleted = del != 0
		if deletedAt.Valid {
			c.DeletedAt = &deletedAt.Time
		}
		configs = append(configs, c)
	}
	return configs, rows.Err()
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

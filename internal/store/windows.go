package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) UpsertWindowsServer(ctx context.Context, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	debug.RecordQuery(ctx, `SELECT id FROM windows_servers WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM windows_servers WHERE name = ? AND is_deleted = 0`, name)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `INSERT INTO windows_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO windows_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert windows_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE windows_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE windows_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) UpsertWindowsServerForAPIKey(ctx context.Context, apiKeyID int64, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	if apiKeyID <= 0 {
		return s.UpsertWindowsServer(ctx, name, ip, publicIP, clientVersion, machineID)
	}
	debug.RecordQuery(ctx, `SELECT id FROM windows_servers WHERE api_key_id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM windows_servers WHERE api_key_id = ? AND is_deleted = 0`, apiKeyID)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `SELECT id FROM windows_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`)
		legacy := s.db.QueryRowContext(ctx, `SELECT id FROM windows_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`, name, machineID)
		if err := legacy.Scan(&id); err == nil {
			debug.RecordQuery(ctx, `UPDATE windows_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
			_, err := s.db.ExecContext(ctx,
				`UPDATE windows_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
				ip, publicIP, clientVersion, machineID, apiKeyID, id,
			)
			return id, err
		} else if err != sql.ErrNoRows {
			return 0, err
		}
		debug.RecordQuery(ctx, `INSERT INTO windows_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO windows_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID, apiKeyID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert windows_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE windows_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE windows_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) InsertWindowsReport(ctx context.Context, serverID int64) (int64, error) {
	debug.RecordQuery(ctx, `INSERT INTO windows_reports (server_id) VALUES (?)`)
	res, err := s.db.ExecContext(ctx, `INSERT INTO windows_reports (server_id) VALUES (?)`, serverID)
	if err != nil {
		return 0, fmt.Errorf("insert windows_report: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertWindowsDisk(ctx context.Context, reportID int64, disk domain.WindowsDiskPayload) error {
	debug.RecordQuery(ctx, `INSERT INTO windows_disks (report_id, name, label, file_system, drive_type, total, used, free, health) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO windows_disks (report_id, name, label, file_system, drive_type, total, used, free, health)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, disk.Name, disk.Label, disk.FileSystem, disk.DriveType, disk.Total, disk.Used, disk.Free, disk.Health,
	)
	return err
}

func (s *Store) ListWindowsServers(ctx context.Context) ([]domain.WindowsServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM windows_servers LEFT JOIN api_keys ON ... WHERE is_deleted = 0 ORDER BY display_name`)
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM windows_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.is_deleted = 0
		ORDER BY display_name, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []domain.WindowsServer
	for rows.Next() {
		var sv domain.WindowsServer
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
			&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, sv)
	}
	return servers, rows.Err()
}

func (s *Store) GetWindowsServer(ctx context.Context, id int64) (*domain.WindowsServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM windows_servers LEFT JOIN api_keys ON ... WHERE id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM windows_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.id = ? AND s.is_deleted = 0`, id)
	var sv domain.WindowsServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetWindowsServerByName(ctx context.Context, name string) (*domain.WindowsServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM windows_servers LEFT JOIN api_keys ON ... WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM windows_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.name = ? AND s.is_deleted = 0
		ORDER BY s.api_key_id IS NULL, s.id
		LIMIT 1`, name)
	var sv domain.WindowsServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetLatestWindowsReport(ctx context.Context, serverID int64) (*domain.WindowsReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT 1`)
	row := s.db.QueryRowContext(ctx, `SELECT id, server_id, reported_at, is_stale
		FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT 1`, serverID)
	var r domain.WindowsReport
	var isStale int
	if err := row.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale); err != nil {
		return nil, err
	}
	r.IsStale = isStale != 0
	return &r, nil
}

func (s *Store) GetLatestWindowsReports(ctx context.Context) (map[int64]*domain.WindowsReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale FROM windows_reports r WHERE r.id = (SELECT r2.id FROM windows_reports r2 WHERE r2.server_id = r.server_id ORDER BY r2.reported_at DESC, r2.id DESC LIMIT 1)`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale
		FROM windows_reports r
		WHERE r.id = (
			SELECT r2.id FROM windows_reports r2
			WHERE r2.server_id = r.server_id
			ORDER BY r2.reported_at DESC, r2.id DESC
			LIMIT 1
		)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make(map[int64]*domain.WindowsReport)
	for rows.Next() {
		var r domain.WindowsReport
		var isStale int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		reports[r.ServerID] = &r
	}
	return reports, rows.Err()
}

func (s *Store) ListWindowsReports(ctx context.Context, serverID int64, limit int) ([]domain.WindowsReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale
		FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.WindowsReport
	for rows.Next() {
		var r domain.WindowsReport
		var isStale int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) ListWindowsReportsPage(ctx context.Context, serverID int64, limit, offset int) ([]domain.WindowsReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale
		FROM windows_reports WHERE server_id = ? ORDER BY reported_at DESC, id DESC LIMIT ? OFFSET ?`, serverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.WindowsReport
	for rows.Next() {
		var r domain.WindowsReport
		var isStale int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) CountWindowsReports(ctx context.Context, serverID int64) (int, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM windows_reports WHERE server_id = ?`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM windows_reports WHERE server_id = ?`, serverID).Scan(&count)
	return count, err
}

func (s *Store) GetWindowsDisksForReport(ctx context.Context, reportID int64) ([]domain.WindowsDisk, error) {
	debug.RecordQuery(ctx, `SELECT id, report_id, name, label, file_system, drive_type, total, used, free, health FROM windows_disks WHERE report_id = ? ORDER BY name`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, name, label, file_system, drive_type, total, used, free, health
		FROM windows_disks WHERE report_id = ? ORDER BY name`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var disks []domain.WindowsDisk
	for rows.Next() {
		var d domain.WindowsDisk
		if err := rows.Scan(&d.ID, &d.ReportID, &d.Name, &d.Label, &d.FileSystem, &d.DriveType, &d.Total, &d.Used, &d.Free, &d.Health); err != nil {
			return nil, err
		}
		disks = append(disks, d)
	}
	return disks, rows.Err()
}

func (s *Store) GetWindowsDisksForReports(ctx context.Context, reportIDs []int64) (map[int64][]domain.WindowsDisk, error) {
	if len(reportIDs) == 0 {
		return nil, nil
	}
	ph, args := int64InArgs(reportIDs)
	debug.RecordQuery(ctx, `SELECT id, report_id, name, label, file_system, drive_type, total, used, free, health FROM windows_disks WHERE report_id IN (...) ORDER BY report_id, name`)
	q := `SELECT id, report_id, name, label, file_system, drive_type, total, used, free, health
		FROM windows_disks WHERE report_id IN (` + ph + `) ORDER BY report_id, name`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]domain.WindowsDisk)
	for rows.Next() {
		var d domain.WindowsDisk
		if err := rows.Scan(&d.ID, &d.ReportID, &d.Name, &d.Label, &d.FileSystem, &d.DriveType, &d.Total, &d.Used, &d.Free, &d.Health); err != nil {
			return nil, err
		}
		result[d.ReportID] = append(result[d.ReportID], d)
	}
	return result, rows.Err()
}

// DeleteOldWindowsReports removes Windows reports and their disk rows older than cutoff.
// Returns the number of reports deleted.
func (s *Store) DeleteOldWindowsReports(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	if _, err := tx.ExecContext(ctx, `DELETE FROM windows_disks WHERE report_id IN (
		SELECT id FROM windows_reports WHERE reported_at < ?)`, cutoff); err != nil {
		return 0, err
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM windows_reports WHERE reported_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func (s *Store) UpsertPVEServer(ctx context.Context, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	debug.RecordQuery(ctx, `SELECT id FROM pve_servers WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM pve_servers WHERE name = ? AND is_deleted = 0`, name)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `INSERT INTO pve_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO pve_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert pve_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE pve_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE pve_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) UpsertPVEServerForAPIKey(ctx context.Context, apiKeyID int64, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	if apiKeyID <= 0 {
		return s.UpsertPVEServer(ctx, name, ip, publicIP, clientVersion, machineID)
	}
	debug.RecordQuery(ctx, `SELECT id FROM pve_servers WHERE api_key_id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM pve_servers WHERE api_key_id = ? AND is_deleted = 0`, apiKeyID)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `SELECT id FROM pve_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`)
		legacy := s.db.QueryRowContext(ctx, `SELECT id FROM pve_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`, name, machineID)
		if err := legacy.Scan(&id); err == nil {
			debug.RecordQuery(ctx, `UPDATE pve_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
			_, err := s.db.ExecContext(ctx,
				`UPDATE pve_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
				ip, publicIP, clientVersion, machineID, apiKeyID, id,
			)
			return id, err
		} else if err != sql.ErrNoRows {
			return 0, err
		}
		debug.RecordQuery(ctx, `INSERT INTO pve_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO pve_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID, apiKeyID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert pve_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE pve_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE pve_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) InsertPVEReport(ctx context.Context, serverID int64, bs *domain.BackupStatus) (int64, error) {
	return s.InsertPVEReportWithSwap(ctx, serverID, bs, domain.HostSwap{})
}

func (s *Store) InsertPVEReportWithSwap(ctx context.Context, serverID int64, bs *domain.BackupStatus, swap domain.HostSwap) (int64, error) {
	status := ""
	var starttime, endtime, duration int64
	if bs != nil {
		status = bs.StatusString()
		starttime = bs.StartTime
		endtime = bs.EndTime
		duration = bs.Duration
	}
	swapEnabled := 0
	if swap.Enabled {
		swapEnabled = 1
	}
	debug.RecordQuery(ctx, `INSERT INTO pve_reports (server_id, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO pve_reports (server_id, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		serverID, status, starttime, endtime, duration, swap.Total, swap.Used, swapEnabled,
	)
	if err != nil {
		return 0, fmt.Errorf("insert pve_report: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertPVEStorage(ctx context.Context, reportID int64, st domain.StoragePayload) (int64, error) {
	pruneJSON, _ := json.Marshal(st.PruneBackups)
	shared := 0
	if st.Shared {
		shared = 1
	}
	debug.RecordQuery(ctx, `INSERT INTO pve_storages (report_id, storage, path, content, type, status, shared, server, digest, prune_backups) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	res, err := s.db.ExecContext(ctx,
		`INSERT INTO pve_storages (report_id, storage, path, content, type, status, shared, server, digest, prune_backups)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, st.Storage, st.Path, st.Content, st.Type, st.Status,
		shared, st.Server, st.Digest, string(pruneJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("insert pve_storage: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertPVEStorageInfo(ctx context.Context, storageID int64, info domain.StorageInfoPayload) error {
	active, enabled := 0, 0
	if info.Active {
		active = 1
	}
	if info.Enabled {
		enabled = 1
	}
	debug.RecordQuery(ctx, `INSERT INTO pve_storage_info (storage_id, total, used, avail, used_percent, active, enabled, lvl) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pve_storage_info (storage_id, total, used, avail, used_percent, active, enabled, lvl)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		storageID, info.Total, info.Used, info.Avail, info.UsedPct, active, enabled, info.Lvl,
	)
	return err
}

func (s *Store) InsertPVEStorageContent(ctx context.Context, storageID int64, c domain.ContentDataPayload) error {
	debug.RecordQuery(ctx, `INSERT INTO pve_storage_content (storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pve_storage_content (storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storageID, c.VMID, c.Format, c.Size, c.Content, c.VolID, c.CTime, c.Subtype, c.Notes, c.Verification,
	)
	return err
}

func (s *Store) ListPVEServers(ctx context.Context) ([]domain.PVEServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pve_servers LEFT JOIN api_keys ON ... WHERE is_deleted = 0 ORDER BY display_name`)
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pve_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.is_deleted = 0
		ORDER BY display_name, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []domain.PVEServer
	for rows.Next() {
		var sv domain.PVEServer
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
			&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, sv)
	}
	return servers, rows.Err()
}

func (s *Store) GetPVEServer(ctx context.Context, id int64) (*domain.PVEServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pve_servers LEFT JOIN api_keys ON ... WHERE id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pve_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.id = ? AND s.is_deleted = 0`, id)
	var sv domain.PVEServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetPVEServerByName(ctx context.Context, name string) (*domain.PVEServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pve_servers LEFT JOIN api_keys ON ... WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pve_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.name = ? AND s.is_deleted = 0
		ORDER BY s.api_key_id IS NULL, s.id
		LIMIT 1`, name)
	var sv domain.PVEServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

type PVEReportRow struct {
	domain.PVEReport
	ServerName string `db:"server_name"`
}

func (s *Store) GetLatestPVEReport(ctx context.Context, serverID int64) (*domain.PVEReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT 1`)
	row := s.db.QueryRowContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT 1`, serverID)
	var r domain.PVEReport
	var isStale int
	var staleReason sql.NullString
	var swapEnabled int
	if err := row.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
		&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
		&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
		return nil, err
	}
	r.IsStale = isStale != 0
	r.StaleReason = staleReason.String
	r.SwapEnabled = swapEnabled != 0
	return &r, nil
}

func (s *Store) GetLatestPVEReports(ctx context.Context) (map[int64]*domain.PVEReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports r WHERE r.id = (SELECT r2.id FROM pve_reports r2 WHERE r2.server_id = r.server_id ORDER BY r2.reported_at DESC, r2.id DESC LIMIT 1)`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports r
		WHERE r.id = (
			SELECT r2.id FROM pve_reports r2
			WHERE r2.server_id = r.server_id
			ORDER BY r2.reported_at DESC, r2.id DESC
			LIMIT 1
		)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make(map[int64]*domain.PVEReport)
	for rows.Next() {
		var r domain.PVEReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
			&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		r.SwapEnabled = swapEnabled != 0
		reports[r.ServerID] = &r
	}
	return reports, rows.Err()
}

func (s *Store) ListPVEReports(ctx context.Context, serverID int64, limit int) ([]domain.PVEReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PVEReport
	for rows.Next() {
		var r domain.PVEReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
			&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		r.SwapEnabled = swapEnabled != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) ListPVEReportsPage(ctx context.Context, serverID int64, limit, offset int) ([]domain.PVEReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`, serverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PVEReport
	for rows.Next() {
		var r domain.PVEReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
			&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		r.SwapEnabled = swapEnabled != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) CountPVEReports(ctx context.Context, serverID int64) (int, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM pve_reports WHERE server_id = ?`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pve_reports WHERE server_id = ?`, serverID).Scan(&count)
	return count, err
}

func (s *Store) GetPVEStoragesForReport(ctx context.Context, reportID int64) ([]domain.PVEStorage, error) {
	debug.RecordQuery(ctx, `SELECT id, report_id, storage, path, content, type, status, shared, server, digest, prune_backups FROM pve_storages WHERE report_id = ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, storage, path, content, type, status, shared, server, digest, prune_backups
		FROM pve_storages WHERE report_id = ?`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var storages []domain.PVEStorage
	for rows.Next() {
		var st domain.PVEStorage
		var shared int
		if err := rows.Scan(&st.ID, &st.ReportID, &st.Storage, &st.Path, &st.Content,
			&st.Type, &st.Status, &shared, &st.Server, &st.Digest, &st.PruneBackups); err != nil {
			return nil, err
		}
		st.Shared = shared != 0
		storages = append(storages, st)
	}
	return storages, rows.Err()
}

func (s *Store) GetPVEStorageContent(ctx context.Context, storageID int64) ([]domain.PVEStorageContent, error) {
	debug.RecordQuery(ctx, `SELECT id, storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification FROM pve_storage_content WHERE storage_id = ? ORDER BY ctime DESC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification
		FROM pve_storage_content WHERE storage_id = ? ORDER BY ctime DESC`, storageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.PVEStorageContent
	for rows.Next() {
		var c domain.PVEStorageContent
		if err := rows.Scan(&c.ID, &c.StorageID, &c.VMID, &c.Format, &c.Size,
			&c.Content, &c.VolID, &c.CTime, &c.Subtype, &c.Notes, &c.Verification); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

func (s *Store) ListPVEReportsByDays(ctx context.Context, serverID int64, days int) ([]domain.PVEReport, error) {
	threshold := time.Now().AddDate(0, 0, -days)
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports WHERE server_id = ? AND reported_at >= ? ORDER BY reported_at DESC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports WHERE server_id = ? AND reported_at >= ? ORDER BY reported_at DESC`,
		serverID, threshold)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PVEReport
	for rows.Next() {
		var r domain.PVEReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
			&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		r.SwapEnabled = swapEnabled != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) ListPVEReportsByDaysPage(ctx context.Context, serverID int64, days, limit, offset int) ([]domain.PVEReport, error) {
	threshold := time.Now().AddDate(0, 0, -days)
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled FROM pve_reports WHERE server_id = ? AND reported_at >= ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration, swap_total, swap_used, swap_enabled
		FROM pve_reports WHERE server_id = ? AND reported_at >= ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`,
		serverID, threshold, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PVEReport
	for rows.Next() {
		var r domain.PVEReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration,
			&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		r.SwapEnabled = swapEnabled != 0
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) CountPVEReportsByDays(ctx context.Context, serverID int64, days int) (int, error) {
	threshold := time.Now().AddDate(0, 0, -days)
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM pve_reports WHERE server_id = ? AND reported_at >= ?`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pve_reports WHERE server_id = ? AND reported_at >= ?`, serverID, threshold).Scan(&count)
	return count, err
}

func (s *Store) GetPVEStorageInfo(ctx context.Context, storageID int64) (*domain.PVEStorageInfo, error) {
	debug.RecordQuery(ctx, `SELECT id, storage_id, total, used, avail, used_percent, active, enabled, lvl FROM pve_storage_info WHERE storage_id = ? LIMIT 1`)
	row := s.db.QueryRowContext(ctx, `SELECT id, storage_id, total, used, avail, used_percent, active, enabled, lvl
		FROM pve_storage_info WHERE storage_id = ? LIMIT 1`, storageID)
	var info domain.PVEStorageInfo
	var active, enabled int
	if err := row.Scan(&info.ID, &info.StorageID, &info.Total, &info.Used, &info.Avail,
		&info.UsedPct, &active, &enabled, &info.Lvl); err != nil {
		return nil, err
	}
	info.Active = active != 0
	info.Enabled = enabled != 0
	return &info, nil
}

func (s *Store) InsertPVEBackupTask(ctx context.Context, reportID int64, t domain.BackupTaskPayload) error {
	debug.RecordQuery(ctx, `INSERT INTO pve_backup_tasks (report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO pve_backup_tasks (report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, t.VMID, t.VMName, t.Status, t.StartTime, t.EndTime, t.Duration, t.Size, t.Filename,
	)
	return err
}

func (s *Store) GetPVEBackupTasksForReport(ctx context.Context, reportID int64) ([]domain.PVEBackupTask, error) {
	debug.RecordQuery(ctx, `SELECT id, report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename FROM pve_backup_tasks WHERE report_id = ? ORDER BY vmid ASC`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename
		FROM pve_backup_tasks WHERE report_id = ? ORDER BY vmid ASC`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []domain.PVEBackupTask
	for rows.Next() {
		var t domain.PVEBackupTask
		if err := rows.Scan(&t.ID, &t.ReportID, &t.VMID, &t.VMName, &t.Status,
			&t.StartTime, &t.EndTime, &t.Duration, &t.Size, &t.Filename); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

func (s *Store) GetPVEBackupTasksForReports(ctx context.Context, reportIDs []int64) (map[int64][]domain.PVEBackupTask, error) {
	if len(reportIDs) == 0 {
		return nil, nil
	}
	ph, args := int64InArgs(reportIDs)
	debug.RecordQuery(ctx, `SELECT id, report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename FROM pve_backup_tasks WHERE report_id IN (...) ORDER BY report_id, vmid ASC`)
	q := `SELECT id, report_id, vmid, vm_name, status, starttime, endtime, duration, size, filename
		FROM pve_backup_tasks WHERE report_id IN (` + ph + `) ORDER BY report_id, vmid ASC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]domain.PVEBackupTask)
	for rows.Next() {
		var t domain.PVEBackupTask
		if err := rows.Scan(&t.ID, &t.ReportID, &t.VMID, &t.VMName, &t.Status,
			&t.StartTime, &t.EndTime, &t.Duration, &t.Size, &t.Filename); err != nil {
			return nil, err
		}
		result[t.ReportID] = append(result[t.ReportID], t)
	}
	return result, rows.Err()
}

func (s *Store) GetPVEStorageContentForStorages(ctx context.Context, storageIDs []int64) (map[int64][]domain.PVEStorageContent, error) {
	if len(storageIDs) == 0 {
		return nil, nil
	}
	ph, args := int64InArgs(storageIDs)
	q := `SELECT id, storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification
		FROM pve_storage_content WHERE storage_id IN (` + ph + `) ORDER BY storage_id, ctime DESC`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]domain.PVEStorageContent)
	for rows.Next() {
		var c domain.PVEStorageContent
		if err := rows.Scan(&c.ID, &c.StorageID, &c.VMID, &c.Format, &c.Size,
			&c.Content, &c.VolID, &c.CTime, &c.Subtype, &c.Notes, &c.Verification); err != nil {
			return nil, err
		}
		result[c.StorageID] = append(result[c.StorageID], c)
	}
	return result, rows.Err()
}

func (s *Store) GetPVEStorageInfoForStorages(ctx context.Context, storageIDs []int64) (map[int64]*domain.PVEStorageInfo, error) {
	if len(storageIDs) == 0 {
		return nil, nil
	}
	ph, args := int64InArgs(storageIDs)
	q := `SELECT id, storage_id, total, used, avail, used_percent, active, enabled, lvl
		FROM pve_storage_info WHERE storage_id IN (` + ph + `)`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64]*domain.PVEStorageInfo)
	for rows.Next() {
		var info domain.PVEStorageInfo
		var active, enabled int
		if err := rows.Scan(&info.ID, &info.StorageID, &info.Total, &info.Used, &info.Avail,
			&info.UsedPct, &active, &enabled, &info.Lvl); err != nil {
			return nil, err
		}
		info.Active = active != 0
		info.Enabled = enabled != 0
		result[info.StorageID] = &info
	}
	return result, rows.Err()
}

func int64InArgs(ids []int64) (placeholders string, args []any) {
	ph := make([]string, len(ids))
	args = make([]any, len(ids))
	for i, id := range ids {
		ph[i] = "?"
		args[i] = id
	}
	return strings.Join(ph, ","), args
}

func (s *Store) MarkPVEReportStale(ctx context.Context, reportID int64, reason string) error {
	debug.RecordQuery(ctx, `UPDATE pve_reports SET is_stale=1, stale_reason=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE pve_reports SET is_stale=1, stale_reason=? WHERE id=?`, reason, reportID)
	return err
}

func (s *Store) DeletePVEServer(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE pve_servers SET is_deleted=1, updated_at=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE pve_servers SET is_deleted=1, updated_at=? WHERE id=?`, time.Now(), id)
	return err
}

// DeleteOldPVEReports removes PVE reports (and their child rows) older than cutoff.
// Returns the number of reports deleted.
func (s *Store) DeleteOldPVEReports(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	steps := []string{
		`DELETE FROM pve_storage_content WHERE storage_id IN (
			SELECT s.id FROM pve_storages s
			JOIN pve_reports r ON r.id = s.report_id
			WHERE r.reported_at < ?)`,
		`DELETE FROM pve_storage_info WHERE storage_id IN (
			SELECT s.id FROM pve_storages s
			JOIN pve_reports r ON r.id = s.report_id
			WHERE r.reported_at < ?)`,
		`DELETE FROM pve_storages WHERE report_id IN (
			SELECT id FROM pve_reports WHERE reported_at < ?)`,
		`DELETE FROM pve_backup_tasks WHERE report_id IN (
			SELECT id FROM pve_reports WHERE reported_at < ?)`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q, cutoff); err != nil {
			return 0, err
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM pve_reports WHERE reported_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

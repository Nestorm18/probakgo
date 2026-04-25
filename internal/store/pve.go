package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"probakgo/internal/domain"
)

func (s *Store) UpsertPVEServer(name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	row := s.db.QueryRow(`SELECT id FROM pve_servers WHERE name = ? AND is_deleted = 0`, name)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		res, err := s.db.Exec(
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
	_, err := s.db.Exec(
		`UPDATE pve_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) InsertPVEReport(serverID int64, bs *domain.BackupStatus) (int64, error) {
	status := ""
	var starttime, endtime, duration int64
	if bs != nil {
		status = bs.StatusString()
		starttime = bs.StartTime
		endtime = bs.EndTime
		duration = bs.Duration
	}
	res, err := s.db.Exec(
		`INSERT INTO pve_reports (server_id, backup_status, backup_starttime, backup_endtime, backup_duration)
		 VALUES (?, ?, ?, ?, ?)`,
		serverID, status, starttime, endtime, duration,
	)
	if err != nil {
		return 0, fmt.Errorf("insert pve_report: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertPVEStorage(reportID int64, st domain.StoragePayload) (int64, error) {
	pruneJSON, _ := json.Marshal(st.PruneBackups)
	shared := 0
	if st.Shared {
		shared = 1
	}
	res, err := s.db.Exec(
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

func (s *Store) InsertPVEStorageInfo(storageID int64, info domain.StorageInfoPayload) error {
	active, enabled := 0, 0
	if info.Active {
		active = 1
	}
	if info.Enabled {
		enabled = 1
	}
	_, err := s.db.Exec(
		`INSERT INTO pve_storage_info (storage_id, total, used, avail, used_percent, active, enabled, lvl)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		storageID, info.Total, info.Used, info.Avail, info.UsedPct, active, enabled, info.Lvl,
	)
	return err
}

func (s *Store) InsertPVEStorageContent(storageID int64, c domain.ContentDataPayload) error {
	_, err := s.db.Exec(
		`INSERT INTO pve_storage_content (storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storageID, c.VMID, c.Format, c.Size, c.Content, c.VolID, c.CTime, c.Subtype, c.Notes, c.Verification,
	)
	return err
}

func (s *Store) ListPVEServers() ([]domain.PVEServer, error) {
	rows, err := s.db.Query(`SELECT id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at
		FROM pve_servers WHERE is_deleted = 0 ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []domain.PVEServer
	for rows.Next() {
		var sv domain.PVEServer
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
			&sv.MachineID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, sv)
	}
	return servers, rows.Err()
}

func (s *Store) GetPVEServer(id int64) (*domain.PVEServer, error) {
	row := s.db.QueryRow(`SELECT id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at
		FROM pve_servers WHERE id = ? AND is_deleted = 0`, id)
	var sv domain.PVEServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

type PVEReportRow struct {
	domain.PVEReport
	ServerName string `db:"server_name"`
}

func (s *Store) GetLatestPVEReport(serverID int64) (*domain.PVEReport, error) {
	row := s.db.QueryRow(`SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration
		FROM pve_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT 1`, serverID)
	var r domain.PVEReport
	var isStale int
	var staleReason sql.NullString
	if err := row.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
		&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration); err != nil {
		return nil, err
	}
	r.IsStale = isStale != 0
	r.StaleReason = staleReason.String
	return &r, nil
}

func (s *Store) ListPVEReports(serverID int64, limit int) ([]domain.PVEReport, error) {
	rows, err := s.db.Query(`SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration
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
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) GetPVEStoragesForReport(reportID int64) ([]domain.PVEStorage, error) {
	rows, err := s.db.Query(`SELECT id, report_id, storage, path, content, type, status, shared, server, digest, prune_backups
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

func (s *Store) GetPVEStorageContent(storageID int64) ([]domain.PVEStorageContent, error) {
	rows, err := s.db.Query(`SELECT id, storage_id, vmid, format, size, content, volid, ctime, subtype, notes, verification
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

func (s *Store) ListPVEReportsByDays(serverID int64, days int) ([]domain.PVEReport, error) {
	threshold := time.Now().AddDate(0, 0, -days)
	rows, err := s.db.Query(`SELECT id, server_id, reported_at, is_stale, stale_reason,
		backup_status, backup_starttime, backup_endtime, backup_duration
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
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
			&r.BackupStatus, &r.BackupStarttime, &r.BackupEndtime, &r.BackupDuration); err != nil {
			return nil, err
		}
		r.IsStale = isStale != 0
		r.StaleReason = staleReason.String
		reports = append(reports, r)
	}
	return reports, rows.Err()
}

func (s *Store) GetPVEStorageInfo(storageID int64) (*domain.PVEStorageInfo, error) {
	row := s.db.QueryRow(`SELECT id, storage_id, total, used, avail, used_percent, active, enabled, lvl
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

func (s *Store) MarkPVEReportStale(reportID int64, reason string) error {
	_, err := s.db.Exec(`UPDATE pve_reports SET is_stale=1, stale_reason=? WHERE id=?`, reason, reportID)
	return err
}

func (s *Store) DeletePVEServer(id int64) error {
	_, err := s.db.Exec(`UPDATE pve_servers SET is_deleted=1, updated_at=? WHERE id=?`, time.Now(), id)
	return err
}

// DeleteOldPVEReports removes PVE reports (and their child rows) older than cutoff.
// Returns the number of reports deleted.
func (s *Store) DeleteOldPVEReports(cutoff time.Time) (int64, error) {
	tx, err := s.db.Begin()
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
	}
	for _, q := range steps {
		if _, err := tx.Exec(q, cutoff); err != nil {
			return 0, err
		}
	}

	res, err := tx.Exec(`DELETE FROM pve_reports WHERE reported_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

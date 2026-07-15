package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

type pbsExecer interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func (s *Store) UpsertPBSServer(ctx context.Context, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	debug.RecordQuery(ctx, `SELECT id FROM pbs_servers WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM pbs_servers WHERE name = ? AND is_deleted = 0`, name)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `INSERT INTO pbs_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO pbs_servers (name, ip, public_ip, client_version, machine_id) VALUES (?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert pbs_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE pbs_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE pbs_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) UpsertPBSServerForAPIKey(ctx context.Context, apiKeyID int64, name, ip, publicIP, clientVersion, machineID string) (int64, error) {
	if apiKeyID <= 0 {
		return s.UpsertPBSServer(ctx, name, ip, publicIP, clientVersion, machineID)
	}
	debug.RecordQuery(ctx, `SELECT id FROM pbs_servers WHERE api_key_id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT id FROM pbs_servers WHERE api_key_id = ? AND is_deleted = 0`, apiKeyID)
	var id int64
	if err := row.Scan(&id); err == sql.ErrNoRows {
		debug.RecordQuery(ctx, `SELECT id FROM pbs_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`)
		legacy := s.db.QueryRowContext(ctx, `SELECT id FROM pbs_servers WHERE name = ? AND machine_id = ? AND api_key_id IS NULL AND is_deleted = 0`, name, machineID)
		if err := legacy.Scan(&id); err == nil {
			debug.RecordQuery(ctx, `UPDATE pbs_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
			_, err := s.db.ExecContext(ctx,
				`UPDATE pbs_servers SET ip=?, public_ip=?, client_version=?, machine_id=?, api_key_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
				ip, publicIP, clientVersion, machineID, apiKeyID, id,
			)
			return id, err
		} else if err != sql.ErrNoRows {
			return 0, err
		}
		debug.RecordQuery(ctx, `INSERT INTO pbs_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`)
		res, err := s.db.ExecContext(ctx,
			`INSERT INTO pbs_servers (name, ip, public_ip, client_version, machine_id, api_key_id) VALUES (?, ?, ?, ?, ?, ?)`,
			name, ip, publicIP, clientVersion, machineID, apiKeyID,
		)
		if err != nil {
			return 0, fmt.Errorf("insert pbs_server: %w", err)
		}
		return res.LastInsertId()
	} else if err != nil {
		return 0, err
	}
	debug.RecordQuery(ctx, `UPDATE pbs_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`)
	_, err := s.db.ExecContext(ctx,
		`UPDATE pbs_servers SET name=?, ip=?, public_ip=?, client_version=?, machine_id=?, updated_at=CURRENT_TIMESTAMP WHERE id=?`,
		name, ip, publicIP, clientVersion, machineID, id,
	)
	return id, err
}

func (s *Store) InsertPBSReport(ctx context.Context, serverID int64) (int64, error) {
	return s.InsertPBSReportWithSwap(ctx, serverID, domain.HostSwap{})
}

func (s *Store) InsertPBSReportWithSwap(ctx context.Context, serverID int64, swap domain.HostSwap) (int64, error) {
	return insertPBSReportWithSwap(ctx, s.db, serverID, swap)
}

func insertPBSReportWithSwap(ctx context.Context, db pbsExecer, serverID int64, swap domain.HostSwap) (int64, error) {
	swapEnabled := 0
	if swap.Enabled {
		swapEnabled = 1
	}
	debug.RecordQuery(ctx, `INSERT INTO pbs_reports (server_id, swap_total, swap_used, swap_enabled) VALUES (?, ?, ?, ?)`)
	res, err := db.ExecContext(ctx, `INSERT INTO pbs_reports (server_id, swap_total, swap_used, swap_enabled) VALUES (?, ?, ?, ?)`,
		serverID, swap.Total, swap.Used, swapEnabled)
	if err != nil {
		return 0, fmt.Errorf("insert pbs_report: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertPBSStore(ctx context.Context, reportID int64, ds domain.PBSDatastorePayload) (int64, error) {
	return insertPBSStore(ctx, s.db, reportID, ds)
}

func insertPBSStore(ctx context.Context, db pbsExecer, reportID int64, ds domain.PBSDatastorePayload) (int64, error) {
	debug.RecordQuery(ctx, `INSERT INTO pbs_stores (report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	res, err := db.ExecContext(ctx,
		`INSERT INTO pbs_stores (report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, ds.Store, ds.Total, ds.Used, ds.Avail,
		ds.EstimatedFullDate, ds.MountStatus, ds.HistoryStart, ds.HistoryDelta,
	)
	if err != nil {
		return 0, fmt.Errorf("insert pbs_store: %w", err)
	}
	return res.LastInsertId()
}

func (s *Store) InsertPBSStoreHistory(ctx context.Context, storeID int64, history []*float64) error {
	return insertPBSStoreHistory(ctx, s.db, storeID, history)
}

func insertPBSStoreHistory(ctx context.Context, db pbsExecer, storeID int64, history []*float64) error {
	if len(history) == 0 {
		return nil
	}
	debug.RecordQuery(ctx, `INSERT INTO pbs_store_history (store_id, position, value) VALUES (?, ?, ?)`)
	for i, v := range history {
		_, err := db.ExecContext(ctx, `INSERT INTO pbs_store_history (store_id, position, value) VALUES (?, ?, ?)`, storeID, i, v)
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) InsertPBSSnapshot(ctx context.Context, storeID int64, g domain.PBSGroupPayload) error {
	return insertPBSSnapshot(ctx, s.db, storeID, g)
}

func insertPBSSnapshot(ctx context.Context, db pbsExecer, storeID int64, g domain.PBSGroupPayload) error {
	debug.RecordQuery(ctx, `INSERT INTO pbs_snapshots (store_id, backup_type, backup_id, last_backup, backup_count, owner, comment, verification_state, size) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := db.ExecContext(ctx,
		`INSERT INTO pbs_snapshots (store_id, backup_type, backup_id, last_backup, backup_count,
		 owner, comment, verification_state, size)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storeID, g.BackupType, g.BackupID, g.LastBackup, g.BackupCount,
		g.Owner, g.Comment, g.VerificationState, g.Size,
	)
	if err != nil {
		return fmt.Errorf("insert pbs_snapshot: %w", err)
	}
	return nil
}

func (s *Store) GetPBSSnapshotsForStore(ctx context.Context, storeID int64) ([]domain.PBSSnapshot, error) {
	debug.RecordQuery(ctx, `SELECT id, store_id, backup_type, backup_id, last_backup, backup_count, owner, comment, verification_state, size FROM pbs_snapshots WHERE store_id = ? ORDER BY backup_type, backup_id`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, store_id, backup_type, backup_id, last_backup, backup_count,
		owner, comment, verification_state, size
		FROM pbs_snapshots WHERE store_id = ? ORDER BY backup_type, backup_id`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snapshots []domain.PBSSnapshot
	for rows.Next() {
		var sn domain.PBSSnapshot
		if err := rows.Scan(&sn.ID, &sn.StoreID, &sn.BackupType, &sn.BackupID, &sn.LastBackup,
			&sn.BackupCount, &sn.Owner, &sn.Comment, &sn.VerificationState, &sn.Size); err != nil {
			return nil, err
		}
		snapshots = append(snapshots, sn)
	}
	return snapshots, rows.Err()
}

func (s *Store) InsertPBSGCStatus(ctx context.Context, storeID int64, gc *domain.GCStatusPayload) error {
	return insertPBSGCStatus(ctx, s.db, storeID, gc)
}

func insertPBSGCStatus(ctx context.Context, db pbsExecer, storeID int64, gc *domain.GCStatusPayload) error {
	if gc == nil {
		return nil
	}
	debug.RecordQuery(ctx, `INSERT INTO pbs_gc_status (store_id, disk_bytes, disk_chunks, index_data_bytes, index_file_count, pending_bytes, pending_chunks, removed_bad, removed_bytes, removed_chunks, still_bad, upid) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := db.ExecContext(ctx,
		`INSERT INTO pbs_gc_status (store_id, disk_bytes, disk_chunks, index_data_bytes, index_file_count,
		 pending_bytes, pending_chunks, removed_bad, removed_bytes, removed_chunks, still_bad, upid)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		storeID, gc.DiskBytes, gc.DiskChunks, gc.IndexDataBytes, gc.IndexFileCount,
		gc.PendingBytes, gc.PendingChunks, gc.RemovedBad, gc.RemovedBytes,
		gc.RemovedChunks, gc.StillBad, gc.UPID,
	)
	return err
}

func (s *Store) InsertPBSTask(ctx context.Context, reportID int64, task domain.PBSTaskPayload) error {
	return insertPBSTask(ctx, s.db, reportID, task)
}

func insertPBSTask(ctx context.Context, db pbsExecer, reportID int64, task domain.PBSTaskPayload) error {
	debug.RecordQuery(ctx, `INSERT INTO pbs_maintenance_tasks (report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	_, err := db.ExecContext(ctx, `INSERT INTO pbs_maintenance_tasks
		(report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		reportID, task.TaskType, task.JobID, task.Remote, task.RemoteStore, task.Store,
		task.Status, task.StartTime, task.EndTime, task.UPID,
	)
	return err
}

// InsertPBSReportData stores a complete PBS report atomically. Keeping all
// child inserts in one transaction avoids an fsync for every history point.
func (s *Store) InsertPBSReportData(ctx context.Context, serverID int64, swap domain.HostSwap, info domain.PBSInformation) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	reportID, err := insertPBSReportWithSwap(ctx, tx, serverID, swap)
	if err != nil {
		return err
	}
	for _, ds := range info.Data {
		storeID, err := insertPBSStore(ctx, tx, reportID, ds)
		if err != nil {
			return fmt.Errorf("insert pbs store %s: %w", ds.Store, err)
		}
		if err := insertPBSStoreHistory(ctx, tx, storeID, ds.History); err != nil {
			return fmt.Errorf("insert pbs history: %w", err)
		}
		if err := insertPBSGCStatus(ctx, tx, storeID, ds.GCStatus); err != nil {
			return fmt.Errorf("insert gc status: %w", err)
		}
		for _, group := range ds.Groups {
			if err := insertPBSSnapshot(ctx, tx, storeID, group); err != nil {
				return fmt.Errorf("insert pbs snapshot %s/%s: %w", group.BackupType, group.BackupID, err)
			}
		}
	}
	for _, task := range info.Tasks {
		if err := insertPBSTask(ctx, tx, reportID, task); err != nil {
			return fmt.Errorf("insert pbs maintenance task %s/%s: %w", task.TaskType, task.JobID, err)
		}
	}
	return tx.Commit()
}

func (s *Store) GetPBSTasksForReport(ctx context.Context, reportID int64) ([]domain.PBSTask, error) {
	debug.RecordQuery(ctx, `SELECT id, report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid FROM pbs_maintenance_tasks WHERE report_id = ? ORDER BY task_type, job_id, remote, store`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid
		FROM pbs_maintenance_tasks WHERE report_id = ? ORDER BY task_type, job_id, remote, store`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanPBSTasks(rows)
}

func (s *Store) GetPBSTasksForReports(ctx context.Context, reportIDs []int64) (map[int64][]domain.PBSTask, error) {
	if len(reportIDs) == 0 {
		return map[int64][]domain.PBSTask{}, nil
	}
	ph, args := int64InArgs(reportIDs)
	debug.RecordQuery(ctx, `SELECT id, report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid FROM pbs_maintenance_tasks WHERE report_id IN (...) ORDER BY report_id, task_type, job_id`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, task_type, job_id, remote, remote_store, store, status, start_time, end_time, upid
		FROM pbs_maintenance_tasks WHERE report_id IN (`+ph+`) ORDER BY report_id, task_type, job_id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tasks, err := scanPBSTasks(rows)
	if err != nil {
		return nil, err
	}
	result := make(map[int64][]domain.PBSTask)
	for _, task := range tasks {
		result[task.ReportID] = append(result[task.ReportID], task)
	}
	return result, nil
}

func scanPBSTasks(rows *sql.Rows) ([]domain.PBSTask, error) {
	var tasks []domain.PBSTask
	for rows.Next() {
		var task domain.PBSTask
		if err := rows.Scan(&task.ID, &task.ReportID, &task.TaskType, &task.JobID, &task.Remote,
			&task.RemoteStore, &task.Store, &task.Status, &task.StartTime, &task.EndTime, &task.UPID); err != nil {
			return nil, err
		}
		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

func (s *Store) ListPBSServers(ctx context.Context) ([]domain.PBSServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pbs_servers LEFT JOIN api_keys ON ... WHERE is_deleted = 0 ORDER BY display_name`)
	rows, err := s.db.QueryContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pbs_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.is_deleted = 0
		ORDER BY display_name, s.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var servers []domain.PBSServer
	for rows.Next() {
		var sv domain.PBSServer
		if err := rows.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
			&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
			return nil, err
		}
		servers = append(servers, sv)
	}
	return servers, rows.Err()
}

func (s *Store) GetPBSServer(ctx context.Context, id int64) (*domain.PBSServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pbs_servers LEFT JOIN api_keys ON ... WHERE id = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pbs_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.id = ? AND s.is_deleted = 0`, id)
	var sv domain.PBSServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetPBSServerByName(ctx context.Context, name string) (*domain.PBSServer, error) {
	debug.RecordQuery(ctx, `SELECT id, name, display_name, ip, public_ip, client_version, machine_id, api_key_id, is_deleted, created_at, updated_at FROM pbs_servers LEFT JOIN api_keys ON ... WHERE name = ? AND is_deleted = 0`)
	row := s.db.QueryRowContext(ctx, `SELECT s.id, s.name, COALESCE(NULLIF(k.name, ''), s.name) AS display_name, s.ip, s.public_ip, s.client_version, s.machine_id, COALESCE(s.api_key_id, 0), s.is_deleted, s.created_at, s.updated_at
		FROM pbs_servers s
		LEFT JOIN api_keys k ON k.id = s.api_key_id
		WHERE s.name = ? AND s.is_deleted = 0
		ORDER BY s.api_key_id IS NULL, s.id
		LIMIT 1`, name)
	var sv domain.PBSServer
	if err := row.Scan(&sv.ID, &sv.Name, &sv.DisplayName, &sv.IP, &sv.PublicIP, &sv.ClientVersion,
		&sv.MachineID, &sv.APIKeyID, &sv.IsDeleted, &sv.CreatedAt, &sv.UpdatedAt); err != nil {
		return nil, err
	}
	return &sv, nil
}

func (s *Store) GetLatestPBSReport(ctx context.Context, serverID int64) (*domain.PBSReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT 1`)
	row := s.db.QueryRowContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled
		FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT 1`, serverID)
	var r domain.PBSReport
	var isStale int
	var staleReason sql.NullString
	var swapEnabled int
	if err := row.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
		&r.SwapTotal, &r.SwapUsed, &swapEnabled); err != nil {
		return nil, err
	}
	r.IsStale = isStale != 0
	r.StaleReason = staleReason.String
	r.SwapEnabled = swapEnabled != 0
	return &r, nil
}

func (s *Store) GetLatestPBSReports(ctx context.Context) (map[int64]*domain.PBSReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled FROM pbs_reports r WHERE r.id = (SELECT r2.id FROM pbs_reports r2 WHERE r2.server_id = r.server_id ORDER BY r2.reported_at DESC, r2.id DESC LIMIT 1)`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled
		FROM pbs_reports r
		WHERE r.id = (
			SELECT r2.id FROM pbs_reports r2
			WHERE r2.server_id = r.server_id
			ORDER BY r2.reported_at DESC, r2.id DESC
			LIMIT 1
		)`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	reports := make(map[int64]*domain.PBSReport)
	for rows.Next() {
		var r domain.PBSReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
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

func (s *Store) ListPBSReports(ctx context.Context, serverID int64, limit int) ([]domain.PBSReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled
		FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ?`, serverID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PBSReport
	for rows.Next() {
		var r domain.PBSReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
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

func (s *Store) ListPBSReportsPage(ctx context.Context, serverID int64, limit, offset int) ([]domain.PBSReport, error) {
	debug.RecordQuery(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, server_id, reported_at, is_stale, stale_reason, swap_total, swap_used, swap_enabled
		FROM pbs_reports WHERE server_id = ? ORDER BY reported_at DESC LIMIT ? OFFSET ?`, serverID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reports []domain.PBSReport
	for rows.Next() {
		var r domain.PBSReport
		var isStale int
		var staleReason sql.NullString
		var swapEnabled int
		if err := rows.Scan(&r.ID, &r.ServerID, &r.ReportedAt, &isStale, &staleReason,
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

func (s *Store) CountPBSReports(ctx context.Context, serverID int64) (int, error) {
	debug.RecordQuery(ctx, `SELECT COUNT(*) FROM pbs_reports WHERE server_id = ?`)
	var count int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pbs_reports WHERE server_id = ?`, serverID).Scan(&count)
	return count, err
}

func (s *Store) GetPBSStoresForReport(ctx context.Context, reportID int64) ([]domain.PBSStore, error) {
	debug.RecordQuery(ctx, `SELECT id, report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta FROM pbs_stores WHERE report_id = ?`)
	rows, err := s.db.QueryContext(ctx, `SELECT id, report_id, store, total, used, avail,
		estimated_full_date, mount_status, history_start, history_delta
		FROM pbs_stores WHERE report_id = ?`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var stores []domain.PBSStore
	for rows.Next() {
		var st domain.PBSStore
		if err := rows.Scan(&st.ID, &st.ReportID, &st.Store, &st.Total, &st.Used, &st.Avail,
			&st.EstimatedFullDate, &st.MountStatus, &st.HistoryStart, &st.HistoryDelta); err != nil {
			return nil, err
		}
		stores = append(stores, st)
	}
	return stores, rows.Err()
}

func (s *Store) GetPBSStoresForReports(ctx context.Context, reportIDs []int64) (map[int64][]domain.PBSStore, error) {
	if len(reportIDs) == 0 {
		return nil, nil
	}
	ph, args := int64InArgs(reportIDs)
	debug.RecordQuery(ctx, `SELECT id, report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta FROM pbs_stores WHERE report_id IN (...) ORDER BY report_id, store`)
	q := `SELECT id, report_id, store, total, used, avail, estimated_full_date, mount_status, history_start, history_delta
		FROM pbs_stores WHERE report_id IN (` + ph + `) ORDER BY report_id, store`
	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make(map[int64][]domain.PBSStore)
	for rows.Next() {
		var st domain.PBSStore
		if err := rows.Scan(&st.ID, &st.ReportID, &st.Store, &st.Total, &st.Used, &st.Avail,
			&st.EstimatedFullDate, &st.MountStatus, &st.HistoryStart, &st.HistoryDelta); err != nil {
			return nil, err
		}
		result[st.ReportID] = append(result[st.ReportID], st)
	}
	return result, rows.Err()
}

func (s *Store) GetPBSGCStatus(ctx context.Context, storeID int64) (*domain.PBSGCStatus, error) {
	debug.RecordQuery(ctx, `SELECT id, store_id, disk_bytes, disk_chunks, index_data_bytes, index_file_count, pending_bytes, pending_chunks, removed_bad, removed_bytes, removed_chunks, still_bad, upid FROM pbs_gc_status WHERE store_id = ?`)
	row := s.db.QueryRowContext(ctx, `SELECT id, store_id, disk_bytes, disk_chunks, index_data_bytes, index_file_count,
		pending_bytes, pending_chunks, removed_bad, removed_bytes, removed_chunks, still_bad, upid
		FROM pbs_gc_status WHERE store_id = ?`, storeID)
	var gc domain.PBSGCStatus
	err := row.Scan(&gc.ID, &gc.StoreID, &gc.DiskBytes, &gc.DiskChunks, &gc.IndexDataBytes,
		&gc.IndexFileCount, &gc.PendingBytes, &gc.PendingChunks, &gc.RemovedBad,
		&gc.RemovedBytes, &gc.RemovedChunks, &gc.StillBad, &gc.UPID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &gc, err
}

func (s *Store) GetPBSHistory(ctx context.Context, storeID int64) ([]*float64, error) {
	debug.RecordQuery(ctx, `SELECT value FROM pbs_store_history WHERE store_id = ? ORDER BY position`)
	rows, err := s.db.QueryContext(ctx, `SELECT value FROM pbs_store_history WHERE store_id = ? ORDER BY position`, storeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var history []*float64
	for rows.Next() {
		var v *float64
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		history = append(history, v)
	}
	return history, rows.Err()
}

func (s *Store) DeletePBSServer(ctx context.Context, id int64) error {
	debug.RecordQuery(ctx, `UPDATE pbs_servers SET is_deleted=1, updated_at=? WHERE id=?`)
	_, err := s.db.ExecContext(ctx, `UPDATE pbs_servers SET is_deleted=1, updated_at=? WHERE id=?`, time.Now(), id)
	return err
}

// DeleteOldPBSReports removes PBS reports (and their child rows) older than cutoff.
// Returns the number of reports deleted.
func (s *Store) DeleteOldPBSReports(ctx context.Context, cutoff time.Time) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	steps := []string{
		`DELETE FROM pbs_maintenance_tasks WHERE report_id IN (SELECT id FROM pbs_reports WHERE reported_at < ?)`,
		`DELETE FROM pbs_store_history WHERE store_id IN (
			SELECT st.id FROM pbs_stores st
			JOIN pbs_reports r ON r.id = st.report_id
			WHERE r.reported_at < ?)`,
		`DELETE FROM pbs_gc_status WHERE store_id IN (
			SELECT st.id FROM pbs_stores st
			JOIN pbs_reports r ON r.id = st.report_id
			WHERE r.reported_at < ?)`,
		`DELETE FROM pbs_snapshots WHERE store_id IN (
			SELECT st.id FROM pbs_stores st
			JOIN pbs_reports r ON r.id = st.report_id
			WHERE r.reported_at < ?)`,
		`DELETE FROM pbs_stores WHERE report_id IN (
			SELECT id FROM pbs_reports WHERE reported_at < ?)`,
	}
	for _, q := range steps {
		if _, err := tx.ExecContext(ctx, q, cutoff); err != nil {
			return 0, err
		}
	}

	res, err := tx.ExecContext(ctx, `DELETE FROM pbs_reports WHERE reported_at < ?`, cutoff)
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, tx.Commit()
}

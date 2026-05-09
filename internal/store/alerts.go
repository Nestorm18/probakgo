package store

import (
	"context"
	"fmt"
	"time"

	"probakgo/internal/debug"
	"probakgo/internal/domain"
)

func diskSeverity(pct int) string {
	if pct >= 95 {
		return domain.AlertSeverityCritical
	}
	return domain.AlertSeverityWarning
}

// GetAlerts returns active disk and backup-error alerts based on current thresholds.
// diskPct=0 disables disk checks. checkBackupErr=false disables backup error checks.
func (s *Store) GetAlerts(ctx context.Context, diskPct int, checkBackupErr bool) ([]domain.Alert, error) {
	var alerts []domain.Alert

	if diskPct > 0 {
		// PBS datastore disk usage
		debug.RecordQuery(ctx, `SELECT sv.name, st.store, st.used, st.total FROM pbs_stores st JOIN pbs_reports r ON r.id = st.report_id JOIN pbs_servers sv ON sv.id = r.server_id WHERE sv.is_deleted = 0 AND r.id IN (SELECT MAX(id) FROM pbs_reports GROUP BY server_id) AND st.total > 0 AND (CAST(st.used AS REAL) * 100 / st.total) >= ? ORDER BY sv.name, st.store`)
		rows, err := s.db.QueryContext(ctx, `
			SELECT sv.name, st.store, st.used, st.total
			FROM pbs_stores st
			JOIN pbs_reports r  ON r.id  = st.report_id
			JOIN pbs_servers sv ON sv.id = r.server_id
			WHERE sv.is_deleted = 0
			  AND r.id IN (SELECT MAX(id) FROM pbs_reports GROUP BY server_id)
			  AND st.total > 0
			  AND (CAST(st.used AS REAL) * 100 / st.total) >= ?
			ORDER BY sv.name, st.store`, diskPct)
		if err != nil {
			return nil, fmt.Errorf("pbs disk alerts: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var serverName, storeName string
			var used, total int64
			if err := rows.Scan(&serverName, &storeName, &used, &total); err != nil {
				return nil, err
			}
			pct := int(float64(used) / float64(total) * 100)
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:pbs:%s:%s", serverName, storeName),
				ServerName: serverName,
				ServerType: "pbs",
				StoreName:  storeName,
				Type:       domain.AlertTypeDisk,
				Severity:   diskSeverity(pct),
				Title:      "Disco casi lleno",
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, fmtBytes(used), fmtBytes(total)),
				Value:      fmt.Sprintf("%d", pct),
				Threshold:  fmt.Sprintf("%d%%", diskPct),
				DetectedAt: time.Now(),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		// PVE backup storage disk usage
		debug.RecordQuery(ctx, `SELECT sv.name, s.storage, si.used, si.total FROM pve_storage_info si JOIN pve_storages s ON s.id = si.storage_id JOIN pve_reports r ON r.id = s.report_id JOIN pve_servers sv ON sv.id = r.server_id WHERE sv.is_deleted = 0 AND r.id IN (SELECT MAX(id) FROM pve_reports GROUP BY server_id) AND s.content LIKE '%backup%' AND si.total > 0 AND (CAST(si.used AS REAL) * 100 / si.total) >= ? ORDER BY sv.name, s.storage`)
		rows2, err := s.db.QueryContext(ctx, `
			SELECT sv.name, s.storage, si.used, si.total
			FROM pve_storage_info si
			JOIN pve_storages s  ON s.id  = si.storage_id
			JOIN pve_reports r   ON r.id  = s.report_id
			JOIN pve_servers sv  ON sv.id = r.server_id
			WHERE sv.is_deleted = 0
			  AND r.id IN (SELECT MAX(id) FROM pve_reports GROUP BY server_id)
			  AND s.content LIKE '%backup%'
			  AND si.total > 0
			  AND (CAST(si.used AS REAL) * 100 / si.total) >= ?
			ORDER BY sv.name, s.storage`, diskPct)
		if err != nil {
			return nil, fmt.Errorf("pve disk alerts: %w", err)
		}
		defer rows2.Close()
		for rows2.Next() {
			var serverName, storageName string
			var used, total int64
			if err := rows2.Scan(&serverName, &storageName, &used, &total); err != nil {
				return nil, err
			}
			pct := int(float64(used) / float64(total) * 100)
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("disk:pve:%s:%s", serverName, storageName),
				ServerName: serverName,
				ServerType: "pve",
				StoreName:  storageName,
				Type:       domain.AlertTypeDisk,
				Severity:   diskSeverity(pct),
				Title:      "Disco casi lleno",
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, fmtBytes(used), fmtBytes(total)),
				Value:      fmt.Sprintf("%d", pct),
				Threshold:  fmt.Sprintf("%d%%", diskPct),
				DetectedAt: time.Now(),
			})
		}
		if err := rows2.Err(); err != nil {
			return nil, err
		}
	}

	if checkBackupErr {
		debug.RecordQuery(ctx, `SELECT sv.name, r.backup_status FROM pve_reports r JOIN pve_servers sv ON sv.id = r.server_id WHERE sv.is_deleted = 0 AND r.id IN (SELECT MAX(id) FROM pve_reports GROUP BY server_id) AND r.backup_status != '' AND r.backup_status != 'OK' ORDER BY sv.name`)
		rows, err := s.db.QueryContext(ctx, `
			SELECT sv.name, r.backup_status
			FROM pve_reports r
			JOIN pve_servers sv ON sv.id = r.server_id
			WHERE sv.is_deleted = 0
			  AND r.id IN (SELECT MAX(id) FROM pve_reports GROUP BY server_id)
			  AND r.backup_status != ''
			  AND r.backup_status != 'OK'
			ORDER BY sv.name`)
		if err != nil {
			return nil, fmt.Errorf("backup error alerts: %w", err)
		}
		defer rows.Close()
		for rows.Next() {
			var serverName, status string
			if err := rows.Scan(&serverName, &status); err != nil {
				return nil, err
			}
			alerts = append(alerts, domain.Alert{
				ID:         fmt.Sprintf("backup_error:pve:%s", serverName),
				ServerName: serverName,
				ServerType: "pve",
				Type:       domain.AlertTypeBackupError,
				Severity:   domain.AlertSeverityCritical,
				Title:      "Backup fallido",
				Message:    fmt.Sprintf("último backup fallido: %s", status),
				DetectedAt: time.Now(),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return alerts, nil
}

// GetPBSStaleAlerts returns alerts for PBS snapshots whose last_backup is older than staleHours.
// staleHours=0 disables the check.
func (s *Store) GetPBSStaleAlerts(ctx context.Context, staleHours int) ([]domain.Alert, error) {
	if staleHours <= 0 {
		return nil, nil
	}
	cutoff := time.Now().Unix() - int64(staleHours)*3600
	debug.RecordQuery(ctx, `SELECT sv.name, st.store, sn.backup_type, sn.backup_id, sn.last_backup FROM pbs_snapshots sn JOIN pbs_stores st ON st.id = sn.store_id JOIN pbs_reports r ON r.id = st.report_id JOIN pbs_servers sv ON sv.id = r.server_id WHERE sv.is_deleted = 0 AND r.id IN (SELECT MAX(id) FROM pbs_reports GROUP BY server_id) AND sn.last_backup > 0 AND sn.last_backup < ? ORDER BY sv.name, st.store, sn.backup_type, sn.backup_id`)
	rows, err := s.db.QueryContext(ctx, `
		SELECT sv.name, st.store, sn.backup_type, sn.backup_id, sn.last_backup
		FROM pbs_snapshots sn
		JOIN pbs_stores st ON st.id = sn.store_id
		JOIN pbs_reports r  ON r.id  = st.report_id
		JOIN pbs_servers sv ON sv.id = r.server_id
		WHERE sv.is_deleted = 0
		  AND r.id IN (SELECT MAX(id) FROM pbs_reports GROUP BY server_id)
		  AND sn.last_backup > 0
		  AND sn.last_backup < ?
		ORDER BY sv.name, st.store, sn.backup_type, sn.backup_id`, cutoff)
	if err != nil {
		return nil, fmt.Errorf("pbs stale alerts: %w", err)
	}
	defer rows.Close()
	var alerts []domain.Alert
	for rows.Next() {
		var serverName, storeName, btype, bid string
		var lastBackup int64
		if err := rows.Scan(&serverName, &storeName, &btype, &bid, &lastBackup); err != nil {
			return nil, err
		}
		h := int(time.Since(time.Unix(lastBackup, 0)).Hours())
		var since string
		if h >= 48 {
			since = fmt.Sprintf("%dd", h/24)
		} else {
			since = fmt.Sprintf("%dh", h)
		}
		alerts = append(alerts, domain.Alert{
			ID:         fmt.Sprintf("pbs_stale:pbs:%s:%s:%s/%s", serverName, storeName, btype, bid),
			ServerName: serverName,
			ServerType: "pbs",
			StoreName:  storeName,
			Type:       domain.AlertTypePBSStale,
			Severity:   domain.AlertSeverityWarning,
			Title:      "Snapshot sin actualizar",
			Message:    fmt.Sprintf("%s/%s sin backup desde hace %s", btype, bid, since),
			Value:      since,
			DetectedAt: time.Now(),
		})
	}
	return alerts, rows.Err()
}

func fmtBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

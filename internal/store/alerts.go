package store

import (
	"fmt"

	"probakgo/internal/domain"
)

// GetAlerts returns active disk and backup-error alerts based on current thresholds.
// diskPct=0 disables disk checks. checkBackupErr=false disables backup error checks.
func (s *Store) GetAlerts(diskPct int, checkBackupErr bool) ([]domain.Alert, error) {
	var alerts []domain.Alert

	if diskPct > 0 {
		// PBS datastore disk usage
		rows, err := s.db.Query(`
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
				ServerName: serverName,
				StoreName:  storeName,
				Type:       "disk",
				UsedPct:    pct,
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, fmtBytes(used), fmtBytes(total)),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}

		// PVE backup storage disk usage
		rows2, err := s.db.Query(`
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
				ServerName: serverName,
				StoreName:  storageName,
				Type:       "disk",
				UsedPct:    pct,
				Message:    fmt.Sprintf("%d%% usado (%s / %s)", pct, fmtBytes(used), fmtBytes(total)),
			})
		}
		if err := rows2.Err(); err != nil {
			return nil, err
		}
	}

	if checkBackupErr {
		rows, err := s.db.Query(`
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
				ServerName: serverName,
				Type:       "backup_error",
				Message:    fmt.Sprintf("último backup fallido: %s", status),
			})
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	return alerts, nil
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

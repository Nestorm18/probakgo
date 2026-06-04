PRAGMA foreign_keys=off;

CREATE TEMP TABLE orphan_pve_server_ids AS
SELECT id
FROM pve_servers
WHERE api_key_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM api_keys WHERE api_keys.id = pve_servers.api_key_id);

CREATE TEMP TABLE orphan_pbs_server_ids AS
SELECT id
FROM pbs_servers
WHERE api_key_id IS NOT NULL
  AND NOT EXISTS (SELECT 1 FROM api_keys WHERE api_keys.id = pbs_servers.api_key_id);

DELETE FROM alert_suppressions
WHERE EXISTS (
    SELECT 1 FROM orphan_pve_server_ids o
    WHERE alert_id LIKE '%:pve:' || o.id
       OR alert_id LIKE '%:pve:' || o.id || ':%'
);

DELETE FROM server_heartbeats
WHERE server_type = 'pve'
  AND server_id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM pve_vm_alert_config
WHERE server_id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM pve_alert_config
WHERE server_id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM vm_backup_configs
WHERE server_type = 'pve'
  AND server_id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM pve_backup_tasks
WHERE report_id IN (
    SELECT id FROM pve_reports WHERE server_id IN (SELECT id FROM orphan_pve_server_ids)
);

DELETE FROM pve_storage_content
WHERE storage_id IN (
    SELECT id
    FROM pve_storages
    WHERE report_id IN (
        SELECT id FROM pve_reports WHERE server_id IN (SELECT id FROM orphan_pve_server_ids)
    )
);

DELETE FROM pve_storage_info
WHERE storage_id IN (
    SELECT id
    FROM pve_storages
    WHERE report_id IN (
        SELECT id FROM pve_reports WHERE server_id IN (SELECT id FROM orphan_pve_server_ids)
    )
);

DELETE FROM pve_storages
WHERE report_id IN (
    SELECT id FROM pve_reports WHERE server_id IN (SELECT id FROM orphan_pve_server_ids)
);

DELETE FROM pve_reports
WHERE server_id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM pve_servers
WHERE id IN (SELECT id FROM orphan_pve_server_ids);

DELETE FROM alert_suppressions
WHERE EXISTS (
    SELECT 1 FROM orphan_pbs_server_ids o
    WHERE alert_id LIKE '%:pbs:' || o.id
       OR alert_id LIKE '%:pbs:' || o.id || ':%'
);

DELETE FROM server_heartbeats
WHERE server_type = 'pbs'
  AND server_id IN (SELECT id FROM orphan_pbs_server_ids);

DELETE FROM pbs_alert_config
WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids);

DELETE FROM pbs_snapshots
WHERE store_id IN (
    SELECT id
    FROM pbs_stores
    WHERE report_id IN (
        SELECT id FROM pbs_reports WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids)
    )
);

DELETE FROM pbs_store_history
WHERE store_id IN (
    SELECT id
    FROM pbs_stores
    WHERE report_id IN (
        SELECT id FROM pbs_reports WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids)
    )
);

DELETE FROM pbs_gc_status
WHERE store_id IN (
    SELECT id
    FROM pbs_stores
    WHERE report_id IN (
        SELECT id FROM pbs_reports WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids)
    )
);

DELETE FROM pbs_stores
WHERE report_id IN (
    SELECT id FROM pbs_reports WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids)
);

DELETE FROM pbs_reports
WHERE server_id IN (SELECT id FROM orphan_pbs_server_ids);

DELETE FROM pbs_servers
WHERE id IN (SELECT id FROM orphan_pbs_server_ids);

DROP TABLE orphan_pve_server_ids;
DROP TABLE orphan_pbs_server_ids;

PRAGMA foreign_keys=on;

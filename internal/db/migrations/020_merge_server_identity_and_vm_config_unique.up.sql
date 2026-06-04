PRAGMA foreign_keys=off;

CREATE TEMP TABLE merge_pve_servers AS
SELECT old.id AS old_id, new.id AS new_id, new.api_key_id AS api_key_id
FROM pve_servers old
JOIN pve_servers new
  ON old.name = new.name
 AND old.machine_id = new.machine_id
WHERE old.api_key_id IS NULL
  AND new.api_key_id IS NOT NULL
  AND old.machine_id IS NOT NULL
  AND old.machine_id <> ''
  AND old.is_deleted = 0
  AND new.is_deleted = 0;

UPDATE pve_servers
SET api_key_id = NULL
WHERE id IN (SELECT new_id FROM merge_pve_servers);

DELETE FROM server_heartbeats
WHERE server_type = 'pve'
  AND server_id IN (SELECT old_id FROM merge_pve_servers);

UPDATE server_heartbeats
SET server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = server_heartbeats.server_id)
WHERE server_type = 'pve'
  AND server_id IN (SELECT new_id FROM merge_pve_servers);

UPDATE pve_reports
SET server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = pve_reports.server_id)
WHERE server_id IN (SELECT new_id FROM merge_pve_servers);

UPDATE pve_alert_config
SET server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = pve_alert_config.server_id)
WHERE server_id IN (SELECT new_id FROM merge_pve_servers)
  AND NOT EXISTS (
      SELECT 1 FROM pve_alert_config existing
      WHERE existing.server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = pve_alert_config.server_id)
  );

DELETE FROM pve_alert_config
WHERE server_id IN (SELECT new_id FROM merge_pve_servers);

UPDATE pve_vm_alert_config
SET server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = pve_vm_alert_config.server_id)
WHERE server_id IN (SELECT new_id FROM merge_pve_servers)
  AND NOT EXISTS (
      SELECT 1 FROM pve_vm_alert_config existing
      WHERE existing.server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = pve_vm_alert_config.server_id)
        AND existing.vmid = pve_vm_alert_config.vmid
  );

DELETE FROM pve_vm_alert_config
WHERE server_id IN (SELECT new_id FROM merge_pve_servers);

UPDATE vm_backup_configs
SET server_id = (SELECT old_id FROM merge_pve_servers WHERE new_id = vm_backup_configs.server_id)
WHERE server_type = 'pve'
  AND server_id IN (SELECT new_id FROM merge_pve_servers);

DELETE FROM alert_suppressions
WHERE EXISTS (
    SELECT 1 FROM merge_pve_servers m
    WHERE alert_id LIKE '%:pve:' || m.new_id
       OR alert_id LIKE '%:pve:' || m.new_id || ':%'
);

UPDATE pve_servers
SET api_key_id = (SELECT api_key_id FROM merge_pve_servers WHERE old_id = pve_servers.id),
    ip = COALESCE((SELECT ip FROM pve_servers n JOIN merge_pve_servers m ON m.new_id = n.id WHERE m.old_id = pve_servers.id), ip),
    public_ip = COALESCE((SELECT public_ip FROM pve_servers n JOIN merge_pve_servers m ON m.new_id = n.id WHERE m.old_id = pve_servers.id), public_ip),
    client_version = COALESCE((SELECT client_version FROM pve_servers n JOIN merge_pve_servers m ON m.new_id = n.id WHERE m.old_id = pve_servers.id), client_version),
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT old_id FROM merge_pve_servers);

DELETE FROM pve_servers
WHERE id IN (SELECT new_id FROM merge_pve_servers);

CREATE TEMP TABLE merge_pbs_servers AS
SELECT old.id AS old_id, new.id AS new_id, new.api_key_id AS api_key_id
FROM pbs_servers old
JOIN pbs_servers new
  ON old.name = new.name
 AND old.machine_id = new.machine_id
WHERE old.api_key_id IS NULL
  AND new.api_key_id IS NOT NULL
  AND old.machine_id IS NOT NULL
  AND old.machine_id <> ''
  AND old.is_deleted = 0
  AND new.is_deleted = 0;

UPDATE pbs_servers
SET api_key_id = NULL
WHERE id IN (SELECT new_id FROM merge_pbs_servers);

DELETE FROM server_heartbeats
WHERE server_type = 'pbs'
  AND server_id IN (SELECT old_id FROM merge_pbs_servers);

UPDATE server_heartbeats
SET server_id = (SELECT old_id FROM merge_pbs_servers WHERE new_id = server_heartbeats.server_id)
WHERE server_type = 'pbs'
  AND server_id IN (SELECT new_id FROM merge_pbs_servers);

UPDATE pbs_reports
SET server_id = (SELECT old_id FROM merge_pbs_servers WHERE new_id = pbs_reports.server_id)
WHERE server_id IN (SELECT new_id FROM merge_pbs_servers);

UPDATE pbs_alert_config
SET server_id = (SELECT old_id FROM merge_pbs_servers WHERE new_id = pbs_alert_config.server_id)
WHERE server_id IN (SELECT new_id FROM merge_pbs_servers)
  AND NOT EXISTS (
      SELECT 1 FROM pbs_alert_config existing
      WHERE existing.server_id = (SELECT old_id FROM merge_pbs_servers WHERE new_id = pbs_alert_config.server_id)
  );

DELETE FROM pbs_alert_config
WHERE server_id IN (SELECT new_id FROM merge_pbs_servers);

DELETE FROM alert_suppressions
WHERE EXISTS (
    SELECT 1 FROM merge_pbs_servers m
    WHERE alert_id LIKE '%:pbs:' || m.new_id
       OR alert_id LIKE '%:pbs:' || m.new_id || ':%'
);

UPDATE pbs_servers
SET api_key_id = (SELECT api_key_id FROM merge_pbs_servers WHERE old_id = pbs_servers.id),
    ip = COALESCE((SELECT ip FROM pbs_servers n JOIN merge_pbs_servers m ON m.new_id = n.id WHERE m.old_id = pbs_servers.id), ip),
    public_ip = COALESCE((SELECT public_ip FROM pbs_servers n JOIN merge_pbs_servers m ON m.new_id = n.id WHERE m.old_id = pbs_servers.id), public_ip),
    client_version = COALESCE((SELECT client_version FROM pbs_servers n JOIN merge_pbs_servers m ON m.new_id = n.id WHERE m.old_id = pbs_servers.id), client_version),
    updated_at = CURRENT_TIMESTAMP
WHERE id IN (SELECT old_id FROM merge_pbs_servers);

DELETE FROM pbs_servers
WHERE id IN (SELECT new_id FROM merge_pbs_servers);

DROP INDEX IF EXISTS idx_vm_backup_configs_server_vmid;

ALTER TABLE vm_backup_configs RENAME TO vm_backup_configs_old;

CREATE TABLE vm_backup_configs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    server_name TEXT    NOT NULL,
    vm_id       TEXT    NOT NULL,
    vm_name     TEXT,
    monday      INTEGER NOT NULL DEFAULT 0,
    tuesday     INTEGER NOT NULL DEFAULT 0,
    wednesday   INTEGER NOT NULL DEFAULT 0,
    thursday    INTEGER NOT NULL DEFAULT 0,
    friday      INTEGER NOT NULL DEFAULT 0,
    saturday    INTEGER NOT NULL DEFAULT 0,
    sunday      INTEGER NOT NULL DEFAULT 0,
    is_excluded INTEGER NOT NULL DEFAULT 0,
    is_deleted  INTEGER NOT NULL DEFAULT 0,
    deleted_at  DATETIME,
    created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    server_type TEXT NOT NULL DEFAULT 'pve',
    server_id   INTEGER NOT NULL DEFAULT 0
);

INSERT INTO vm_backup_configs (
    id, server_name, vm_id, vm_name, monday, tuesday, wednesday, thursday,
    friday, saturday, sunday, is_excluded, is_deleted, deleted_at, created_at,
    server_type, server_id
)
SELECT id, server_name, vm_id, vm_name, monday, tuesday, wednesday, thursday,
       friday, saturday, sunday, is_excluded, is_deleted, deleted_at, created_at,
       server_type, server_id
FROM vm_backup_configs_old;

DROP TABLE vm_backup_configs_old;

CREATE UNIQUE INDEX IF NOT EXISTS idx_vm_backup_configs_server_vmid
ON vm_backup_configs(server_type, server_id, vm_id)
WHERE server_id > 0 AND is_deleted = 0;

DROP TABLE merge_pve_servers;
DROP TABLE merge_pbs_servers;

PRAGMA foreign_keys=on;

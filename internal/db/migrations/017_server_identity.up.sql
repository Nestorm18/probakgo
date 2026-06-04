PRAGMA foreign_keys=off;

CREATE TABLE pve_servers_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    ip         TEXT,
    public_ip  TEXT,
    client_version TEXT,
    machine_id TEXT,
    api_key_id INTEGER,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO pve_servers_new (id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at)
SELECT id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at FROM pve_servers;

DROP TABLE pve_servers;
ALTER TABLE pve_servers_new RENAME TO pve_servers;

CREATE INDEX IF NOT EXISTS idx_pve_servers_name ON pve_servers(name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pve_servers_api_key ON pve_servers(api_key_id) WHERE api_key_id IS NOT NULL;

CREATE TABLE pbs_servers_new (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    name       TEXT    NOT NULL,
    ip         TEXT,
    public_ip  TEXT,
    client_version TEXT,
    machine_id TEXT,
    api_key_id INTEGER,
    is_deleted INTEGER NOT NULL DEFAULT 0,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

INSERT INTO pbs_servers_new (id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at)
SELECT id, name, ip, public_ip, client_version, machine_id, is_deleted, created_at, updated_at FROM pbs_servers;

DROP TABLE pbs_servers;
ALTER TABLE pbs_servers_new RENAME TO pbs_servers;

CREATE INDEX IF NOT EXISTS idx_pbs_servers_name ON pbs_servers(name);
CREATE UNIQUE INDEX IF NOT EXISTS idx_pbs_servers_api_key ON pbs_servers(api_key_id) WHERE api_key_id IS NOT NULL;

ALTER TABLE vm_backup_configs ADD COLUMN server_type TEXT NOT NULL DEFAULT 'pve';
ALTER TABLE vm_backup_configs ADD COLUMN server_id INTEGER NOT NULL DEFAULT 0;

UPDATE vm_backup_configs
SET server_id = COALESCE((SELECT id FROM pve_servers WHERE pve_servers.name = vm_backup_configs.server_name LIMIT 1), 0)
WHERE server_id = 0;

CREATE UNIQUE INDEX IF NOT EXISTS idx_vm_backup_configs_server_vmid
ON vm_backup_configs(server_type, server_id, vm_id)
WHERE server_id > 0;

PRAGMA foreign_keys=on;

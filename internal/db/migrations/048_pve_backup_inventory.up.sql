ALTER TABLE pve_servers ADD COLUMN backup_inventory_known INTEGER NOT NULL DEFAULT 0;
ALTER TABLE pve_servers ADD COLUMN has_backup_vms INTEGER NOT NULL DEFAULT 0;

CREATE INDEX IF NOT EXISTS idx_pve_storages_report
ON pve_storages(report_id);

CREATE INDEX IF NOT EXISTS idx_pve_storage_info_storage
ON pve_storage_info(storage_id);

CREATE INDEX IF NOT EXISTS idx_pve_storage_content_storage_ctime
ON pve_storage_content(storage_id, ctime DESC);

CREATE INDEX IF NOT EXISTS idx_pve_backup_tasks_report_vmid
ON pve_backup_tasks(report_id, vmid);

CREATE INDEX IF NOT EXISTS idx_pbs_stores_report
ON pbs_stores(report_id);

CREATE INDEX IF NOT EXISTS idx_pbs_store_history_store_position
ON pbs_store_history(store_id, position);

CREATE INDEX IF NOT EXISTS idx_pbs_gc_status_store
ON pbs_gc_status(store_id);

CREATE INDEX IF NOT EXISTS idx_alert_states_present
ON alert_states(is_present, alert_id);

CREATE INDEX IF NOT EXISTS idx_vm_backup_configs_legacy_active
ON vm_backup_configs(server_name, vm_id)
WHERE server_id = 0 AND is_deleted = 0;

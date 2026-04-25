package domain

import "time"

type PVEServer struct {
	ID            int64      `db:"id"`
	Name          string     `db:"name"`
	IP            string     `db:"ip"`
	PublicIP      string     `db:"public_ip"`
	ClientVersion string     `db:"client_version"`
	MachineID     string     `db:"machine_id"`
	IsDeleted     bool       `db:"is_deleted"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

type PVEReport struct {
	ID              int64     `db:"id"`
	ServerID        int64     `db:"server_id"`
	ReportedAt      time.Time `db:"reported_at"`
	IsStale         bool      `db:"is_stale"`
	StaleReason     string    `db:"stale_reason"`
	BackupStatus    string    `db:"backup_status"`
	BackupStarttime int64     `db:"backup_starttime"`
	BackupEndtime   int64     `db:"backup_endtime"`
	BackupDuration  int64     `db:"backup_duration"`
}

type PVEStorage struct {
	ID           int64  `db:"id"`
	ReportID     int64  `db:"report_id"`
	Storage      string `db:"storage"`
	Path         string `db:"path"`
	Content      string `db:"content"`
	Type         string `db:"type"`
	Status       string `db:"status"`
	Shared       bool   `db:"shared"`
	Server       string `db:"server"`
	Digest       string `db:"digest"`
	PruneBackups string `db:"prune_backups"`
}

type PVEStorageInfo struct {
	ID         int64   `db:"id"`
	StorageID  int64   `db:"storage_id"`
	Total      int64   `db:"total"`
	Used       int64   `db:"used"`
	Avail      int64   `db:"avail"`
	UsedPct    float64 `db:"used_percent"`
	Active     bool    `db:"active"`
	Enabled    bool    `db:"enabled"`
	Lvl        int     `db:"lvl"`
}

type PVEStorageContent struct {
	ID        int64  `db:"id"`
	StorageID int64  `db:"storage_id"`
	VMID      int64  `db:"vmid"`
	Format    string `db:"format"`
	Size      int64  `db:"size"`
	Content   string `db:"content"`
	VolID     string `db:"volid"`
	CTime     int64  `db:"ctime"`
	Subtype   string `db:"subtype"`
	Notes     string `db:"notes"`
}

type PBSServer struct {
	ID            int64      `db:"id"`
	Name          string     `db:"name"`
	IP            string     `db:"ip"`
	PublicIP      string     `db:"public_ip"`
	ClientVersion string     `db:"client_version"`
	MachineID     string     `db:"machine_id"`
	IsDeleted     bool       `db:"is_deleted"`
	CreatedAt     time.Time  `db:"created_at"`
	UpdatedAt     time.Time  `db:"updated_at"`
}

type PBSReport struct {
	ID          int64     `db:"id"`
	ServerID    int64     `db:"server_id"`
	ReportedAt  time.Time `db:"reported_at"`
	IsStale     bool      `db:"is_stale"`
	StaleReason string    `db:"stale_reason"`
}

type PBSStore struct {
	ID                int64  `db:"id"`
	ReportID          int64  `db:"report_id"`
	Store             string `db:"store"`
	Total             int64  `db:"total"`
	Used              int64  `db:"used"`
	Avail             int64  `db:"avail"`
	EstimatedFullDate int64  `db:"estimated_full_date"`
	MountStatus       string `db:"mount_status"`
	HistoryStart      int64  `db:"history_start"`
	HistoryDelta      int64  `db:"history_delta"`
}

type PBSStoreHistory struct {
	ID       int64    `db:"id"`
	StoreID  int64    `db:"store_id"`
	Position int      `db:"position"`
	Value    *float64 `db:"value"`
}

type PBSGCStatus struct {
	ID              int64  `db:"id"`
	StoreID         int64  `db:"store_id"`
	DiskBytes       int64  `db:"disk_bytes"`
	DiskChunks      int64  `db:"disk_chunks"`
	IndexDataBytes  int64  `db:"index_data_bytes"`
	IndexFileCount  int64  `db:"index_file_count"`
	PendingBytes    int64  `db:"pending_bytes"`
	PendingChunks   int64  `db:"pending_chunks"`
	RemovedBad      int64  `db:"removed_bad"`
	RemovedBytes    int64  `db:"removed_bytes"`
	RemovedChunks   int64  `db:"removed_chunks"`
	StillBad        int64  `db:"still_bad"`
	UPID            string `db:"upid"`
}

type APIKey struct {
	ID         int64      `db:"id"`
	Key        string     `db:"key"`
	Name       string     `db:"name"`
	KeyType    string     `db:"key_type"`
	IsActive   bool       `db:"is_active"`
	MachineID  string     `db:"machine_id"`
	LastUsed   *time.Time `db:"last_used"`
	ServerName string     `db:"server_name"`
	CreatedAt  time.Time  `db:"created_at"`
}

type User struct {
	ID           int64     `db:"id"`
	Username     string    `db:"username"`
	PasswordHash string    `db:"password_hash"`
	Role         string    `db:"role"`
	IsActive     bool      `db:"is_active"`
	CreatedAt    time.Time `db:"created_at"`
}

type VMBackupConfig struct {
	ID         int64      `db:"id"`
	ServerName string     `db:"server_name"`
	VMID       string     `db:"vm_id"`
	VMName     string     `db:"vm_name"`
	Monday     bool       `db:"monday"`
	Tuesday    bool       `db:"tuesday"`
	Wednesday  bool       `db:"wednesday"`
	Thursday   bool       `db:"thursday"`
	Friday     bool       `db:"friday"`
	Saturday   bool       `db:"saturday"`
	Sunday     bool       `db:"sunday"`
	IsExcluded bool       `db:"is_excluded"`
	IsDeleted  bool       `db:"is_deleted"`
	DeletedAt  *time.Time `db:"deleted_at"`
	CreatedAt  time.Time  `db:"created_at"`
}

type EmailConfig struct {
	ID         int64  `db:"id"`
	SMTPHost   string `db:"smtp_host"`
	SMTPPort   int    `db:"smtp_port"`
	SMTPUser   string `db:"smtp_user"`
	SMTPPass   string `db:"smtp_password"`
	Recipients string `db:"recipients"` // JSON array
	IsEnabled  bool   `db:"is_enabled"`
	SendTime   string `db:"send_time"`
}

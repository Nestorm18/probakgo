package domain

import "time"

type PVEServer struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	DisplayName   string    `db:"display_name"`
	IP            string    `db:"ip"`
	PublicIP      string    `db:"public_ip"`
	ClientVersion string    `db:"client_version"`
	MachineID     string    `db:"machine_id"`
	APIKeyID      int64     `db:"api_key_id"`
	IsDeleted     bool      `db:"is_deleted"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
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
	SwapTotal       int64     `db:"swap_total"`
	SwapUsed        int64     `db:"swap_used"`
	SwapEnabled     bool      `db:"swap_enabled"`
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
	ID        int64   `db:"id"`
	StorageID int64   `db:"storage_id"`
	Total     int64   `db:"total"`
	Used      int64   `db:"used"`
	Avail     int64   `db:"avail"`
	UsedPct   float64 `db:"used_percent"`
	Active    bool    `db:"active"`
	Enabled   bool    `db:"enabled"`
	Lvl       int     `db:"lvl"`
}

type PVEStorageContent struct {
	ID           int64  `db:"id"`
	StorageID    int64  `db:"storage_id"`
	VMID         int64  `db:"vmid"`
	Format       string `db:"format"`
	Size         int64  `db:"size"`
	Content      string `db:"content"`
	VolID        string `db:"volid"`
	CTime        int64  `db:"ctime"`
	Subtype      string `db:"subtype"`
	Notes        string `db:"notes"`
	Verification string `db:"verification"`
}

type PBSServer struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	DisplayName   string    `db:"display_name"`
	IP            string    `db:"ip"`
	PublicIP      string    `db:"public_ip"`
	ClientVersion string    `db:"client_version"`
	MachineID     string    `db:"machine_id"`
	APIKeyID      int64     `db:"api_key_id"`
	IsDeleted     bool      `db:"is_deleted"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type PBSReport struct {
	ID          int64     `db:"id"`
	ServerID    int64     `db:"server_id"`
	ReportedAt  time.Time `db:"reported_at"`
	IsStale     bool      `db:"is_stale"`
	StaleReason string    `db:"stale_reason"`
	SwapTotal   int64     `db:"swap_total"`
	SwapUsed    int64     `db:"swap_used"`
	SwapEnabled bool      `db:"swap_enabled"`
}

type HostSwap struct {
	Total   int64
	Used    int64
	Enabled bool
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

type PBSSnapshot struct {
	ID                int64  `db:"id"`
	StoreID           int64  `db:"store_id"`
	BackupType        string `db:"backup_type"`
	BackupID          string `db:"backup_id"`
	LastBackup        int64  `db:"last_backup"`
	BackupCount       int64  `db:"backup_count"`
	Owner             string `db:"owner"`
	Comment           string `db:"comment"`
	VerificationState string `db:"verification_state"`
	Size              int64  `db:"size"`
}

type PBSGCStatus struct {
	ID             int64  `db:"id"`
	StoreID        int64  `db:"store_id"`
	DiskBytes      int64  `db:"disk_bytes"`
	DiskChunks     int64  `db:"disk_chunks"`
	IndexDataBytes int64  `db:"index_data_bytes"`
	IndexFileCount int64  `db:"index_file_count"`
	PendingBytes   int64  `db:"pending_bytes"`
	PendingChunks  int64  `db:"pending_chunks"`
	RemovedBad     int64  `db:"removed_bad"`
	RemovedBytes   int64  `db:"removed_bytes"`
	RemovedChunks  int64  `db:"removed_chunks"`
	StillBad       int64  `db:"still_bad"`
	UPID           string `db:"upid"`
}

type PVEBackupTask struct {
	ID        int64  `db:"id"`
	ReportID  int64  `db:"report_id"`
	VMID      int64  `db:"vmid"`
	VMName    string `db:"vm_name"`
	Status    string `db:"status"`
	StartTime int64  `db:"starttime"`
	EndTime   int64  `db:"endtime"`
	Duration  int64  `db:"duration"`
	Size      int64  `db:"size"`
	Filename  string `db:"filename"`
}

type WindowsServer struct {
	ID            int64     `db:"id"`
	Name          string    `db:"name"`
	DisplayName   string    `db:"display_name"`
	IP            string    `db:"ip"`
	PublicIP      string    `db:"public_ip"`
	ClientVersion string    `db:"client_version"`
	MachineID     string    `db:"machine_id"`
	APIKeyID      int64     `db:"api_key_id"`
	IsDeleted     bool      `db:"is_deleted"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

type WindowsReport struct {
	ID         int64     `db:"id"`
	ServerID   int64     `db:"server_id"`
	ReportedAt time.Time `db:"reported_at"`
	IsStale    bool      `db:"is_stale"`
}

type WindowsDisk struct {
	ID         int64  `db:"id"`
	ReportID   int64  `db:"report_id"`
	Name       string `db:"name"`
	Label      string `db:"label"`
	FileSystem string `db:"file_system"`
	DriveType  string `db:"drive_type"`
	Total      int64  `db:"total"`
	Used       int64  `db:"used"`
	Free       int64  `db:"free"`
	Health     string `db:"health"`
}

type ServerHeartbeat struct {
	ID            int64     `db:"id"`
	ServerType    string    `db:"server_type"`
	ServerID      int64     `db:"server_id"`
	Hostname      string    `db:"hostname"`
	IP            string    `db:"ip"`
	PublicIP      string    `db:"public_ip"`
	ClientVersion string    `db:"client_version"`
	MachineID     string    `db:"machine_id"`
	LastSeenAt    time.Time `db:"last_seen_at"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
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
	ServerURL  string     `db:"server_url"`
	CreatedAt  time.Time  `db:"created_at"`
}

type User struct {
	ID                 int64      `db:"id"`
	Username           string     `db:"username"`
	PasswordHash       string     `db:"password_hash" json:"-"`
	Role               string     `db:"role"`
	IsActive           bool       `db:"is_active"`
	CreatedAt          time.Time  `db:"created_at"`
	LastLoginAt        *time.Time `db:"last_login_at"`
	LastLoginIP        string     `db:"last_login_ip"`
	TOTPEnabled        bool       `db:"totp_enabled"`
	TOTPSecret         string     `db:"totp_secret" json:"-"`
	TOTPConfirmedAt    *time.Time `db:"totp_confirmed_at"`
	TOTPGraceStartedAt *time.Time `db:"totp_grace_started_at"`
}

type LoginAttempt struct {
	ID          int64     `db:"id"`
	Username    string    `db:"username"`
	IP          string    `db:"ip"`
	UserAgent   string    `db:"user_agent"`
	Result      string    `db:"result"`
	Reason      string    `db:"reason"`
	AttemptedAt time.Time `db:"attempted_at"`
}

type AuditLog struct {
	ID            int64     `db:"id"`
	ActorUsername string    `db:"actor_username"`
	ActorRole     string    `db:"actor_role"`
	ActorIP       string    `db:"actor_ip"`
	Action        string    `db:"action"`
	TargetType    string    `db:"target_type"`
	TargetID      string    `db:"target_id"`
	TargetName    string    `db:"target_name"`
	Metadata      string    `db:"metadata"`
	CreatedAt     time.Time `db:"created_at"`
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
	ID                          int64  `db:"id"`
	SMTPHost                    string `db:"smtp_host"`
	SMTPPort                    int    `db:"smtp_port"`
	SMTPUser                    string `db:"smtp_user"`
	SMTPPass                    string `db:"smtp_password"`
	Recipients                  string `db:"recipients"`
	IsEnabled                   bool   `db:"is_enabled"`
	SendTime                    string `db:"send_time"`
	RetentionMonths             int    `db:"retention_months"`
	RetentionEnabled            bool   `db:"retention_enabled"`
	AlertDiskPct                int    `db:"alert_disk_pct"`         // 0 = disabled
	AlertWindowsDiskPct         int    `db:"alert_windows_disk_pct"` // 0 = disabled
	AlertBackupErr              bool   `db:"alert_backup_err"`
	AlertPBSStaleHours          int    `db:"alert_pbs_stale_hours"` // 0 = disabled
	PublicAPIURL                string `db:"public_api_url"`
	AlertPVEHeartbeatMinutes    int    `db:"alert_pve_heartbeat_minutes"` // 0 = disabled
	CriticalAlertsEnabled       bool   `db:"critical_alerts_enabled"`
	EnforceTOTPNonReaders       bool   `db:"enforce_totp_non_readers"`
	SensitiveActionsRequireTOTP bool   `db:"sensitive_actions_require_totp"`
}

// Alert represents a detected condition requiring attention.
type Alert struct {
	ID         string // dedup key: "type:serverType:serverID:store:vmid"
	ServerName string
	ServerID   int64
	ServerType string // "pve" | "pbs" | "windows"
	StoreName  string // empty if server-level
	VMID       int64  // 0 if not applicable
	VMName     string
	Type       string // AlertType* constant
	Severity   string // AlertSeverity* constant
	Title      string
	Message    string
	Value      string // measured value, e.g. "87%"
	Threshold  string // configured threshold, e.g. "85%"
	DetectedAt time.Time
}

type AlertStateEvent struct {
	ID         int64     `db:"id"`
	AlertID    string    `db:"alert_id"`
	EventType  string    `db:"event_type"`
	Severity   string    `db:"severity"`
	Title      string    `db:"title"`
	Message    string    `db:"message"`
	ServerName string    `db:"server_name"`
	ServerType string    `db:"server_type"`
	ServerID   int64     `db:"server_id"`
	StoreName  string    `db:"store_name"`
	VMID       int64     `db:"vmid"`
	VMName     string    `db:"vm_name"`
	Note       string    `db:"note"`
	CreatedAt  time.Time `db:"created_at"`
}

const (
	AlertTypeDisk              = "disk"
	AlertTypeBackupError       = "backup_error"
	AlertTypeBackupSize        = "backup_size"
	AlertTypePBSFill           = "pbs_fill"
	AlertTypePBSStale          = "pbs_stale"
	AlertTypePBSVerify         = "pbs_verify"
	AlertTypePVEStale          = "pve_stale"
	AlertTypePVEHeartbeat      = "pve_heartbeat"
	AlertTypePBSReportStale    = "pbs_report_stale"
	AlertTypePVEMissingVM      = "pve_missing_vm"
	AlertTypePVEUnknownVM      = "pve_unknown_vm"
	AlertTypeSwap              = "swap"
	AlertTypeWindowsHeartbeat  = "windows_heartbeat"
	AlertTypeWindowsDiskHealth = "windows_disk_health"
	AlertTypeWindowsVolumeGone = "windows_volume_missing"
)

const (
	AlertSeverityCritical = "critical"
	AlertSeverityWarning  = "warning"
)

// PVEAlertConfig holds per-server alert thresholds for a PVE server.
// nil fields inherit from the global email_config values.
type PVEAlertConfig struct {
	ServerID           int64
	DiskPct            *int    // nil = use global; storage usage threshold %
	StaleHours         *int    // nil = use global; 0 = disabled
	BackupErr          *int    // nil = use global; 0 = ignore; 1 = alert
	ExpectedFinishTime *string // nil = 09:00; HH:MM cutoff for previous night's report
}

// PVEVMAlertConfig holds per-VM overrides within a PVE server.
// nil fields inherit from the server-level PVEAlertConfig.
type PVEVMAlertConfig struct {
	ID        int64
	ServerID  int64
	VMID      int64
	BackupErr *int // nil = inherit; 0 = ignore; 1 = alert
	MinSizeMB *int // nil = no size check; alert if backup < N MB
}

// PBSAlertConfig holds per-server alert thresholds for a PBS server.
// nil fields inherit from the global email_config values.
type PBSAlertConfig struct {
	ServerID      int64
	DiskPct       *int // nil = use global
	DaysUntilFull *int // nil = disabled; alert if disk fills in < N days
	StaleHours    *int // nil = use global; 0 = disabled
	VerifyAlert   bool
}

// WindowsAlertConfig holds per-server Windows alert thresholds.
// nil fields inherit from Windows-specific global email_config values.
type WindowsAlertConfig struct {
	ServerID int64
	DiskPct  *int // nil = use global Windows disk threshold
}

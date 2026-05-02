package domain

import (
	"encoding/json"
	"time"
)

// --- PVE report payload (sent by client) ---

type PVEReportRequest struct {
	Hostname         string               `json:"hostname"`
	IPAddress        string               `json:"ip_address"`
	PublicIP         string               `json:"public_ip"`
	ClientVersion    string               `json:"client_version"`
	MachineID        string               `json:"machine_id"`
	LastBackupStatus *BackupStatus        `json:"last_backup_status"`
	Storages         []StoragePayload     `json:"storages"`
	BackupTasks      []BackupTaskPayload  `json:"backup_tasks"`
}

type BackupTaskPayload struct {
	VMID      int64  `json:"vmid"`
	VMName    string `json:"vm_name"`
	Status    string `json:"status"`
	StartTime int64  `json:"starttime"`
	EndTime   int64  `json:"endtime"`
	Duration  int64  `json:"duration"`
	Size      int64  `json:"size"`
	Filename  string `json:"filename"`
}

// BackupStatus.Status can arrive as bool or string from the client.
type BackupStatus struct {
	Status    json.RawMessage `json:"status"`
	StartTime int64           `json:"starttime"`
	EndTime   int64           `json:"endtime"`
	Duration  int64           `json:"duration"`
}

// StatusString normalises the raw JSON value to a human-readable string.
func (b *BackupStatus) StatusString() string {
	if b == nil {
		return ""
	}
	var s string
	if err := json.Unmarshal(b.Status, &s); err == nil {
		return s
	}
	var v bool
	if err := json.Unmarshal(b.Status, &v); err == nil {
		if v {
			return "OK"
		}
		return "ERROR"
	}
	return string(b.Status)
}

type StoragePayload struct {
	Digest       string            `json:"digest"`
	PruneBackups json.RawMessage   `json:"prune_backups"`
	Shared       bool              `json:"shared"`
	Server       string            `json:"server"`
	Storage      string            `json:"storage"`
	Path         string            `json:"path"`
	Content      string            `json:"content"`
	Type         string            `json:"type"`
	Status       string            `json:"status"`
	StorageInfo  []StorageInfoPayload  `json:"storage_info"`
	ContentData  []ContentDataPayload  `json:"content_data"`
}

type StorageInfoPayload struct {
	Total   int64   `json:"total"`
	Used    int64   `json:"used"`
	Avail   int64   `json:"avail"`
	UsedPct float64 `json:"used_percent"`
	Active  bool    `json:"active"`
	Enabled bool    `json:"enabled"`
	Lvl     int     `json:"lvl"`
}

type ContentDataPayload struct {
	VMID         int64  `json:"vmid"`
	Format       string `json:"format"`
	Size         int64  `json:"size"`
	Content      string `json:"content"`
	VolID        string `json:"volid"`
	CTime        int64  `json:"ctime"`
	Subtype      string `json:"subtype"`
	Notes        string `json:"notes"`
	Verification string `json:"verification"`
}

// --- PBS report payload (sent by client) ---

type PBSReportRequest struct {
	Hostname       string         `json:"hostname"`
	IPAddress      string         `json:"ip_address"`
	PublicIP       string         `json:"public_ip"`
	ClientVersion  string         `json:"client_version"`
	MachineID      string         `json:"machine_id"`
	PBSInformation PBSInformation `json:"pbs_information"`
}

type PBSInformation struct {
	Data []PBSDatastorePayload `json:"data"`
}

type PBSDatastorePayload struct {
	Store             string     `json:"store"`
	Total             int64      `json:"total"`
	Used              int64      `json:"used"`
	Avail             int64      `json:"avail"`
	EstimatedFullDate int64      `json:"estimated-full-date"`
	MountStatus       string     `json:"mount-status"`
	HistoryStart      int64      `json:"history-start"`
	HistoryDelta      int64      `json:"history-delta"`
	History           []*float64 `json:"history"`
	GCStatus          *GCStatusPayload `json:"gc-status"`
}

type GCStatusPayload struct {
	DiskBytes      int64  `json:"disk-bytes"`
	DiskChunks     int64  `json:"disk-chunks"`
	IndexDataBytes int64  `json:"index-data-bytes"`
	IndexFileCount int64  `json:"index-file-count"`
	PendingBytes   int64  `json:"pending-bytes"`
	PendingChunks  int64  `json:"pending-chunks"`
	RemovedBad     int64  `json:"removed-bad"`
	RemovedBytes   int64  `json:"removed-bytes"`
	RemovedChunks  int64  `json:"removed-chunks"`
	StillBad       int64  `json:"still-bad"`
	UPID           string `json:"upid"`
}

// --- API responses ---

type PVEServerResponse struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	PublicIP      string    `json:"public_ip"`
	ClientVersion string    `json:"client_version"`
	MachineBound  bool      `json:"machine_bound"`
	IsStale       bool      `json:"is_stale"`
	StaleReason   string    `json:"stale_reason,omitempty"`
	LastReport    *time.Time `json:"last_report"`
	BackupStatus  string    `json:"backup_status"`
}

type PBSServerResponse struct {
	ID            int64     `json:"id"`
	Name          string    `json:"name"`
	IP            string    `json:"ip"`
	PublicIP      string    `json:"public_ip"`
	ClientVersion string    `json:"client_version"`
	MachineBound  bool      `json:"machine_bound"`
	IsStale       bool      `json:"is_stale"`
	StaleReason   string    `json:"stale_reason,omitempty"`
	LastReport    *time.Time `json:"last_report"`
}

type APIKeyResponse struct {
	ID         int64      `json:"id"`
	Name       string     `json:"name"`
	KeyPreview string     `json:"key_preview"`
	KeyType    string     `json:"key_type"`
	IsActive   bool       `json:"is_active"`
	MachineID  string     `json:"machine_id,omitempty"`
	LastUsed   *time.Time `json:"last_used"`
	ServerName string     `json:"server_name,omitempty"`
	CreatedAt  time.Time  `json:"created_at"`
}

type CreateAPIKeyRequest struct {
	Name       string `json:"name"`
	KeyType    string `json:"key_type"`
	ServerName string `json:"server_name,omitempty"`
}

type CreateAPIKeyResponse struct {
	ID  int64  `json:"id"`
	Key string `json:"key"`
	Name string `json:"name"`
}

type UpdateAPIKeyRequest struct {
	Name       string `json:"name,omitempty"`
	ServerName string `json:"server_name,omitempty"`
}

type VMBackupConfigResponse struct {
	ID         int64  `json:"id"`
	ServerName string `json:"server_name"`
	VMID       string `json:"vm_id"`
	VMName     string `json:"vm_name"`
	Monday     bool   `json:"monday"`
	Tuesday    bool   `json:"tuesday"`
	Wednesday  bool   `json:"wednesday"`
	Thursday   bool   `json:"thursday"`
	Friday     bool   `json:"friday"`
	Saturday   bool   `json:"saturday"`
	Sunday     bool   `json:"sunday"`
	IsExcluded bool   `json:"is_excluded"`
}

type CreateVMBackupConfigRequest struct {
	VMID      string `json:"vm_id"`
	VMName    string `json:"vm_name"`
	Monday    bool   `json:"monday"`
	Tuesday   bool   `json:"tuesday"`
	Wednesday bool   `json:"wednesday"`
	Thursday  bool   `json:"thursday"`
	Friday    bool   `json:"friday"`
	Saturday  bool   `json:"saturday"`
	Sunday    bool   `json:"sunday"`
}

package domain

import (
	"fmt"
	"strings"
)

const (
	maxHostnameLength = 255
	maxReportIDLength = 128
	maxMetadataLength = 512
)

func (r *PVEReportRequest) Validate() error {
	if err := validateReportIdentity(r.ReportID, r.Hostname, r.IPAddress, r.PublicIP, r.ClientVersion, r.MachineID); err != nil {
		return err
	}
	if r.SwapTotal < 0 || r.SwapUsed < 0 {
		return fmt.Errorf("swap values cannot be negative")
	}
	if len(r.Storages) > 256 {
		return fmt.Errorf("too many storages")
	}
	if len(r.BackupTasks) > 10000 {
		return fmt.Errorf("too many backup tasks")
	}
	contentItems := 0
	for _, storage := range r.Storages {
		if len(storage.Storage) > maxMetadataLength {
			return fmt.Errorf("storage name is too long")
		}
		if len(storage.StorageInfo) > 16 {
			return fmt.Errorf("too many storage info entries")
		}
		contentItems += len(storage.ContentData)
		if contentItems > 20000 {
			return fmt.Errorf("too many storage content entries")
		}
		for _, info := range storage.StorageInfo {
			if info.Total < 0 || info.Used < 0 || info.Avail < 0 {
				return fmt.Errorf("storage byte values cannot be negative")
			}
		}
		for _, content := range storage.ContentData {
			if content.VMID < 0 || content.Size < 0 {
				return fmt.Errorf("storage content values cannot be negative")
			}
		}
	}
	for _, task := range r.BackupTasks {
		if task.VMID < 0 || task.StartTime < 0 || task.EndTime < 0 || task.Duration < 0 || task.Size < 0 {
			return fmt.Errorf("backup task values cannot be negative")
		}
	}
	return nil
}

func (r *PBSReportRequest) Validate() error {
	if err := validateReportIdentity(r.ReportID, r.Hostname, r.IPAddress, r.PublicIP, r.ClientVersion, r.MachineID); err != nil {
		return err
	}
	if r.SwapTotal < 0 || r.SwapUsed < 0 {
		return fmt.Errorf("swap values cannot be negative")
	}
	if len(r.PBSInformation.Data) > 256 {
		return fmt.Errorf("too many datastores")
	}
	if len(r.PBSInformation.Tasks) > 10000 {
		return fmt.Errorf("too many maintenance tasks")
	}
	groups := 0
	for _, datastore := range r.PBSInformation.Data {
		if len(datastore.Store) > maxMetadataLength {
			return fmt.Errorf("datastore name is too long")
		}
		if datastore.Total < 0 || datastore.Used < 0 || datastore.Avail < 0 {
			return fmt.Errorf("datastore byte values cannot be negative")
		}
		if len(datastore.History) > 10000 {
			return fmt.Errorf("datastore history is too large")
		}
		groups += len(datastore.Groups)
		if groups > 50000 {
			return fmt.Errorf("too many backup groups")
		}
		for _, group := range datastore.Groups {
			if group.LastBackup < 0 || group.BackupCount < 0 || group.Size < 0 {
				return fmt.Errorf("backup group values cannot be negative")
			}
		}
	}
	return nil
}

func (r *WindowsReportRequest) Validate() error {
	if err := validateReportIdentity(r.ReportID, r.Hostname, r.IPAddress, r.PublicIP, r.ClientVersion, r.MachineID); err != nil {
		return err
	}
	if len(r.Disks) > 512 {
		return fmt.Errorf("too many disks")
	}
	for _, disk := range r.Disks {
		if len(disk.Name) > maxMetadataLength {
			return fmt.Errorf("disk name is too long")
		}
		if disk.Total < 0 || disk.Used < 0 || disk.Free < 0 {
			return fmt.Errorf("disk byte values cannot be negative")
		}
	}
	return nil
}

func (r *HeartbeatRequest) Validate() error {
	if err := validateReportIdentity("", r.Hostname, r.IPAddress, r.PublicIP, r.ClientVersion, r.MachineID); err != nil {
		return err
	}
	if r.SwapTotal < 0 || r.SwapUsed < 0 {
		return fmt.Errorf("swap values cannot be negative")
	}
	return nil
}

func validateReportIdentity(reportID, hostname, ip, publicIP, clientVersion, machineID string) error {
	if strings.TrimSpace(hostname) == "" {
		return fmt.Errorf("hostname is required")
	}
	if len(hostname) > maxHostnameLength {
		return fmt.Errorf("hostname is too long")
	}
	if len(reportID) > maxReportIDLength || (reportID != "" && !validReportID(reportID)) {
		return fmt.Errorf("report_id is invalid")
	}
	for name, value := range map[string]string{
		"ip_address": ip, "public_ip": publicIP, "client_version": clientVersion, "machine_id": machineID,
	} {
		if len(value) > maxMetadataLength {
			return fmt.Errorf("%s is too long", name)
		}
	}
	return nil
}

func validReportID(value string) bool {
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || strings.ContainsRune("._:-", r) {
			continue
		}
		return false
	}
	return value != ""
}

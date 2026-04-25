package service

import (
	"fmt"
	"strings"
	"time"

	"probaky/internal/domain"
	"probaky/internal/store"
)

type ReportService struct {
	store *store.Store
	tz    *time.Location
}

func NewReport(st *store.Store, tz *time.Location) *ReportService {
	return &ReportService{store: st, tz: tz}
}

func (r *ReportService) SavePVEReport(req *domain.PVEReportRequest) error {
	serverID, err := r.store.UpsertPVEServer(
		req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
	)
	if err != nil {
		return fmt.Errorf("upsert server: %w", err)
	}

	reportID, err := r.store.InsertPVEReport(serverID, req.LastBackupStatus)
	if err != nil {
		return fmt.Errorf("insert report: %w", err)
	}

	for _, st := range req.Storages {
		stID, err := r.store.InsertPVEStorage(reportID, st)
		if err != nil {
			return fmt.Errorf("insert storage %s: %w", st.Storage, err)
		}
		for _, info := range st.StorageInfo {
			if err := r.store.InsertPVEStorageInfo(stID, info); err != nil {
				return fmt.Errorf("insert storage info: %w", err)
			}
		}
		for _, c := range st.ContentData {
			if err := r.store.InsertPVEStorageContent(stID, c); err != nil {
				return fmt.Errorf("insert content: %w", err)
			}
		}
	}
	return nil
}

func (r *ReportService) SavePBSReport(req *domain.PBSReportRequest) error {
	serverID, err := r.store.UpsertPBSServer(
		req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
	)
	if err != nil {
		return fmt.Errorf("upsert pbs server: %w", err)
	}

	reportID, err := r.store.InsertPBSReport(serverID)
	if err != nil {
		return fmt.Errorf("insert pbs report: %w", err)
	}

	for _, ds := range req.PBSInformation.Data {
		storeID, err := r.store.InsertPBSStore(reportID, ds)
		if err != nil {
			return fmt.Errorf("insert pbs store %s: %w", ds.Store, err)
		}
		if err := r.store.InsertPBSStoreHistory(storeID, ds.History); err != nil {
			return fmt.Errorf("insert pbs history: %w", err)
		}
		if err := r.store.InsertPBSGCStatus(storeID, ds.GCStatus); err != nil {
			return fmt.Errorf("insert gc status: %w", err)
		}
	}
	return nil
}

// IsStale returns true when the report was not received today (in the configured timezone).
func (r *ReportService) IsStale(reportedAt time.Time) bool {
	now := time.Now().In(r.tz)
	rep := reportedAt.In(r.tz)
	return now.Year() != rep.Year() || now.YearDay() != rep.YearDay()
}

// BuildPVEServerResponse assembles a PVEServerResponse enriched with latest report data.
func (r *ReportService) BuildPVEServerResponse(sv domain.PVEServer) domain.PVEServerResponse {
	resp := domain.PVEServerResponse{
		ID:           sv.ID,
		Name:         sv.Name,
		IP:           sv.IP,
		PublicIP:     sv.PublicIP,
		ClientVersion: sv.ClientVersion,
		MachineBound: sv.MachineID != "",
	}
	rep, err := r.store.GetLatestPVEReport(sv.ID)
	if err != nil {
		resp.IsStale = true
		resp.StaleReason = "no reports received"
		return resp
	}
	resp.LastReport = &rep.ReportedAt
	resp.BackupStatus = rep.BackupStatus
	if r.IsStale(rep.ReportedAt) {
		resp.IsStale = true
		resp.StaleReason = "no report received today"
	} else {
		resp.IsStale = rep.IsStale
		resp.StaleReason = rep.StaleReason
	}
	return resp
}

// BuildPBSServerResponse assembles a PBSServerResponse enriched with latest report data.
func (r *ReportService) BuildPBSServerResponse(sv domain.PBSServer) domain.PBSServerResponse {
	resp := domain.PBSServerResponse{
		ID:           sv.ID,
		Name:         sv.Name,
		IP:           sv.IP,
		PublicIP:     sv.PublicIP,
		ClientVersion: sv.ClientVersion,
		MachineBound: sv.MachineID != "",
	}
	rep, err := r.store.GetLatestPBSReport(sv.ID)
	if err != nil {
		resp.IsStale = true
		resp.StaleReason = "no reports received"
		return resp
	}
	resp.LastReport = &rep.ReportedAt
	if r.IsStale(rep.ReportedAt) {
		resp.IsStale = true
		resp.StaleReason = "no report received today"
	} else {
		resp.IsStale = rep.IsStale
		resp.StaleReason = rep.StaleReason
	}
	return resp
}

// KeyPreview returns "pbk-xxxx...yyyy" style preview for display.
func KeyPreview(key string) string {
	if len(key) <= 12 {
		return key
	}
	parts := strings.SplitN(key, "-", 2)
	if len(parts) != 2 || len(parts[1]) <= 8 {
		return key[:4] + "..." + key[len(key)-4:]
	}
	tok := parts[1]
	return parts[0] + "-" + tok[:8] + "..." + tok[len(tok)-4:]
}

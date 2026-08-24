package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

type ReportService struct {
	store *store.Store
	tz    *time.Location
	now   func() time.Time
}

func NewReport(st *store.Store, tz *time.Location) *ReportService {
	return &ReportService{store: st, tz: tz, now: time.Now}
}

func (r *ReportService) SavePVEReport(ctx context.Context, req *domain.PVEReportRequest) error {
	return r.SavePVEReportForAPIKey(ctx, req, 0)
}

func (r *ReportService) SavePVEReportForAPIKey(ctx context.Context, req *domain.PVEReportRequest, apiKeyID int64) error {
	var (
		serverID int64
		err      error
	)
	if apiKeyID > 0 {
		serverID, err = r.store.UpsertPVEServerForAPIKey(ctx, apiKeyID,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	} else {
		serverID, err = r.store.UpsertPVEServer(ctx,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	}
	if err != nil {
		return fmt.Errorf("upsert server: %w", err)
	}

	if err := r.store.InsertPVEReportData(ctx, serverID, req); err != nil {
		return fmt.Errorf("insert report data: %w", err)
	}
	return nil
}

func (r *ReportService) SavePBSReport(ctx context.Context, req *domain.PBSReportRequest) error {
	return r.SavePBSReportForAPIKey(ctx, req, 0)
}

func (r *ReportService) SavePBSReportForAPIKey(ctx context.Context, req *domain.PBSReportRequest, apiKeyID int64) error {
	var (
		serverID int64
		err      error
	)
	if apiKeyID > 0 {
		serverID, err = r.store.UpsertPBSServerForAPIKey(ctx, apiKeyID,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	} else {
		serverID, err = r.store.UpsertPBSServer(ctx,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	}
	if err != nil {
		return fmt.Errorf("upsert pbs server: %w", err)
	}

	err = r.store.InsertPBSReportData(ctx, serverID, req.ReportID, domain.HostSwap{
		Total: req.SwapTotal, Used: req.SwapUsed, Enabled: req.SwapEnabled,
	}, req.PBSInformation)
	if err != nil {
		return fmt.Errorf("insert pbs report: %w", err)
	}
	return nil
}

func (r *ReportService) SaveWindowsReportForAPIKey(ctx context.Context, req *domain.WindowsReportRequest, apiKeyID int64) error {
	var (
		serverID int64
		err      error
	)
	if apiKeyID > 0 {
		serverID, err = r.store.UpsertWindowsServerForAPIKey(ctx, apiKeyID,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	} else {
		serverID, err = r.store.UpsertWindowsServer(ctx,
			req.Hostname, req.IPAddress, req.PublicIP, req.ClientVersion, req.MachineID,
		)
	}
	if err != nil {
		return fmt.Errorf("upsert windows server: %w", err)
	}

	if err := r.store.InsertWindowsReportData(ctx, serverID, req); err != nil {
		return fmt.Errorf("insert windows report data: %w", err)
	}
	return nil
}

// IsStale returns true when the report was not received today (in the configured timezone).
func (r *ReportService) IsStale(reportedAt time.Time) bool {
	now := r.now().In(r.tz)
	rep := reportedAt.In(r.tz)
	return now.Year() != rep.Year() || now.YearDay() != rep.YearDay()
}

// IsStaleForServer checks staleness considering the server's configured backup schedule.
// Returns (isStale, reason). If no schedule is configured, falls back to IsStale.
// A backup day is considered "completed" at the server's expected finish time
// the next morning, using the global cutoff unless the server overrides it.
func (r *ReportService) IsStaleForServer(ctx context.Context, reportedAt time.Time, serverName string) (bool, string) {
	configs, err := r.store.ListVMBackupConfigs(ctx, serverName)
	if err != nil || len(configs) == 0 {
		return r.IsStale(reportedAt), "No se ha recibido el reporte de hoy"
	}

	expected := make(map[time.Weekday]bool)
	for _, c := range configs {
		if c.IsExcluded {
			continue
		}
		if c.Monday {
			expected[time.Monday] = true
		}
		if c.Tuesday {
			expected[time.Tuesday] = true
		}
		if c.Wednesday {
			expected[time.Wednesday] = true
		}
		if c.Thursday {
			expected[time.Thursday] = true
		}
		if c.Friday {
			expected[time.Friday] = true
		}
		if c.Saturday {
			expected[time.Saturday] = true
		}
		if c.Sunday {
			expected[time.Sunday] = true
		}
	}
	if len(expected) == 0 {
		return false, "sin backups activos configurados"
	}

	now := r.now().In(r.tz)
	finishHour, finishMinute := r.expectedFinishTime(ctx, serverName)
	// Look back up to 14 days for the most recent completed expected backup day.
	for i := 1; i <= 14; i++ {
		candidate := now.AddDate(0, 0, -i)
		if !expected[candidate.Weekday()] {
			continue
		}
		dayStart := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, r.tz)
		cutoff := dayStart.AddDate(0, 0, 1).Add(time.Duration(finishHour)*time.Hour + time.Duration(finishMinute)*time.Minute)
		if now.Before(cutoff) {
			// This day's backup window hasn't closed yet - keep looking back
			continue
		}
		return reportedAt.Before(dayStart), "no se ha recibido reporte del ultimo dia de backup"
	}
	return r.IsStale(reportedAt), "No se ha recibido el reporte de hoy"
}

func (r *ReportService) IsStaleForServerID(ctx context.Context, reportedAt time.Time, serverID int64) (bool, string) {
	configs, err := r.store.ListVMBackupConfigsForServer(ctx, "pve", serverID)
	if err != nil || len(configs) == 0 {
		return r.IsStale(reportedAt), "No se ha recibido el reporte de hoy"
	}
	return r.isStaleForConfigs(ctx, reportedAt, configs, serverID)
}

func (r *ReportService) IsStaleForLoadedPVEConfig(reportedAt time.Time, configs []domain.VMBackupConfig, alertCfg domain.PVEAlertConfig, globalFinishTime string) (bool, string) {
	if len(configs) == 0 {
		return r.IsStale(reportedAt), "No se ha recibido el reporte de hoy"
	}
	finishHour, finishMinute := expectedFinishFromAlertConfig(alertCfg, globalFinishTime)
	return r.isStaleForConfigsAt(reportedAt, configs, finishHour, finishMinute)
}

func (r *ReportService) isStaleForConfigs(ctx context.Context, reportedAt time.Time, configs []domain.VMBackupConfig, serverID int64) (bool, string) {
	finishHour, finishMinute := r.expectedFinishTimeByServerID(ctx, serverID)
	return r.isStaleForConfigsAt(reportedAt, configs, finishHour, finishMinute)
}

func (r *ReportService) isStaleForConfigsAt(reportedAt time.Time, configs []domain.VMBackupConfig, finishHour, finishMinute int) (bool, string) {
	expected := make(map[time.Weekday]bool)
	for _, c := range configs {
		if c.IsExcluded {
			continue
		}
		if c.Monday {
			expected[time.Monday] = true
		}
		if c.Tuesday {
			expected[time.Tuesday] = true
		}
		if c.Wednesday {
			expected[time.Wednesday] = true
		}
		if c.Thursday {
			expected[time.Thursday] = true
		}
		if c.Friday {
			expected[time.Friday] = true
		}
		if c.Saturday {
			expected[time.Saturday] = true
		}
		if c.Sunday {
			expected[time.Sunday] = true
		}
	}
	if len(expected) == 0 {
		return false, "sin backups activos configurados"
	}

	now := r.now().In(r.tz)
	for i := 1; i <= 14; i++ {
		candidate := now.AddDate(0, 0, -i)
		if !expected[candidate.Weekday()] {
			continue
		}
		dayStart := time.Date(candidate.Year(), candidate.Month(), candidate.Day(), 0, 0, 0, 0, r.tz)
		cutoff := dayStart.AddDate(0, 0, 1).Add(time.Duration(finishHour)*time.Hour + time.Duration(finishMinute)*time.Minute)
		if now.Before(cutoff) {
			continue
		}
		return reportedAt.Before(dayStart), "no se ha recibido reporte del ultimo dia de backup"
	}
	return r.IsStale(reportedAt), "No se ha recibido el reporte de hoy"
}

func expectedFinishFromAlertConfig(cfg domain.PVEAlertConfig, globalFinishTime string) (int, int) {
	const defaultHour, defaultMinute = 9, 0
	if cfg.ExpectedFinishTime != nil {
		if t, err := time.Parse("15:04", *cfg.ExpectedFinishTime); err == nil {
			return t.Hour(), t.Minute()
		}
	}
	if t, err := time.Parse("15:04", globalFinishTime); err == nil {
		return t.Hour(), t.Minute()
	}
	return defaultHour, defaultMinute
}

func (r *ReportService) expectedFinishTime(ctx context.Context, serverName string) (int, int) {
	const defaultHour, defaultMinute = 9, 0
	sv, err := r.store.GetPVEServerByName(ctx, serverName)
	if err != nil {
		return defaultHour, defaultMinute
	}
	cfg, err := r.store.GetPVEAlertConfig(ctx, sv.ID)
	if err != nil {
		return defaultHour, defaultMinute
	}
	return expectedFinishFromAlertConfig(cfg, r.globalPVEExpectedFinishTime(ctx))
}

func (r *ReportService) expectedFinishTimeByServerID(ctx context.Context, serverID int64) (int, int) {
	cfg, err := r.store.GetPVEAlertConfig(ctx, serverID)
	if err != nil {
		return 9, 0
	}
	return expectedFinishFromAlertConfig(cfg, r.globalPVEExpectedFinishTime(ctx))
}

func (r *ReportService) globalPVEExpectedFinishTime(ctx context.Context) string {
	cfg, err := r.store.GetEmailConfig(ctx)
	if err != nil {
		return ""
	}
	return cfg.AlertPVEExpectedFinishTime
}

// BuildPVEServerResponse assembles a PVEServerResponse enriched with latest report data.
func (r *ReportService) BuildPVEServerResponse(ctx context.Context, sv domain.PVEServer) domain.PVEServerResponse {
	resp := domain.PVEServerResponse{
		ID:            sv.ID,
		Name:          sv.Name,
		IP:            sv.IP,
		PublicIP:      sv.PublicIP,
		ClientVersion: sv.ClientVersion,
		MachineBound:  sv.MachineID != "",
	}
	rep, err := r.store.GetLatestPVEReport(ctx, sv.ID)
	if err != nil {
		resp.IsStale = true
		resp.StaleReason = "no se han recibido reportes"
		return resp
	}
	resp.LastReport = &rep.ReportedAt
	resp.BackupStatus = rep.BackupStatus
	if stale, reason := r.IsStaleForServerID(ctx, rep.ReportedAt, sv.ID); stale {
		resp.IsStale = true
		resp.StaleReason = reason
	} else {
		resp.IsStale = rep.IsStale
		resp.StaleReason = rep.StaleReason
	}
	return resp
}

// BuildPBSServerResponse assembles a PBSServerResponse enriched with latest report data.
func (r *ReportService) BuildPBSServerResponse(ctx context.Context, sv domain.PBSServer) domain.PBSServerResponse {
	resp := domain.PBSServerResponse{
		ID:            sv.ID,
		Name:          sv.Name,
		IP:            sv.IP,
		PublicIP:      sv.PublicIP,
		ClientVersion: sv.ClientVersion,
		MachineBound:  sv.MachineID != "",
	}
	rep, err := r.store.GetLatestPBSReport(ctx, sv.ID)
	if err != nil {
		resp.IsStale = true
		resp.StaleReason = "no se han recibido reportes"
		return resp
	}
	resp.LastReport = &rep.ReportedAt
	if r.IsStale(rep.ReportedAt) {
		resp.IsStale = true
		resp.StaleReason = "No se ha recibido el reporte de hoy"
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

package service

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

//go:embed email_template.html
var emailTemplateHTML string

type serverRow struct {
	Name        string
	IP          string
	StaleReason string
	VMTasks     []vmTaskRow
	Datastores  []datastoreRow
}

type datastoreRow struct {
	Name        string
	Used        string
	Total       string
	UsedPct     int
	MountStatus string
}

type vmTaskRow struct {
	VMID      string
	VMName    string
	Status    string
	Duration  string
	Size      string
	IsMissing bool
}

type diskAlertRow struct {
	ServerName string
	StoreName  string
	UsedPct    int
	Detail     string
}

type emailData struct {
	ReportDate   string
	SendTime     string
	HeaderColor  string
	StatusText   string
	TotalPVE     int
	TotalPBS     int
	TotalIssues  int
	TotalOK      int
	PVEIssues    []serverRow
	PBSIssues    []serverRow
	PVEOk        []serverRow
	PBSOk        []serverRow
	DiskAlerts   []diskAlertRow
	BackupErrors []serverRow
}

// SendDailyReport builds and sends the daily status email.
func SendDailyReport(st *store.Store, rep *ReportService) error {
	ctx := context.Background()
	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		return fmt.Errorf("get email config: %w", err)
	}
	if !cfg.IsEnabled {
		slog.Info("email disabled, skipping daily report")
		return nil
	}
	if cfg.SMTPUser == "" || cfg.SMTPPass == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}

	recipients := parseRecipients(cfg.Recipients)
	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	data, err := buildEmailData(st, rep, cfg)
	if err != nil {
		return fmt.Errorf("build email data: %w", err)
	}

	html, err := renderEmailTemplate(data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("Probakgo Report: Todos los sistemas operativos - %s", data.ReportDate)
	if data.TotalIssues > 0 {
		subject = fmt.Sprintf("Probakgo Alert: %d servidor(es) con problemas - %s", data.TotalIssues, data.ReportDate)
	}

	return sendSMTP(cfg, recipients, subject, html)
}

func buildEmailData(st *store.Store, rep *ReportService, cfg *domain.EmailConfig) (emailData, error) {
	ctx := context.Background()
	sendTime := cfg.SendTime
	pveServers, err := st.ListPVEServers(ctx)
	if err != nil {
		return emailData{}, err
	}
	pbsServers, err := st.ListPBSServers(ctx)
	if err != nil {
		return emailData{}, err
	}

	var pveIssues, pveOk []serverRow
	for _, sv := range pveServers {
		row := serverRow{Name: sv.Name, IP: sv.IP}
		r, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			row.StaleReason = "no reports received"
			pveIssues = append(pveIssues, row)
			continue
		}

		tasks, _ := st.GetPVEBackupTasksForReport(ctx, r.ID)
		for _, t := range tasks {
			name := t.VMName
			if name == "" {
				name = fmt.Sprintf("%d", t.VMID)
			}
			row.VMTasks = append(row.VMTasks, vmTaskRow{
				VMID:     fmt.Sprintf("%d", t.VMID),
				VMName:   name,
				Status:   t.Status,
				Duration: emailFmtDuration(t.Duration),
				Size:     emailFmtBytes(t.Size),
			})
		}
		if len(tasks) > 0 {
			configs, _ := st.ListVMBackupConfigs(ctx, sv.Name)
			if len(configs) > 0 {
				jobDay := time.Unix(tasks[0].StartTime, 0).Weekday()
				seenVMIDs := make(map[string]bool)
				for _, t := range tasks {
					seenVMIDs[fmt.Sprintf("%d", t.VMID)] = true
				}
				for _, c := range configs {
					if c.IsExcluded || !emailVMScheduledForDay(c, jobDay) || seenVMIDs[c.VMID] {
						continue
					}
					name := c.VMName
					if name == "" {
						name = c.VMID
					}
					row.VMTasks = append(row.VMTasks, vmTaskRow{
						VMID:      c.VMID,
						VMName:    name,
						IsMissing: true,
					})
				}
			}
		}

		if stale, reason := rep.IsStaleForServer(r.ReportedAt, sv.Name); stale {
			row.StaleReason = reason
			pveIssues = append(pveIssues, row)
		} else if r.IsStale {
			row.StaleReason = r.StaleReason
			pveIssues = append(pveIssues, row)
		} else {
			pveOk = append(pveOk, row)
		}
	}

	var pbsIssues, pbsOk []serverRow
	for _, sv := range pbsServers {
		row := serverRow{Name: sv.Name, IP: sv.IP}
		r, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			row.StaleReason = "no reports received"
			pbsIssues = append(pbsIssues, row)
			continue
		}
		if stores, err := st.GetPBSStoresForReport(ctx, r.ID); err == nil {
			for _, ds := range stores {
				usedPct := 0
				if ds.Total > 0 {
					usedPct = int(ds.Used * 100 / ds.Total)
				}
				row.Datastores = append(row.Datastores, datastoreRow{
					Name:        ds.Store,
					Used:        emailFmtBytes(ds.Used),
					Total:       emailFmtBytes(ds.Total),
					UsedPct:     usedPct,
					MountStatus: ds.MountStatus,
				})
			}
		}
		if rep.IsStale(r.ReportedAt) {
			row.StaleReason = "no report received today"
			pbsIssues = append(pbsIssues, row)
		} else if r.IsStale {
			row.StaleReason = r.StaleReason
			pbsIssues = append(pbsIssues, row)
		} else {
			pbsOk = append(pbsOk, row)
		}
	}

	// Alerts: disk usage and backup errors via unified alert engine
	var diskAlerts []diskAlertRow
	var backupErrors []serverRow
	if alertCfg, err := LoadAlertConfigs(st); err == nil {
		if alerts, err := RunAll(st, alertCfg); err == nil {
			for _, a := range alerts {
				switch a.Type {
				case domain.AlertTypeDisk:
					pct, _ := strconv.Atoi(a.Value)
					diskAlerts = append(diskAlerts, diskAlertRow{
						ServerName: a.ServerName,
						StoreName:  a.StoreName,
						UsedPct:    pct,
						Detail:     a.Message,
					})
				case domain.AlertTypeBackupError:
					backupErrors = append(backupErrors, serverRow{
						Name:        a.ServerName,
						StaleReason: a.Message,
					})
				}
			}
		}
	}

	totalIssues := len(pveIssues) + len(pbsIssues)
	totalProblems := totalIssues + len(diskAlerts) + len(backupErrors)
	totalOK := len(pveOk) + len(pbsOk)
	headerColor := "#28a745"
	statusText := "Todos los servidores operativos"
	if totalProblems > 0 {
		headerColor = "#dc3545"
		statusText = fmt.Sprintf("%d problema(s) detectado(s)", totalProblems)
	}

	return emailData{
		ReportDate:   time.Now().In(rep.tz).Format("2006-01-02"),
		SendTime:     sendTime,
		HeaderColor:  headerColor,
		StatusText:   statusText,
		TotalPVE:     len(pveServers),
		TotalPBS:     len(pbsServers),
		TotalIssues:  totalProblems,
		TotalOK:      totalOK,
		PVEIssues:    pveIssues,
		PBSIssues:    pbsIssues,
		PVEOk:        pveOk,
		PBSOk:        pbsOk,
		DiskAlerts:   diskAlerts,
		BackupErrors: backupErrors,
	}, nil
}

func renderEmailTemplate(data emailData) (string, error) {
	tmpl, err := template.New("email").Parse(emailTemplateHTML)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func sendSMTP(cfg *domain.EmailConfig, recipients []string, subject, html string) error {
	addr := fmt.Sprintf("%s:%d", cfg.SMTPHost, cfg.SMTPPort)
	auth := smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPass, cfg.SMTPHost)

	msg := buildMIMEMessage(cfg.SMTPUser, recipients, subject, html)
	if err := smtp.SendMail(addr, auth, cfg.SMTPUser, recipients, msg); err != nil {
		return fmt.Errorf("smtp send: %w", err)
	}
	slog.Info("daily email sent", "recipients", len(recipients))
	return nil
}

func buildMIMEMessage(from string, to []string, subject, html string) []byte {
	var b strings.Builder
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + strings.Join(to, ", ") + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("\r\n")
	b.WriteString(html)
	return []byte(b.String())
}

func parseRecipients(raw string) []string {
	var out []string
	for s := range strings.SplitSeq(raw, ",") {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// StartEmailScheduler runs in a goroutine and fires SendDailyReport each day at the configured send_time.
func StartEmailScheduler(ctx context.Context, st *store.Store, rep *ReportService) {
	go func() {
		for {
			cfg, err := st.GetEmailConfig(context.Background())
			if err != nil || !cfg.IsEnabled {
				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Minute):
					continue
				}
			}

			next := nextRunTime(cfg.SendTime, rep.tz)
			slog.Info("email scheduler: next run", "at", next.Format(time.RFC3339))

			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
			}

			if err := SendDailyReport(st, rep); err != nil {
				slog.Error("daily email failed", "err", err)
			}

			// Sleep 1 min to avoid re-firing in the same minute
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Minute):
			}
		}
	}()
}

func emailFmtBytes(b int64) string { return domain.FormatBytes(b) }

func emailFmtDuration(secs int64) string {
	if secs <= 0 {
		return "–"
	}
	h := secs / 3600
	m := (secs % 3600) / 60
	s := secs % 60
	if h > 0 {
		return fmt.Sprintf("%dh %dm", h, m)
	}
	if m > 0 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	return fmt.Sprintf("%ds", s)
}

func emailVMScheduledForDay(c domain.VMBackupConfig, day time.Weekday) bool {
	switch day {
	case time.Monday:
		return c.Monday
	case time.Tuesday:
		return c.Tuesday
	case time.Wednesday:
		return c.Wednesday
	case time.Thursday:
		return c.Thursday
	case time.Friday:
		return c.Friday
	case time.Saturday:
		return c.Saturday
	case time.Sunday:
		return c.Sunday
	}
	return false
}

// nextRunTime returns the next wall-clock moment matching HH:MM in the given timezone.
func nextRunTime(sendTime string, loc *time.Location) time.Time {
	now := time.Now().In(loc)
	var h, m int
	fmt.Sscanf(sendTime, "%d:%d", &h, &m)
	next := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, loc)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}

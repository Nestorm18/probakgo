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
	"sync"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

//go:embed email_template.html
var emailTemplateHTML string

var criticalEmailState = struct {
	sync.Mutex
	sentAt map[string]time.Time
}{sentAt: make(map[string]time.Time)}

const criticalEmailThrottle = time.Hour

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
	VMID       string
	VMName     string
	Status     string
	Duration   string
	Size       string
	IsMissing  bool
	IsExcluded bool
}

type diskAlertRow struct {
	ServerName string
	StoreName  string
	UsedPct    int
	Detail     string
}

type summaryIssueRow struct {
	Name   string
	Kind   string
	Detail string
}

type emailData struct {
	ReportDate    string
	SendTime      string
	HeaderColor   string
	StatusText    string
	SummaryIssues []summaryIssueRow
	TotalPVE      int
	TotalPBS      int
	TotalWindows  int
	TotalIssues   int
	TotalOK       int
	PVEIssues     []serverRow
	PBSIssues     []serverRow
	WindowsIssues []serverRow
	PVEOk         []serverRow
	PBSOk         []serverRow
	WindowsOk     []serverRow
	DiskAlerts    []diskAlertRow
	BackupErrors  []serverRow
}

// SendDailyReport builds and sends the daily status email.
func SendDailyReport(st *store.Store, rep *ReportService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
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

	data, err := buildEmailData(ctx, st, rep, cfg)
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

// SendImmediateCriticalAlerts sends an optional, throttled email for active critical alerts.
func SendImmediateCriticalAlerts(st *store.Store, rep *ReportService) error {
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		return fmt.Errorf("get email config: %w", err)
	}
	if !cfg.CriticalAlertsEnabled {
		return nil
	}
	if cfg.SMTPUser == "" || cfg.SMTPPass == "" {
		return fmt.Errorf("SMTP credentials not configured")
	}
	recipients := parseRecipients(cfg.Recipients)
	if len(recipients) == 0 {
		return fmt.Errorf("no email recipients configured")
	}

	alertCfg, err := LoadAlertConfigs(ctx, st)
	if err != nil {
		return fmt.Errorf("load alert config: %w", err)
	}
	alertCfg.Report = rep
	alerts, err := RunAll(st, alertCfg)
	if err != nil {
		return fmt.Errorf("run alerts: %w", err)
	}
	_ = st.SyncAlertStates(ctx, alerts)
	suppressed, _ := st.GetActiveSuppressions(ctx)

	now := time.Now()
	var selected []domain.Alert
	criticalEmailState.Lock()
	for _, a := range alerts {
		if !shouldSendImmediateCriticalEmail(a) {
			continue
		}
		if _, ok := suppressed[a.ID]; ok {
			continue
		}
		if last, ok := criticalEmailState.sentAt[a.ID]; ok && now.Sub(last) < criticalEmailThrottle {
			continue
		}
		selected = append(selected, a)
	}
	criticalEmailState.Unlock()
	if len(selected) == 0 {
		return nil
	}

	subject := fmt.Sprintf("Probakgo alerta critica: %d alerta(s) activa(s)", len(selected))
	if err := sendSMTP(cfg, recipients, subject, renderImmediateCriticalEmail(selected, now)); err != nil {
		return err
	}

	criticalEmailState.Lock()
	for _, a := range selected {
		criticalEmailState.sentAt[a.ID] = now
	}
	criticalEmailState.Unlock()
	return nil
}

func shouldSendImmediateCriticalEmail(a domain.Alert) bool {
	if a.Severity != domain.AlertSeverityCritical {
		return false
	}
	switch a.Type {
	case domain.AlertTypePBSReportStale, domain.AlertTypePBSStale:
		return false
	default:
		return true
	}
}

func renderImmediateCriticalEmail(alerts []domain.Alert, now time.Time) string {
	var b strings.Builder
	b.WriteString(`<div style="font-family:Arial,sans-serif;color:#111827">`)
	b.WriteString(`<h2 style="margin:0 0 12px;color:#dc2626">Probakgo alerta critica</h2>`)
	b.WriteString(`<p style="margin:0 0 16px;color:#4b5563">Detectado el `)
	b.WriteString(template.HTMLEscapeString(now.Format("2006-01-02 15:04:05")))
	b.WriteString(`.</p>`)
	b.WriteString(`<table style="width:100%;border-collapse:collapse;font-size:14px">`)
	b.WriteString(`<thead><tr><th align="left" style="border-bottom:1px solid #e5e7eb;padding:8px">Servidor</th><th align="left" style="border-bottom:1px solid #e5e7eb;padding:8px">Alerta</th><th align="left" style="border-bottom:1px solid #e5e7eb;padding:8px">Detalle</th></tr></thead><tbody>`)
	for _, a := range alerts {
		b.WriteString(`<tr>`)
		b.WriteString(`<td style="border-bottom:1px solid #f3f4f6;padding:8px">`)
		b.WriteString(template.HTMLEscapeString(a.ServerName))
		b.WriteString(`</td><td style="border-bottom:1px solid #f3f4f6;padding:8px">`)
		b.WriteString(template.HTMLEscapeString(a.Title))
		b.WriteString(`</td><td style="border-bottom:1px solid #f3f4f6;padding:8px">`)
		b.WriteString(template.HTMLEscapeString(a.Message))
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</tbody></table></div>`)
	return b.String()
}

func buildEmailData(ctx context.Context, st *store.Store, rep *ReportService, cfg *domain.EmailConfig) (emailData, error) {
	sendTime := cfg.SendTime
	pveServers, err := st.ListPVEServers(ctx)
	if err != nil {
		return emailData{}, err
	}
	pbsServers, err := st.ListPBSServers(ctx)
	if err != nil {
		return emailData{}, err
	}
	windowsServers, err := st.ListWindowsServers(ctx)
	if err != nil {
		return emailData{}, err
	}

	var pveIssues, pveOk []serverRow
	for _, sv := range pveServers {
		row := serverRow{Name: sv.DisplayName, IP: sv.IP}
		configs, _ := st.ListVMBackupConfigsForServerOrName(ctx, "pve", sv.ID, sv.Name)
		if len(configs) > 0 && !domain.HasActiveVMBackupConfigs(configs) {
			continue
		}
		r, err := st.GetLatestPVEReport(ctx, sv.ID)
		if err != nil {
			row.StaleReason = "no se han recibido reportes"
			pveIssues = append(pveIssues, row)
			continue
		}

		tasks, _ := st.GetPVEBackupTasksForReport(ctx, r.ID)
		isStale := false
		staleReason := ""
		if stale, reason := rep.IsStaleForServerID(ctx, r.ReportedAt, sv.ID); stale {
			isStale = true
			staleReason = reason
		} else if r.IsStale {
			isStale = true
			staleReason = r.StaleReason
		}

		if isStale {
			row.StaleReason = staleReason
			row.VMTasks = staleVMRows(configs, tasks)
			pveIssues = append(pveIssues, row)
			continue
		}

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
		missingRows, activeMissing := missingVMRows(configs, tasks)
		row.VMTasks = append(row.VMTasks, missingRows...)
		if activeMissing > 0 {
			if activeMissing == 1 {
				row.StaleReason = "1 VM activa sin backup en el ultimo job"
			} else {
				row.StaleReason = fmt.Sprintf("%d VMs activas sin backup en el ultimo job", activeMissing)
			}
			pveIssues = append(pveIssues, row)
		} else {
			pveOk = append(pveOk, row)
		}
	}

	var pbsIssues, pbsOk []serverRow
	for _, sv := range pbsServers {
		row := serverRow{Name: sv.DisplayName, IP: sv.IP}
		r, err := st.GetLatestPBSReport(ctx, sv.ID)
		if err != nil {
			row.StaleReason = "no se han recibido reportes"
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
			row.StaleReason = "No se ha recibido el reporte de hoy"
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
	windowsAlertReasons := make(map[int64][]string)
	if alertCfg, err := LoadAlertConfigs(ctx, st); err == nil {
		alertCfg.Report = rep
		if alerts, err := RunAll(st, alertCfg); err == nil {
			suppressed, _ := st.GetActiveSuppressions(ctx)
			for _, a := range alerts {
				if _, ok := suppressed[a.ID]; ok {
					continue
				}
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
				case domain.AlertTypeWindowsHeartbeat, domain.AlertTypeWindowsDiskHealth:
					windowsAlertReasons[a.ServerID] = append(windowsAlertReasons[a.ServerID], a.Message)
				}
			}
		}
	}

	var windowsIssues, windowsOk []serverRow
	for _, sv := range windowsServers {
		row := serverRow{Name: sv.DisplayName, IP: sv.IP}
		r, err := st.GetLatestWindowsReport(ctx, sv.ID)
		if err != nil {
			row.StaleReason = "no se han recibido reportes"
			windowsIssues = append(windowsIssues, row)
			continue
		}
		if disks, err := st.GetWindowsDisksForReport(ctx, r.ID); err == nil {
			for _, disk := range disks {
				if !isWindowsLogicalAlertDisk(disk) {
					continue
				}
				usedPct := 0
				if disk.Total > 0 {
					usedPct = int(disk.Used * 100 / disk.Total)
				}
				row.Datastores = append(row.Datastores, datastoreRow{
					Name:        disk.Name,
					Used:        emailFmtBytes(disk.Used),
					Total:       emailFmtBytes(disk.Total),
					UsedPct:     usedPct,
					MountStatus: emailWindowsDiskStatus(disk),
				})
			}
		}
		if r.IsStale {
			row.StaleReason = "reporte Windows marcado como obsoleto"
			windowsIssues = append(windowsIssues, row)
			continue
		}
		if reasons := windowsAlertReasons[sv.ID]; len(reasons) > 0 {
			row.StaleReason = strings.Join(reasons, "; ")
			windowsIssues = append(windowsIssues, row)
		} else {
			windowsOk = append(windowsOk, row)
		}
	}

	totalIssues := len(pveIssues) + len(pbsIssues) + len(windowsIssues)
	backupProblems := totalIssues + len(backupErrors)
	totalOK := len(pveOk) + len(pbsOk) + len(windowsOk)
	headerColor := "#28a745"
	statusText := "Todos los servidores operativos"
	if backupProblems > 0 {
		headerColor = "#dc3545"
		statusText = fmt.Sprintf("%d problema(s) de backup detectado(s)", backupProblems)
	}

	summaryIssues := buildSummaryIssues(pveIssues, pbsIssues, windowsIssues, backupErrors)

	return emailData{
		ReportDate:    time.Now().In(rep.tz).Format("2006-01-02"),
		SendTime:      sendTime,
		HeaderColor:   headerColor,
		StatusText:    statusText,
		SummaryIssues: summaryIssues,
		TotalPVE:      len(pveServers),
		TotalPBS:      len(pbsServers),
		TotalWindows:  len(windowsServers),
		TotalIssues:   backupProblems,
		TotalOK:       totalOK,
		PVEIssues:     pveIssues,
		PBSIssues:     pbsIssues,
		WindowsIssues: windowsIssues,
		PVEOk:         pveOk,
		PBSOk:         pbsOk,
		WindowsOk:     windowsOk,
		DiskAlerts:    diskAlerts,
		BackupErrors:  backupErrors,
	}, nil
}

func staleVMRows(configs []domain.VMBackupConfig, tasks []domain.PVEBackupTask) []vmTaskRow {
	var rows []vmTaskRow
	if len(configs) > 0 {
		for _, c := range configs {
			if c.IsExcluded {
				continue
			}
			name := c.VMName
			if name == "" {
				name = c.VMID
			}
			rows = append(rows, vmTaskRow{
				VMID:      c.VMID,
				VMName:    name,
				IsMissing: true,
			})
		}
		return rows
	}
	for _, t := range tasks {
		name := t.VMName
		if name == "" {
			name = fmt.Sprintf("%d", t.VMID)
		}
		rows = append(rows, vmTaskRow{
			VMID:      fmt.Sprintf("%d", t.VMID),
			VMName:    name,
			IsMissing: true,
		})
	}
	return rows
}

func missingVMRows(configs []domain.VMBackupConfig, tasks []domain.PVEBackupTask) ([]vmTaskRow, int) {
	if len(tasks) == 0 || len(configs) == 0 {
		return nil, 0
	}
	jobDay := time.Unix(tasks[0].StartTime, 0).Weekday()
	seenVMIDs := make(map[string]bool)
	for _, t := range tasks {
		seenVMIDs[fmt.Sprintf("%d", t.VMID)] = true
	}
	var rows []vmTaskRow
	activeMissing := 0
	for _, c := range configs {
		if !domain.VMScheduledForDay(c, jobDay) || seenVMIDs[c.VMID] {
			continue
		}
		name := c.VMName
		if name == "" {
			name = c.VMID
		}
		if !c.IsExcluded {
			activeMissing++
		}
		rows = append(rows, vmTaskRow{
			VMID:       c.VMID,
			VMName:     name,
			IsMissing:  true,
			IsExcluded: c.IsExcluded,
		})
	}
	return rows, activeMissing
}

func buildSummaryIssues(pveIssues, pbsIssues, windowsIssues, backupErrors []serverRow) []summaryIssueRow {
	var rows []summaryIssueRow
	add := func(kind string, items []serverRow) {
		for _, item := range items {
			detail := item.StaleReason
			if detail == "" {
				detail = "Problema de backup"
			}
			rows = append(rows, summaryIssueRow{
				Name:   item.Name,
				Kind:   kind,
				Detail: detail,
			})
		}
	}
	add("PVE", pveIssues)
	add("PBS", pbsIssues)
	add("Windows", windowsIssues)
	add("Backup", backupErrors)
	return rows
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
	slog.Info("email sent", "recipients", len(recipients))
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

func emailWindowsDiskStatus(disk domain.WindowsDisk) string {
	health := strings.TrimSpace(disk.Health)
	if health == "" {
		return "OK"
	}
	return health
}

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

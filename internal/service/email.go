package service

import (
	"bytes"
	"context"
	_ "embed"
	"fmt"
	"html/template"
	"log/slog"
	"net/smtp"
	"strings"
	"time"

	"probaky/internal/domain"
	"probaky/internal/store"
)

//go:embed email_template.html
var emailTemplateHTML string

type serverRow struct {
	Name        string
	IP          string
	StaleReason string
}

type emailData struct {
	ReportDate  string
	SendTime    string
	HeaderColor string
	StatusText  string
	TotalPVE    int
	TotalPBS    int
	TotalIssues int
	TotalOK     int
	PVEIssues   []serverRow
	PBSIssues   []serverRow
	PVEOk       []serverRow
	PBSOk       []serverRow
}

// SendDailyReport builds and sends the daily status email.
func SendDailyReport(st *store.Store, rep *ReportService) error {
	cfg, err := st.GetEmailConfig()
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

	data, err := buildEmailData(st, rep, cfg.SendTime)
	if err != nil {
		return fmt.Errorf("build email data: %w", err)
	}

	html, err := renderEmailTemplate(data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	subject := fmt.Sprintf("Probaky Report: Todos los sistemas operativos - %s", data.ReportDate)
	if data.TotalIssues > 0 {
		subject = fmt.Sprintf("Probaky Alert: %d servidor(es) con problemas - %s", data.TotalIssues, data.ReportDate)
	}

	return sendSMTP(cfg, recipients, subject, html)
}

func buildEmailData(st *store.Store, rep *ReportService, sendTime string) (emailData, error) {
	pveServers, err := st.ListPVEServers()
	if err != nil {
		return emailData{}, err
	}
	pbsServers, err := st.ListPBSServers()
	if err != nil {
		return emailData{}, err
	}

	var pveIssues, pveOk []serverRow
	for _, sv := range pveServers {
		row := serverRow{Name: sv.Name, IP: sv.IP}
		r, err := st.GetLatestPVEReport(sv.ID)
		if err != nil {
			row.StaleReason = "no reports received"
			pveIssues = append(pveIssues, row)
			continue
		}
		if rep.IsStale(r.ReportedAt) {
			row.StaleReason = "no report received today"
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
		r, err := st.GetLatestPBSReport(sv.ID)
		if err != nil {
			row.StaleReason = "no reports received"
			pbsIssues = append(pbsIssues, row)
			continue
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

	totalIssues := len(pveIssues) + len(pbsIssues)
	totalOK := len(pveOk) + len(pbsOk)
	headerColor := "#28a745"
	statusText := "Todos los servidores operativos"
	if totalIssues > 0 {
		headerColor = "#dc3545"
		statusText = fmt.Sprintf("%d problema(s) detectado(s)", totalIssues)
	}

	return emailData{
		ReportDate:  time.Now().In(rep.tz).Format("2006-01-02"),
		SendTime:    sendTime,
		HeaderColor: headerColor,
		StatusText:  statusText,
		TotalPVE:    len(pveServers),
		TotalPBS:    len(pbsServers),
		TotalIssues: totalIssues,
		TotalOK:     totalOK,
		PVEIssues:   pveIssues,
		PBSIssues:   pbsIssues,
		PVEOk:       pveOk,
		PBSOk:       pbsOk,
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
	for _, s := range strings.Split(raw, ",") {
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
			cfg, err := st.GetEmailConfig()
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

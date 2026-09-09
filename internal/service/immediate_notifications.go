package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"sync"
	"time"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

var immediateNotificationMu sync.Mutex

// SendImmediateCriticalAlerts coordinates the independent email, Web Push and
// Telegram channels when a critical alert appears or is resolved.
func SendImmediateCriticalAlerts(st *store.Store, rep *ReportService) error {
	immediateNotificationMu.Lock()
	defer immediateNotificationMu.Unlock()
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cfg, err := st.GetEmailConfig(ctx)
	if err != nil {
		return fmt.Errorf("get email config: %w", err)
	}
	emailEnabled := cfg.CriticalAlertsEnabled
	push := GetPushSender()
	if push != nil && !push.Ready(ctx) {
		push = nil
	}
	telegram := GetTelegramSender()
	if telegram != nil && !telegram.Ready(ctx) {
		telegram = nil
	}
	if !emailEnabled && push == nil && telegram == nil {
		return nil
	}
	var recipients []string
	var emailConfigErr error
	if emailEnabled {
		if cfg.SMTPUser == "" || cfg.SMTPPass == "" {
			emailConfigErr = fmt.Errorf("SMTP credentials not configured")
		} else {
			recipients = parseRecipients(cfg.Recipients)
			if len(recipients) == 0 {
				emailConfigErr = fmt.Errorf("no email recipients configured")
			}
		}
	}
	if emailConfigErr != nil && push == nil && telegram == nil {
		return emailConfigErr
	}

	alertCfg, err := LoadAlertConfigs(ctx, st)
	if err != nil {
		return fmt.Errorf("load alert config: %w", err)
	}
	alertCfg.Report = rep
	rawAlerts, err := RunAll(st, alertCfg)
	if err != nil {
		return fmt.Errorf("run alerts: %w", err)
	}
	if err := st.SyncAlertStates(ctx, rawAlerts); err != nil {
		return fmt.Errorf("sync alert states: %w", err)
	}
	alerts := FilterMaintenanceAlerts(ctx, st, rawAlerts)
	suppressed, _ := st.GetActiveSuppressions(ctx)

	now := time.Now()
	if push != nil {
		selected, err := criticalAlertsPendingPush(ctx, st, alerts, suppressed)
		if err != nil {
			slog.Warn("get critical push state", "err", err)
		} else if len(selected) > 0 {
			dispatchPush(st, push, selected, alertLink(selected), false)
		}
		resolved, err := st.ListPendingAlertResolutionPushes(ctx)
		if err != nil {
			slog.Warn("list resolved critical push alerts", "err", err)
		} else if len(resolved) > 0 {
			dispatchPush(st, push, resolved, "/alerts", true)
		}
	}
	telegramErr := dispatchTelegram(ctx, st, telegram, alerts, suppressed, now)
	if emailConfigErr != nil {
		return errors.Join(telegramErr, emailConfigErr)
	}
	if emailEnabled {
		if err := dispatchImmediateEmail(ctx, st, cfg, recipients, alerts, suppressed, now); err != nil {
			return errors.Join(telegramErr, err)
		}
	}
	return telegramErr
}

func dispatchTelegram(ctx context.Context, st *store.Store, sender *TelegramSender, alerts []domain.Alert, suppressed map[string]time.Time, now time.Time) error {
	if sender == nil {
		return nil
	}
	destinations, err := st.ListActiveTelegramDestinations(ctx)
	if err != nil {
		sender.RecordDelivery(err)
		return fmt.Errorf("list active Telegram users: %w", err)
	}
	var deliveryErr error
	attempted := false
	for _, destination := range destinations {
		selected, err := criticalAlertsPendingTelegram(ctx, st, destination.ID, alerts, suppressed)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: %w", destination.Username, err))
			attempted = true
		} else if len(selected) > 0 {
			attempted = true
			if err := sender.SendAlerts(ctx, destination, selected, alertLink(selected), false); err != nil {
				deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: %w", destination.Username, err))
			} else if err := st.MarkAlertCriticalTelegramsSent(ctx, destination.ID, alertIDs(selected), now); err != nil {
				deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: mark critical Telegram sent: %w", destination.Username, err))
			}
		}
		resolved, err := st.ListPendingAlertResolutionTelegrams(ctx, destination.ID)
		if err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: list resolved Telegram alerts: %w", destination.Username, err))
			attempted = true
		} else if len(resolved) > 0 {
			attempted = true
			if err := sender.SendAlerts(ctx, destination, resolved, "/alerts", true); err != nil {
				deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: %w", destination.Username, err))
			} else if err := st.MarkAlertResolutionTelegramsSent(ctx, destination.ID, alertIDs(resolved), now); err != nil {
				deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: mark resolution Telegram sent: %w", destination.Username, err))
			}
		}
	}
	if attempted {
		sender.RecordDelivery(deliveryErr)
	}
	return deliveryErr
}

func dispatchImmediateEmail(ctx context.Context, st *store.Store, cfg *domain.EmailConfig, recipients []string, alerts []domain.Alert, suppressed map[string]time.Time, now time.Time) error {
	selected, err := criticalAlertsPendingEmail(ctx, st, alerts, suppressed)
	if err != nil {
		return err
	}
	resolved, err := st.ListPendingAlertResolutionEmails(ctx)
	if err != nil {
		return fmt.Errorf("list resolved critical alerts: %w", err)
	}
	due, err := st.AlertEmailBatchDue(ctx, cfg.AlertEmailBatchMinutes, len(selected)+len(resolved) > 0, now)
	if err != nil {
		return fmt.Errorf("check email batch: %w", err)
	}
	if !due {
		return nil
	}
	if cfg.AlertEmailBatchMinutes > 0 {
		subject := fmt.Sprintf("Probakgo incidencias: %d activa(s), %d resuelta(s)", len(selected), len(resolved))
		if err := sendSMTP(cfg, recipients, subject, renderAlertDigestEmail(selected, resolved, now)); err != nil {
			return err
		}
		if err := st.MarkAlertCriticalEmailsSent(ctx, alertIDs(selected), now); err != nil {
			return err
		}
		if err := st.MarkAlertResolutionEmailsSent(ctx, alertIDs(resolved), now); err != nil {
			return err
		}
	} else {
		if len(selected) > 0 {
			subject := fmt.Sprintf("Probakgo alerta critica: %d alerta(s) activa(s)", len(selected))
			if err := sendSMTP(cfg, recipients, subject, renderImmediateCriticalEmail(selected, now)); err != nil {
				return err
			}
			if err := st.MarkAlertCriticalEmailsSent(ctx, alertIDs(selected), now); err != nil {
				return err
			}
		}
		if len(resolved) > 0 {
			subject := fmt.Sprintf("Probakgo alerta resuelta: %d alerta(s)", len(resolved))
			if err := sendSMTP(cfg, recipients, subject, renderResolvedCriticalEmail(resolved, now)); err != nil {
				return err
			}
			if err := st.MarkAlertResolutionEmailsSent(ctx, alertIDs(resolved), now); err != nil {
				return err
			}
		}
	}
	return st.ClearAlertEmailBatch(ctx)
}

func dispatchPush(st *store.Store, sender *PushSender, alerts []domain.Alert, linkURL string, resolved bool) {
	batch := append([]domain.Alert(nil), alerts...)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		var delivered int
		if resolved {
			delivered = sender.SendResolutions(ctx, batch, linkURL)
		} else {
			delivered = sender.SendAlerts(ctx, batch, linkURL)
		}
		if delivered == 0 {
			return
		}
		now := time.Now()
		var err error
		if resolved {
			err = st.MarkAlertResolutionPushesSent(ctx, alertIDs(batch), now)
		} else {
			err = st.MarkAlertCriticalPushesSent(ctx, alertIDs(batch), now)
		}
		if err != nil {
			slog.Warn("mark push notification sent", "resolved", resolved, "err", err)
		}
	}()
}

func alertIDs(alerts []domain.Alert) []string {
	ids := make([]string, 0, len(alerts))
	for _, alert := range alerts {
		ids = append(ids, alert.ID)
	}
	return ids
}

func alertLink(alerts []domain.Alert) string {
	if len(alerts) != 1 {
		return "/alerts"
	}
	a := alerts[0]
	switch a.ServerType {
	case "pve", "pbs", "windows":
		if a.ServerID > 0 {
			return "/servers/" + a.ServerType + "/" + strconv.FormatInt(a.ServerID, 10)
		}
	}
	return "/alerts"
}

func criticalAlertsPendingEmail(ctx context.Context, st *store.Store, alerts []domain.Alert, suppressed map[string]time.Time) ([]domain.Alert, error) {
	return criticalAlertsPending(ctx, alerts, suppressed, func(ids []string) (map[string]bool, error) {
		return st.ListCriticalEmailSentAlertIDs(ctx, ids)
	})
}

func criticalAlertsPendingPush(ctx context.Context, st *store.Store, alerts []domain.Alert, suppressed map[string]time.Time) ([]domain.Alert, error) {
	return criticalAlertsPending(ctx, alerts, suppressed, func(ids []string) (map[string]bool, error) {
		return st.ListCriticalPushSentAlertIDs(ctx, ids)
	})
}

func criticalAlertsPendingTelegram(ctx context.Context, st *store.Store, destinationID int64, alerts []domain.Alert, suppressed map[string]time.Time) ([]domain.Alert, error) {
	return criticalAlertsPending(ctx, alerts, suppressed, func(ids []string) (map[string]bool, error) {
		return st.ListCriticalTelegramSentAlertIDs(ctx, destinationID, ids)
	})
}

func criticalAlertsPending(_ context.Context, alerts []domain.Alert, suppressed map[string]time.Time, sentLookup func([]string) (map[string]bool, error)) ([]domain.Alert, error) {
	var candidates []domain.Alert
	for _, alert := range alerts {
		if !shouldSendImmediateCriticalEmail(alert) {
			continue
		}
		if _, ok := suppressed[alert.ID]; ok {
			continue
		}
		candidates = append(candidates, alert)
	}
	sent, err := sentLookup(alertIDs(candidates))
	if err != nil {
		return nil, fmt.Errorf("get critical notification state: %w", err)
	}
	selected := make([]domain.Alert, 0, len(candidates))
	for _, alert := range candidates {
		if !sent[alert.ID] {
			selected = append(selected, alert)
		}
	}
	return selected, nil
}

// Keep both existing styled sections inside a single HTML document.
func renderAlertDigestEmail(active, resolved []domain.Alert, now time.Time) string {
	var b strings.Builder
	b.WriteString(`<!DOCTYPE html><html lang="es"><head><meta charset="UTF-8"><title>Probakgo incidencias</title></head><body style="margin:0;font-family:Arial,sans-serif;background:#f5f5f5">`)
	appendSection := func(document string) {
		start := strings.Index(document, "<body")
		start += strings.Index(document[start:], ">") + 1
		b.WriteString(document[start:strings.LastIndex(document, "</body>")])
	}
	if len(active) > 0 {
		appendSection(renderImmediateCriticalEmail(active, now))
	}
	if len(resolved) > 0 {
		appendSection(renderResolvedCriticalEmail(resolved, now))
	}
	b.WriteString("</body></html>")
	return b.String()
}

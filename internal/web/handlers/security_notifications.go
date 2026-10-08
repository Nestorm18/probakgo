package webhandlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"probakgo/internal/service"
)

type adminSecurityNotifier interface {
	SendAdminSecurityNotification(ctx context.Context, text, linkURL string) error
}

func (h *WebH) notifyAdminLoginSuccess(username, ip string) {
	h.notifyAdminSecurity(
		fmt.Sprintf("🔐 Probakgo: inicio de sesión\n\nUsuario: %s\nIP: %s", securityField(username), securityField(ip)),
		"/settings/ip-bans",
	)
}

// Failed logins come from unauthenticated clients. Notify at most once per IP
// per interval and cap the global rate; skipped notices are summarised later.
const (
	failedLoginNoticeInterval = 10 * time.Minute
	failedLoginNoticeWindow   = time.Hour
	failedLoginNoticeLimit    = 10
	failedLoginNoticeMaxIPs   = 4096
)

type loginNoticeThrottle struct {
	mu         sync.Mutex
	lastByIP   map[string]time.Time
	sent       []time.Time
	suppressed int
}

// allow reports whether a notice for ip may be sent now and how many notices
// were skipped since the last one sent.
func (t *loginNoticeThrottle) allow(ip string, now time.Time) (bool, int) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.lastByIP == nil {
		t.lastByIP = make(map[string]time.Time)
	}
	if len(t.lastByIP) >= failedLoginNoticeMaxIPs {
		for key, last := range t.lastByIP {
			if now.Sub(last) >= failedLoginNoticeInterval {
				delete(t.lastByIP, key)
			}
		}
	}
	recent := t.sent[:0]
	for _, sentAt := range t.sent {
		if now.Sub(sentAt) < failedLoginNoticeWindow {
			recent = append(recent, sentAt)
		}
	}
	t.sent = recent
	if last, ok := t.lastByIP[ip]; ok && now.Sub(last) < failedLoginNoticeInterval {
		t.suppressed++
		return false, 0
	}
	if len(t.sent) >= failedLoginNoticeLimit || len(t.lastByIP) >= failedLoginNoticeMaxIPs {
		t.suppressed++
		return false, 0
	}
	t.lastByIP[ip] = now
	t.sent = append(t.sent, now)
	skipped := t.suppressed
	t.suppressed = 0
	return true, skipped
}

// notifyAdminLoginFailed sends a throttled notice. A new ban is keyed apart
// from ordinary failures so the per-IP interval cannot hide it.
func (h *WebH) notifyAdminLoginFailed(username, ip, reason string, banned bool) {
	if h == nil || h.telegram == nil {
		return
	}
	key := ip
	if banned {
		key = ip + "|ban"
	}
	ok, skipped := h.loginNotices.allow(key, time.Now())
	if !ok {
		return
	}
	message := fmt.Sprintf("⚠️ Probakgo: intento de acceso fallido\n\nUsuario: %s\nIP: %s\nMotivo: %s", securityField(username), securityField(ip), securityField(reason))
	if skipped > 0 {
		message += fmt.Sprintf("\n\nAvisos omitidos desde el anterior: %d", skipped)
	}
	h.notifyAdminSecurity(message, "/settings/ip-bans")
}

func (h *WebH) notifyAdminUserCreated(username, role, actor, ip string, userID int64) {
	h.notifyAdminSecurity(
		fmt.Sprintf("👤 Probakgo: usuario creado\n\nUsuario: %s\nRol: %s\nCreado por: %s\nIP: %s", securityField(username), securityField(role), securityField(actor), securityField(ip)),
		fmt.Sprintf("/users/%d/edit", userID),
	)
}

func (h *WebH) notifyAdminSecurity(message, linkURL string) {
	if h == nil || h.telegram == nil {
		return
	}
	notifier := h.telegram
	// Tracked so shutdown waits for it; its own timeout lets it finish.
	service.Go(func(context.Context) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := notifier.SendAdminSecurityNotification(ctx, message, linkURL); err != nil {
			slog.Warn("send Telegram admin security notification", "err", err)
		}
	})
}

func securityField(value string) string {
	value = strings.Join(strings.Fields(value), " ")
	if value == "" {
		return "(vacío)"
	}
	const maxLength = 160
	runes := []rune(value)
	if len(runes) > maxLength {
		return string(runes[:maxLength]) + "…"
	}
	return value
}

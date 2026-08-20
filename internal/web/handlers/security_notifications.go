package webhandlers

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"
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

func (h *WebH) notifyAdminLoginFailed(username, ip, reason string) {
	h.notifyAdminSecurity(
		fmt.Sprintf("⚠️ Probakgo: intento de acceso fallido\n\nUsuario: %s\nIP: %s\nMotivo: %s", securityField(username), securityField(ip), securityField(reason)),
		"/settings/ip-bans",
	)
}

func (h *WebH) notifyAdminUserCreated(username, role, actor, ip string, userID int64) {
	h.notifyAdminSecurity(
		fmt.Sprintf("👤 Probakgo: usuario creado\n\nUsuario: %s\nRol: %s\nCreado por: %s\nIP: %s", securityField(username), securityField(role), securityField(actor), securityField(ip)),
		fmt.Sprintf("/users/%d", userID),
	)
}

func (h *WebH) notifyAdminSecurity(message, linkURL string) {
	if h == nil || h.telegram == nil {
		return
	}
	notifier := h.telegram
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := notifier.SendAdminSecurityNotification(ctx, message, linkURL); err != nil {
			slog.Warn("send Telegram admin security notification", "err", err)
		}
	}()
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

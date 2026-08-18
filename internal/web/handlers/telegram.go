package webhandlers

import (
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/domain"
	"probakgo/internal/service"
	"probakgo/internal/session"
)

func (h *WebH) TelegramSettings(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	cfg, err := h.store.GetTelegramConfig(ctx)
	if err != nil {
		slog.Error("load Telegram config", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	status, err := h.store.GetTelegramDeliveryStatus(ctx)
	if err != nil {
		slog.Warn("load Telegram delivery status", "err", err)
	}
	destinations, err := h.store.ListTelegramDestinations(ctx)
	if err != nil {
		slog.Warn("load Telegram destinations", "err", err)
	}
	activeDestinations, _ := h.store.ListActiveTelegramDestinations(ctx)
	h.tmpl.Render(w, r, "telegram_settings.html", map[string]any{
		"Username":               username,
		"Role":                   role,
		"Config":                 cfg,
		"Destinations":           destinations,
		"ActiveDestinationCount": len(activeDestinations),
		"Status":                 status,
		"Flash":                  r.URL.Query().Get("flash"),
		"FlashOK":                r.URL.Query().Get("ok") == "1",
		"TokenPresent":           cfg.BotToken != "",
	})
}

func (h *WebH) TelegramSettingsPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	existing, err := h.store.GetTelegramConfig(ctx)
	if err != nil {
		redirectTelegramSettingsError(w, r, "No se pudo leer la configuracion")
		return
	}
	token := strings.TrimSpace(r.FormValue("bot_token"))
	botUsername := existing.BotUsername
	if token == "" {
		token = existing.BotToken
	} else {
		bot, err := h.telegramSender().VerifyToken(ctx, token)
		if err != nil {
			redirectTelegramSettingsError(w, r, err.Error())
			return
		}
		botUsername = bot.Username
	}
	cfg := domain.TelegramConfig{
		BotToken:    token,
		BotUsername: botUsername,
		IsEnabled:   r.FormValue("is_enabled") == "on",
	}
	if cfg.IsEnabled && cfg.BotToken == "" {
		redirectTelegramSettingsError(w, r, "Falta el token del bot")
		return
	}
	tokenChanged := existing.BotToken != cfg.BotToken
	if err := h.store.UpsertTelegramConfig(ctx, cfg); err != nil {
		redirectTelegramSettingsError(w, r, err.Error())
		return
	}
	if tokenChanged {
		if err := h.store.DeleteAllTelegramDestinations(ctx); err != nil {
			slog.Warn("clear Telegram links after bot change", "err", err)
		}
	}
	h.audit(r, "settings.telegram_update", "settings", "telegram", "Telegram", map[string]any{
		"bot_token_set": cfg.BotToken != "",
		"bot_changed":   tokenChanged,
		"is_enabled":    cfg.IsEnabled,
	})
	http.Redirect(w, r, "/settings/telegram?flash=Configuracion+guardada&ok=1", http.StatusSeeOther)
}

func (h *WebH) ProfileTelegramPair(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		redirectTelegramProfileError(w, r, "Usuario no encontrado")
		return
	}
	cfg, err := h.store.GetTelegramConfig(ctx)
	if err != nil || cfg.BotToken == "" {
		redirectTelegramProfileError(w, r, "El administrador debe configurar primero el bot de Telegram")
		return
	}
	code, ok := session.GetTelegramPairing(r, user.ID, time.Now())
	if !ok {
		redirectTelegramProfileError(w, r, "El codigo de vinculacion ha caducado; recarga el perfil")
		return
	}
	chat, err := h.telegramSender().FindPairingChat(ctx, cfg.BotToken, code)
	if err != nil {
		redirectTelegramProfileError(w, r, err.Error())
		return
	}
	newChatID := strconv.FormatInt(chat.ID, 10)
	if err := service.ValidateTelegramChatID(newChatID); err != nil {
		redirectTelegramProfileError(w, r, err.Error())
		return
	}
	if _, err := h.store.UpsertTelegramDestination(ctx, domain.TelegramDestination{
		UserID: user.ID, ChatID: newChatID, ChatTitle: chat.DisplayName(), ChatType: "private",
	}); err != nil {
		message := err.Error()
		if strings.Contains(message, "telegram_destinations.chat_id") {
			message = "Este chat de Telegram ya esta vinculado a otro usuario"
		}
		redirectTelegramProfileError(w, r, message)
		return
	}
	_ = session.ClearTelegramPairing(w, r)
	h.audit(r, "profile.telegram_pair", "user", strconv.FormatInt(user.ID, 10), user.Username, nil)
	http.Redirect(w, r, "/profile?flash="+url.QueryEscape("Telegram vinculado: "+chat.DisplayName())+"&ok=1", http.StatusSeeOther)
}

func (h *WebH) ProfileTelegramTest(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		redirectTelegramProfileError(w, r, "Usuario no encontrado")
		return
	}
	destination, err := h.store.GetTelegramDestinationForUser(ctx, user.ID)
	if err != nil || destination == nil {
		redirectTelegramProfileError(w, r, "No tienes Telegram vinculado")
		return
	}
	if err := h.telegramSender().SendTestToDestination(ctx, *destination); err != nil {
		redirectTelegramProfileError(w, r, err.Error())
		return
	}
	h.audit(r, "profile.telegram_test", "user", strconv.FormatInt(user.ID, 10), user.Username, nil)
	http.Redirect(w, r, "/profile?flash=Notificacion+de+prueba+enviada&ok=1", http.StatusSeeOther)
}

func (h *WebH) ProfileTelegramDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		redirectTelegramProfileError(w, r, "Usuario no encontrado")
		return
	}
	if err := h.store.DeleteTelegramDestinationForUser(ctx, user.ID); err != nil {
		redirectTelegramProfileError(w, r, err.Error())
		return
	}
	_ = session.ClearTelegramPairing(w, r)
	h.audit(r, "profile.telegram_unlink", "user", strconv.FormatInt(user.ID, 10), user.Username, nil)
	http.Redirect(w, r, "/profile?flash=Telegram+desvinculado&ok=1", http.StatusSeeOther)
}

func (h *WebH) UserTelegramDelete(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil || id <= 0 {
		redirectWithFlash(w, r, "/users", "Usuario no valido", false)
		return
	}
	user, err := h.store.GetUser(r.Context(), id)
	if err != nil {
		redirectWithFlash(w, r, "/users", "Usuario no encontrado", false)
		return
	}
	if err := h.store.DeleteTelegramDestinationForUser(r.Context(), id); err != nil {
		redirectWithFlash(w, r, "/users/"+strconv.FormatInt(id, 10)+"/edit", err.Error(), false)
		return
	}
	h.audit(r, "user.telegram_unlink", "user", strconv.FormatInt(id, 10), user.Username, nil)
	back := formBackOrDefault(r, "/users/"+strconv.FormatInt(id, 10)+"/edit")
	redirectWithFlash(w, r, back, "Telegram desvinculado de "+user.Username, true)
}

func (h *WebH) TelegramTest(w http.ResponseWriter, r *http.Request) {
	if err := h.telegramSender().SendTest(r.Context()); err != nil {
		redirectTelegramSettingsError(w, r, err.Error())
		return
	}
	h.audit(r, "settings.telegram_test", "settings", "telegram", "Telegram", nil)
	http.Redirect(w, r, "/settings/telegram?flash=Notificacion+de+prueba+enviada&ok=1", http.StatusSeeOther)
}

func (h *WebH) TelegramDelete(w http.ResponseWriter, r *http.Request) {
	if r.FormValue("confirm") != "delete" {
		redirectTelegramSettingsError(w, r, "Confirma la eliminacion de la configuracion")
		return
	}
	if err := h.store.DeleteTelegramConfig(r.Context()); err != nil {
		redirectTelegramSettingsError(w, r, err.Error())
		return
	}
	_ = session.ClearTelegramPairing(w, r)
	h.audit(r, "settings.telegram_delete", "settings", "telegram", "Telegram", nil)
	http.Redirect(w, r, "/settings/telegram?flash=Configuracion+de+Telegram+eliminada&ok=1", http.StatusSeeOther)
}

func (h *WebH) telegramPairingURL(w http.ResponseWriter, r *http.Request, userID int64, cfg *domain.TelegramConfig) string {
	if cfg == nil || cfg.BotToken == "" || cfg.BotUsername == "" {
		return ""
	}
	code, err := newTelegramPairingCode()
	if err != nil {
		return ""
	}
	if err := session.SetTelegramPairing(w, r, userID, code, time.Now().Add(10*time.Minute)); err != nil {
		return ""
	}
	return "https://t.me/" + cfg.BotUsername + "?start=" + code
}

func (h *WebH) telegramSender() *service.TelegramSender {
	if sender := service.GetTelegramSender(); sender != nil {
		return sender
	}
	return service.NewTelegramSender(h.store)
}

func newTelegramPairingCode() (string, error) {
	raw := make([]byte, 12)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return hex.EncodeToString(raw), nil
}

func redirectTelegramSettingsError(w http.ResponseWriter, r *http.Request, message string) {
	http.Redirect(w, r, "/settings/telegram?flash="+url.QueryEscape(message), http.StatusSeeOther)
}

func redirectTelegramProfileError(w http.ResponseWriter, r *http.Request, message string) {
	http.Redirect(w, r, "/profile?flash="+url.QueryEscape(message), http.StatusSeeOther)
}

package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

const telegramAPIBaseURL = "https://api.telegram.org"

var (
	telegramTokenPattern = regexp.MustCompile(`^[0-9]{5,20}:[A-Za-z0-9_-]{20,128}$`)
	telegramChatPattern  = regexp.MustCompile(`^-?[0-9]{1,20}$|^@[A-Za-z0-9_]{5,64}$`)
)

type TelegramBot struct {
	ID       int64  `json:"id"`
	IsBot    bool   `json:"is_bot"`
	Username string `json:"username"`
}

type TelegramChat struct {
	ID        int64  `json:"id"`
	Type      string `json:"type"`
	Title     string `json:"title"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (c TelegramChat) DisplayName() string {
	if c.Title != "" {
		return c.Title
	}
	if c.Username != "" {
		return "@" + c.Username
	}
	name := strings.TrimSpace(c.FirstName + " " + c.LastName)
	if name != "" {
		return name
	}
	return strconv.FormatInt(c.ID, 10)
}

type TelegramAPIError struct {
	Code        int
	Description string
	RetryAfter  int
}

func (e *TelegramAPIError) Error() string {
	if e.RetryAfter > 0 {
		return fmt.Sprintf("Telegram: %s (reintentar en %ds)", e.Description, e.RetryAfter)
	}
	return "Telegram: " + e.Description
}

type telegramAPIResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter int `json:"retry_after"`
	} `json:"parameters"`
}

type TelegramSender struct {
	st      *store.Store
	client  *http.Client
	baseURL string
}

func NewTelegramSender(st *store.Store) *TelegramSender {
	return &TelegramSender{
		st: st,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
		baseURL: telegramAPIBaseURL,
	}
}

func ValidateTelegramChatID(chatID string) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return errors.New("falta el destino de Telegram")
	}
	if !telegramChatPattern.MatchString(chatID) {
		return errors.New("chat_id de Telegram no valido")
	}
	return nil
}

func validateTelegramToken(token string) error {
	if !telegramTokenPattern.MatchString(strings.TrimSpace(token)) {
		return errors.New("token de Telegram no valido")
	}
	return nil
}

func (s *TelegramSender) Ready(ctx context.Context) bool {
	if s == nil || s.st == nil {
		return false
	}
	cfg, err := s.st.GetTelegramConfig(ctx)
	if err != nil || cfg == nil || !cfg.IsEnabled || validateTelegramToken(cfg.BotToken) != nil {
		return false
	}
	destinations, err := s.st.ListActiveTelegramDestinations(ctx)
	return err == nil && len(destinations) > 0
}

func (s *TelegramSender) VerifyToken(ctx context.Context, token string) (*TelegramBot, error) {
	if err := validateTelegramToken(token); err != nil {
		return nil, err
	}
	var bot TelegramBot
	if err := s.call(ctx, token, "getMe", struct{}{}, &bot); err != nil {
		return nil, err
	}
	if !bot.IsBot || bot.Username == "" {
		return nil, errors.New("Telegram no devolvio una cuenta de bot valida")
	}
	return &bot, nil
}

func (s *TelegramSender) FindPairingChat(ctx context.Context, token, code string) (*TelegramChat, error) {
	if err := validateTelegramToken(token); err != nil {
		return nil, err
	}
	if code == "" {
		return nil, errors.New("el codigo de vinculacion ha caducado")
	}
	var webhook struct {
		URL string `json:"url"`
	}
	if err := s.call(ctx, token, "getWebhookInfo", struct{}{}, &webhook); err != nil {
		return nil, err
	}
	if webhook.URL != "" {
		return nil, errors.New("el bot tiene un webhook configurado; usa un bot dedicado a Probakgo sin webhook")
	}
	var updates []struct {
		Message *struct {
			Text string       `json:"text"`
			Chat TelegramChat `json:"chat"`
		} `json:"message"`
	}
	payload := map[string]any{
		"offset":          -100,
		"limit":           100,
		"timeout":         0,
		"allowed_updates": []string{"message"},
	}
	if err := s.call(ctx, token, "getUpdates", payload, &updates); err != nil {
		return nil, err
	}
	for i := len(updates) - 1; i >= 0; i-- {
		if updates[i].Message == nil {
			continue
		}
		fields := strings.Fields(updates[i].Message.Text)
		isStart := len(fields) == 2 && (fields[0] == "/start" || strings.HasPrefix(fields[0], "/start@"))
		if !isStart || fields[1] != code {
			continue
		}
		chat := updates[i].Message.Chat
		if chat.Type != "private" {
			return nil, errors.New("la vinculacion debe realizarse desde un chat privado con el bot")
		}
		return &chat, nil
	}
	return nil, errors.New("no se encontro el mensaje de vinculacion; abre el bot, pulsa Start y vuelve a intentarlo")
}

func (s *TelegramSender) SendTest(ctx context.Context) error {
	cfg, err := s.st.GetTelegramConfig(ctx)
	if err != nil {
		return fmt.Errorf("leer configuracion de Telegram: %w", err)
	}
	if err := validateTelegramToken(cfg.BotToken); err != nil {
		return err
	}
	destinations, err := s.st.ListActiveTelegramDestinations(ctx)
	if err != nil {
		return fmt.Errorf("leer destinos de Telegram: %w", err)
	}
	if len(destinations) == 0 {
		return errors.New("no hay usuarios activos con Telegram vinculado")
	}
	var deliveryErr error
	for _, destination := range destinations {
		if err := s.sendMessage(ctx, cfg, destination, "✅ Probakgo: notificacion de prueba\n\nLa vinculacion con Telegram funciona correctamente.", "/alerts"); err != nil {
			deliveryErr = errors.Join(deliveryErr, fmt.Errorf("%s: %w", destination.DisplayName(), err))
		}
	}
	s.recordDelivery(deliveryErr)
	return deliveryErr
}

func (s *TelegramSender) SendTestToDestination(ctx context.Context, destination domain.TelegramDestination) error {
	cfg, err := s.st.GetTelegramConfig(ctx)
	if err != nil {
		return fmt.Errorf("leer configuracion de Telegram: %w", err)
	}
	if err := validateTelegramToken(cfg.BotToken); err != nil {
		return err
	}
	if err := ValidateTelegramChatID(destination.ChatID); err != nil {
		return err
	}
	deliveryErr := s.sendMessage(ctx, cfg, destination, "✅ Probakgo: notificacion de prueba\n\nTu cuenta de Telegram esta vinculada correctamente.", "/alerts")
	s.recordDelivery(deliveryErr)
	return deliveryErr
}

func (s *TelegramSender) SendAlerts(ctx context.Context, destination domain.TelegramDestination, alerts []domain.Alert, linkURL string, resolved bool) error {
	if len(alerts) == 0 {
		return nil
	}
	cfg, err := s.st.GetTelegramConfig(ctx)
	if err != nil {
		return fmt.Errorf("leer configuracion de Telegram: %w", err)
	}
	if !cfg.IsEnabled {
		return nil
	}
	if err := validateTelegramToken(cfg.BotToken); err != nil {
		return err
	}
	if err := ValidateTelegramChatID(destination.ChatID); err != nil {
		return err
	}
	err = s.sendMessage(ctx, cfg, destination, buildTelegramAlertMessage(alerts, resolved), linkURL)
	return err
}

func (s *TelegramSender) RecordDelivery(deliveryErr error) {
	s.recordDelivery(deliveryErr)
}

func (s *TelegramSender) sendMessage(ctx context.Context, cfg *domain.TelegramConfig, destination domain.TelegramDestination, text, linkURL string) error {
	payload := map[string]any{
		"chat_id": destination.ChatID,
		"text":    text,
	}
	if linkURL != "" {
		emailCfg, err := s.st.GetEmailConfig(ctx)
		if err == nil && emailCfg.PublicAPIURL != "" {
			url := strings.TrimRight(emailCfg.PublicAPIURL, "/") + "/" + strings.TrimLeft(linkURL, "/")
			payload["reply_markup"] = map[string]any{
				"inline_keyboard": [][]map[string]string{{{
					"text": "Abrir en Probakgo",
					"url":  url,
				}}},
			}
		}
	}
	return s.call(ctx, cfg.BotToken, "sendMessage", payload, nil)
}

func (s *TelegramSender) call(ctx context.Context, token, method string, payload any, result any) error {
	if err := validateTelegramToken(token); err != nil {
		return err
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return errors.New("no se pudo preparar la peticion a Telegram")
	}
	endpoint := strings.TrimRight(s.baseURL, "/") + "/bot" + token + "/" + method
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return errors.New("no se pudo preparar la peticion a Telegram")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := s.client.Do(req)
	if err != nil {
		// A net/http error can contain the request URL, and therefore the bot
		// token. Do not wrap or log it verbatim.
		return errors.New("no se pudo contactar con Telegram")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return errors.New("respuesta de Telegram no valida")
	}
	var apiResp telegramAPIResponse
	if err := json.Unmarshal(raw, &apiResp); err != nil {
		return errors.New("respuesta de Telegram no valida")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || !apiResp.OK {
		description := strings.TrimSpace(strings.ReplaceAll(apiResp.Description, "\n", " "))
		if description == "" {
			description = http.StatusText(resp.StatusCode)
		}
		if len(description) > 300 {
			description = description[:300]
		}
		return &TelegramAPIError{Code: apiResp.ErrorCode, Description: description, RetryAfter: apiResp.Parameters.RetryAfter}
	}
	if result != nil && len(apiResp.Result) > 0 {
		if err := json.Unmarshal(apiResp.Result, result); err != nil {
			return errors.New("resultado de Telegram no valido")
		}
	}
	return nil
}

func (s *TelegramSender) recordDelivery(deliveryErr error) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := s.st.RecordTelegramDelivery(ctx, deliveryErr); err != nil {
		slog.Warn("record Telegram delivery status", "err", err)
	}
}

func buildTelegramAlertMessage(alerts []domain.Alert, resolved bool) string {
	header := fmt.Sprintf("🚨 Probakgo: %d alerta(s) critica(s)", len(alerts))
	if resolved {
		header = fmt.Sprintf("✅ Probakgo: %d alerta(s) resuelta(s)", len(alerts))
	}
	var b strings.Builder
	b.WriteString(header)
	for i, alert := range alerts {
		var entry strings.Builder
		entry.WriteString("\n\n")
		if alert.ServerName != "" {
			entry.WriteString(alert.ServerName)
		} else {
			entry.WriteString("Servidor")
		}
		if alert.ServerType != "" {
			entry.WriteString(" · ")
			entry.WriteString(strings.ToUpper(alert.ServerType))
		}
		entry.WriteString("\n")
		entry.WriteString(alert.Title)
		if alert.Message != "" && alert.Message != alert.Title {
			entry.WriteString("\n")
			entry.WriteString(alert.Message)
		}
		if !resolved && (alert.Value != "" || alert.Threshold != "") {
			entry.WriteString("\n")
			if alert.Value != "" {
				entry.WriteString("Valor: ")
				entry.WriteString(alert.Value)
			}
			if alert.Value != "" && alert.Threshold != "" {
				entry.WriteString(" · ")
			}
			if alert.Threshold != "" {
				entry.WriteString("Umbral: ")
				entry.WriteString(alert.Threshold)
			}
		}
		candidate := entry.String()
		if utf8.RuneCountInString(b.String())+utf8.RuneCountInString(candidate) > 3800 {
			b.WriteString(fmt.Sprintf("\n\n… y %d alerta(s) mas", len(alerts)-i))
			break
		}
		b.WriteString(candidate)
	}
	return b.String()
}

var telegramSender atomic.Pointer[TelegramSender]

func SetTelegramSender(sender *TelegramSender) { telegramSender.Store(sender) }

func GetTelegramSender() *TelegramSender { return telegramSender.Load() }

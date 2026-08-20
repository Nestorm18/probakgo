package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"unicode/utf8"

	"probakgo/internal/domain"
)

const testTelegramToken = "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi"

func TestTelegramVerifyTokenAndPair(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/getMe"):
			writeTelegramTestResponse(t, w, map[string]any{"id": 1, "is_bot": true, "username": "probakgo_test_bot"})
		case strings.HasSuffix(r.URL.Path, "/getWebhookInfo"):
			writeTelegramTestResponse(t, w, map[string]any{"url": ""})
		case strings.HasSuffix(r.URL.Path, "/getUpdates"):
			writeTelegramTestResponse(t, w, []map[string]any{{
				"update_id": 4,
				"message": map[string]any{
					"text": "/start pair-code",
					"chat": map[string]any{"id": int64(987654321), "type": "private", "first_name": "Nestor"},
				},
			}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	_, st := openTestStore(t)
	sender := NewTelegramSender(st)
	sender.baseURL = server.URL
	sender.client = server.Client()
	bot, err := sender.VerifyToken(context.Background(), testTelegramToken)
	if err != nil || bot.Username != "probakgo_test_bot" {
		t.Fatalf("VerifyToken: bot=%+v err=%v", bot, err)
	}
	chat, err := sender.FindPairingChat(context.Background(), testTelegramToken, "pair-code")
	if err != nil || chat.ID != 987654321 || chat.DisplayName() != "Nestor" {
		t.Fatalf("FindPairingChat: chat=%+v err=%v", chat, err)
	}
}

func TestTelegramSendTestAndAlertPayload(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode Telegram payload: %v", err)
		}
		requests = append(requests, payload)
		writeTelegramTestResponse(t, w, map[string]any{"message_id": 1})
	}))
	defer server.Close()
	_, st := openTestStore(t)
	if err := st.UpsertTelegramConfig(context.Background(), domain.TelegramConfig{
		BotToken: testTelegramToken, IsEnabled: true,
	}); err != nil {
		t.Fatalf("save Telegram config: %v", err)
	}
	firstUserID, _ := st.CreateUser(context.Background(), "alice", "hash", "reader")
	secondUserID, _ := st.CreateUser(context.Background(), "bob", "hash", "reader")
	first := domain.TelegramDestination{UserID: firstUserID, ChatID: "123456789", ChatTitle: "Nestor", ChatType: "private"}
	second := domain.TelegramDestination{UserID: secondUserID, ChatID: "987654321", ChatTitle: "Ops", ChatType: "private"}
	if _, err := st.UpsertTelegramDestination(context.Background(), first); err != nil {
		t.Fatalf("save first Telegram destination: %v", err)
	}
	if _, err := st.UpsertTelegramDestination(context.Background(), second); err != nil {
		t.Fatalf("save second Telegram destination: %v", err)
	}
	emailCfg, _ := st.GetEmailConfig(context.Background())
	emailCfg.PublicAPIURL = "https://probakgo.example"
	if err := st.UpsertEmailConfig(context.Background(), *emailCfg); err != nil {
		t.Fatalf("save public URL: %v", err)
	}
	sender := NewTelegramSender(st)
	sender.baseURL = server.URL
	sender.client = server.Client()
	if err := sender.SendTest(context.Background()); err != nil {
		t.Fatalf("SendTest: %v", err)
	}
	alert := domain.Alert{ID: "disk:pve:1", ServerName: "pve-1", ServerType: "pve", Title: "Disco lleno", Message: "Uso al 95%", Value: "95%", Threshold: "85%"}
	if err := sender.SendAlerts(context.Background(), first, []domain.Alert{alert}, "/servers/pve/1", false); err != nil {
		t.Fatalf("SendAlerts: %v", err)
	}
	if len(requests) != 3 {
		t.Fatalf("requests: got %d, want 3", len(requests))
	}
	if requests[0]["chat_id"] != first.ChatID || requests[1]["chat_id"] != second.ChatID {
		t.Fatalf("test notification did not fan out: %+v", requests)
	}
	text, _ := requests[2]["text"].(string)
	if !strings.Contains(text, "pve-1") || !strings.Contains(text, "95%") {
		t.Fatalf("alert payload missing detail: %q", text)
	}
	if _, ok := requests[2]["reply_markup"]; !ok {
		t.Fatal("alert payload missing Probakgo link button")
	}
	status, err := st.GetTelegramDeliveryStatus(context.Background())
	if err != nil || status == nil || status.LastSuccessAt == nil || status.LastError != "" {
		t.Fatalf("delivery status: status=%+v err=%v", status, err)
	}
}

func TestTelegramSecurityNotificationOnlyReachesActiveAdmins(t *testing.T) {
	var requests []map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode Telegram payload: %v", err)
		}
		requests = append(requests, payload)
		writeTelegramTestResponse(t, w, map[string]any{"message_id": 1})
	}))
	defer server.Close()

	_, st := openTestStore(t)
	ctx := context.Background()
	if err := st.UpsertTelegramConfig(ctx, domain.TelegramConfig{BotToken: testTelegramToken, IsEnabled: true}); err != nil {
		t.Fatalf("save Telegram config: %v", err)
	}
	adminID, _ := st.CreateUser(ctx, "admin", "hash", "admin")
	readerID, _ := st.CreateUser(ctx, "reader", "hash", "reader")
	inactiveAdminID, _ := st.CreateUser(ctx, "inactive-admin", "hash", "admin")
	_ = st.SetUserActive(ctx, inactiveAdminID, false)
	for _, destination := range []domain.TelegramDestination{
		{UserID: adminID, ChatID: "111111111", ChatTitle: "Admin"},
		{UserID: readerID, ChatID: "222222222", ChatTitle: "Reader"},
		{UserID: inactiveAdminID, ChatID: "333333333", ChatTitle: "Inactive"},
	} {
		if _, err := st.UpsertTelegramDestination(ctx, destination); err != nil {
			t.Fatalf("save Telegram destination: %v", err)
		}
	}
	emailCfg, _ := st.GetEmailConfig(ctx)
	emailCfg.PublicAPIURL = "https://probakgo.example"
	if err := st.UpsertEmailConfig(ctx, *emailCfg); err != nil {
		t.Fatalf("save public URL: %v", err)
	}

	sender := NewTelegramSender(st)
	sender.baseURL = server.URL
	sender.client = server.Client()
	if err := sender.SendAdminSecurityNotification(ctx, "🔐 Inicio de sesión", "/settings/ip-bans"); err != nil {
		t.Fatalf("SendAdminSecurityNotification: %v", err)
	}
	if len(requests) != 1 || requests[0]["chat_id"] != "111111111" {
		t.Fatalf("security recipients: %+v", requests)
	}
	if requests[0]["text"] != "🔐 Inicio de sesión" {
		t.Fatalf("security message: %+v", requests[0])
	}
	if _, ok := requests[0]["reply_markup"]; !ok {
		t.Fatal("security notification is missing the Probakgo link")
	}
}

func TestTelegramAPIErrorDoesNotExposeToken(t *testing.T) {
	_, st := openTestStore(t)
	sender := NewTelegramSender(st)
	sender.baseURL = "http://127.0.0.1:1"
	err := sender.call(context.Background(), testTelegramToken, "getMe", struct{}{}, nil)
	if err == nil {
		t.Fatal("expected transport error")
	}
	if strings.Contains(err.Error(), testTelegramToken) {
		t.Fatalf("error exposed token: %v", err)
	}
}

func TestBuildTelegramAlertMessageStaysWithinLimit(t *testing.T) {
	alerts := make([]domain.Alert, 100)
	for i := range alerts {
		alerts[i] = domain.Alert{ServerName: strings.Repeat("server", 20), ServerType: "windows", Title: strings.Repeat("problem", 30), Message: strings.Repeat("detail", 60)}
	}
	message := buildTelegramAlertMessage(alerts, false)
	if got := utf8.RuneCountInString(message); got > 4096 {
		t.Fatalf("message has %d runes", got)
	}
	if !strings.Contains(message, "… y ") {
		t.Fatal("truncated batch does not report omitted alerts")
	}
}

func writeTelegramTestResponse(t *testing.T, w http.ResponseWriter, result any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(map[string]any{"ok": true, "result": result}); err != nil {
		t.Fatalf("encode Telegram response: %v", err)
	}
}

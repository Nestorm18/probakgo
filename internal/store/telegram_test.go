package store

import (
	"context"
	"strings"
	"testing"
	"time"

	"probakgo/internal/domain"
)

func TestTelegramConfigEncryptedRoundTrip(t *testing.T) {
	plain := openTestDB(t)
	encrypted, err := NewEncrypted(plain.db, "0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("NewEncrypted: %v", err)
	}
	want := domain.TelegramConfig{
		BotToken:    "123456789:ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghi",
		BotUsername: "probakgo_test_bot",
		IsEnabled:   true,
	}
	if err := encrypted.UpsertTelegramConfig(context.Background(), want); err != nil {
		t.Fatalf("UpsertTelegramConfig: %v", err)
	}
	var stored string
	if err := plain.db.QueryRow(`SELECT bot_token FROM telegram_config WHERE id = 1`).Scan(&stored); err != nil {
		t.Fatalf("read stored token: %v", err)
	}
	if stored == want.BotToken || !strings.HasPrefix(stored, "enc:v1:") {
		t.Fatalf("Telegram token not encrypted: %q", stored)
	}
	got, err := encrypted.GetTelegramConfig(context.Background())
	if err != nil {
		t.Fatalf("GetTelegramConfig: %v", err)
	}
	if got.BotToken != want.BotToken || got.BotUsername != want.BotUsername || !got.IsEnabled {
		t.Fatalf("Telegram round trip: got %+v", got)
	}
}

func TestTelegramDeliveryStateIsIndependent(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	firstUserID, err := st.CreateUser(ctx, "nestor", "hash", "admin")
	if err != nil {
		t.Fatalf("create first user: %v", err)
	}
	secondUserID, err := st.CreateUser(ctx, "operaciones", "hash", "reader")
	if err != nil {
		t.Fatalf("create second user: %v", err)
	}
	firstID, err := st.UpsertTelegramDestination(ctx, domain.TelegramDestination{
		UserID: firstUserID, ChatID: "123456789", ChatTitle: "Nestor", ChatType: "private",
	})
	if err != nil {
		t.Fatalf("save first destination: %v", err)
	}
	secondID, err := st.UpsertTelegramDestination(ctx, domain.TelegramDestination{
		UserID: secondUserID, ChatID: "987654321", ChatTitle: "Operaciones", ChatType: "private",
	})
	if err != nil {
		t.Fatalf("save second destination: %v", err)
	}
	destinations, err := st.ListTelegramDestinations(ctx)
	if err != nil || len(destinations) != 2 {
		t.Fatalf("list destinations: got=%+v err=%v", destinations, err)
	}
	if destinations[0].Username != "nestor" || !destinations[0].UserIsActive {
		t.Fatalf("destination ownership not loaded: %+v", destinations[0])
	}
	if err := st.SetUserActive(ctx, secondUserID, false); err != nil {
		t.Fatalf("disable second user: %v", err)
	}
	active, err := st.ListActiveTelegramDestinations(ctx)
	if err != nil || len(active) != 1 || active[0].UserID != firstUserID {
		t.Fatalf("inactive Telegram user was not filtered: got=%+v err=%v", active, err)
	}
	admins, err := st.ListActiveAdminTelegramDestinations(ctx)
	if err != nil || len(admins) != 1 || admins[0].UserID != firstUserID {
		t.Fatalf("active admin Telegram users were not filtered: got=%+v err=%v", admins, err)
	}
	alert := domain.Alert{
		ID: "disk:pve:42", Severity: domain.AlertSeverityCritical, Title: "Disk full", ServerType: "pve", ServerID: 42,
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatalf("sync active alert: %v", err)
	}
	if err := st.MarkAlertCriticalTelegramsSent(ctx, firstID, []string{alert.ID}, time.Now()); err != nil {
		t.Fatalf("mark Telegram sent: %v", err)
	}
	firstSent, err := st.ListCriticalTelegramSentAlertIDs(ctx, firstID, []string{alert.ID})
	if err != nil || !firstSent[alert.ID] {
		t.Fatalf("first destination state: sent=%v err=%v", firstSent, err)
	}
	secondSent, err := st.ListCriticalTelegramSentAlertIDs(ctx, secondID, []string{alert.ID})
	if err != nil || secondSent[alert.ID] {
		t.Fatalf("second destination state crossed over: sent=%v err=%v", secondSent, err)
	}
	emailSent, err := st.ListCriticalEmailSentAlertIDs(ctx, []string{alert.ID})
	if err != nil {
		t.Fatalf("list email state: %v", err)
	}
	pushSent, err := st.ListCriticalPushSentAlertIDs(ctx, []string{alert.ID})
	if err != nil {
		t.Fatalf("list push state: %v", err)
	}
	if emailSent[alert.ID] || pushSent[alert.ID] {
		t.Fatal("marking Telegram sent crossed into another channel")
	}
	if err := st.SyncAlertStates(ctx, nil); err != nil {
		t.Fatalf("sync resolved alert: %v", err)
	}
	telegramResolved, err := st.ListPendingAlertResolutionTelegrams(ctx, firstID)
	if err != nil {
		t.Fatalf("list Telegram resolutions: %v", err)
	}
	secondResolved, err := st.ListPendingAlertResolutionTelegrams(ctx, secondID)
	if err != nil {
		t.Fatalf("list second Telegram resolutions: %v", err)
	}
	emailResolved, _ := st.ListPendingAlertResolutionEmails(ctx)
	pushResolved, _ := st.ListPendingAlertResolutionPushes(ctx)
	if len(telegramResolved) != 1 || len(secondResolved) != 0 || len(emailResolved) != 0 || len(pushResolved) != 0 {
		t.Fatalf("resolution state crossed destinations/channels: first=%d second=%d email=%d push=%d", len(telegramResolved), len(secondResolved), len(emailResolved), len(pushResolved))
	}
	if err := st.MarkAlertResolutionTelegramsSent(ctx, firstID, []string{alert.ID}, time.Now()); err != nil {
		t.Fatalf("mark Telegram resolution sent: %v", err)
	}
	telegramResolved, _ = st.ListPendingAlertResolutionTelegrams(ctx, firstID)
	if len(telegramResolved) != 0 {
		t.Fatal("Telegram resolution remained pending")
	}
	if err := st.SyncAlertStates(ctx, []domain.Alert{alert}); err != nil {
		t.Fatalf("sync recurring alert: %v", err)
	}
	firstSent, _ = st.ListCriticalTelegramSentAlertIDs(ctx, firstID, []string{alert.ID})
	if firstSent[alert.ID] {
		t.Fatal("recurring alert retained the previous delivery state")
	}
}

func TestTelegramDestinationBelongsToOneUserAndCascadesOnDelete(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	aliceID, _ := st.CreateUser(ctx, "alice", "hash", "reader")
	bobID, _ := st.CreateUser(ctx, "bob", "hash", "reader")
	if _, err := st.UpsertTelegramDestination(ctx, domain.TelegramDestination{
		UserID: aliceID, ChatID: "123456789", ChatTitle: "Alice",
	}); err != nil {
		t.Fatalf("link Alice: %v", err)
	}
	if _, err := st.UpsertTelegramDestination(ctx, domain.TelegramDestination{
		UserID: bobID, ChatID: "123456789", ChatTitle: "Alice again",
	}); err == nil {
		t.Fatal("the same Telegram chat was linked to two users")
	}
	if err := st.DeleteUser(ctx, aliceID); err != nil {
		t.Fatalf("delete Alice: %v", err)
	}
	destination, err := st.GetTelegramDestinationForUser(ctx, aliceID)
	if err != nil || destination != nil {
		t.Fatalf("Telegram link survived user deletion: destination=%+v err=%v", destination, err)
	}
}

func TestRecordTelegramDelivery(t *testing.T) {
	ctx := context.Background()
	st := openTestDB(t)
	if err := st.RecordTelegramDelivery(ctx, context.DeadlineExceeded); err != nil {
		t.Fatalf("record failure: %v", err)
	}
	status, err := st.GetTelegramDeliveryStatus(ctx)
	if err != nil || status == nil || status.LastAttemptAt == nil || status.LastError == "" {
		t.Fatalf("failed status: status=%+v err=%v", status, err)
	}
	if err := st.RecordTelegramDelivery(ctx, nil); err != nil {
		t.Fatalf("record success: %v", err)
	}
	status, err = st.GetTelegramDeliveryStatus(ctx)
	if err != nil || status.LastSuccessAt == nil || status.LastError != "" {
		t.Fatalf("success status: status=%+v err=%v", status, err)
	}
}

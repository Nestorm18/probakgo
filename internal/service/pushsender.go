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
	"strings"
	"sync"
	"sync/atomic"
	"time"

	webpush "github.com/SherClockHolmes/webpush-go"

	"probakgo/internal/domain"
	"probakgo/internal/store"
)

// PushNotification is the JSON payload the service worker receives and turns
// into a native OS notification. The keys are intentionally small so the
// notification stays readable on phones and Windows toasts.
type PushNotification struct {
	Title    string `json:"title"`
	Body     string `json:"body"`
	Tag      string `json:"tag,omitempty"`
	Icon     string `json:"icon,omitempty"`
	URL      string `json:"url,omitempty"`
	Severity string `json:"severity,omitempty"`
}

// PushSender is the runtime instance that signs Web Push messages. It is safe
// to call Send* methods concurrently: the underlying webpush library creates
// a fresh HTTP request per call.
type PushSender struct {
	st     *store.Store
	getter func(context.Context) *store.PushConfig
	// deliverFn is the per-subscription HTTP sender. It defaults to a real
	// Web Push call against the configured VAPID keys. Tests override it to
	// simulate expired endpoints without hitting a real push service.
	deliverFn func(ctx context.Context, cfg *store.PushConfig, sub store.PushSubscription, payload []byte) error
	keyMu     sync.Mutex
}

// NewPushSender wires a PushSender to the store. The getter is called on every
// send so the operator can rotate VAPID keys by simply upserting a new pair
// into push_config without restarting the server.
func NewPushSender(st *store.Store) *PushSender {
	p := &PushSender{
		st: st,
		getter: func(ctx context.Context) *store.PushConfig {
			cfg, err := st.GetPushConfig(ctx)
			if err != nil {
				slog.Warn("push config load", "err", err)
				return &store.PushConfig{}
			}
			return cfg
		},
	}
	p.deliverFn = p.realDeliver
	return p
}

// Ready reports whether there is both a usable VAPID pair and at least one
// subscription to receive a notification. It lets the alert path avoid a full
// evaluation when email is disabled and nobody has opted in to push.
func (p *PushSender) Ready(ctx context.Context) bool {
	cfg := p.getter(ctx)
	if cfg == nil || cfg.PublicKey == "" || cfg.PrivateKey == "" {
		return false
	}
	count, err := p.st.CountPushSubscriptions(ctx)
	if err != nil {
		slog.Warn("push: count subscriptions", "err", err)
		return false
	}
	return count > 0
}

// EnsureVAPIDKeys returns the current VAPID key pair, generating a fresh one
// the first time it is called. The private key is stored encrypted with the
// existing secretbox; the public key is returned in the same form the browser
// will use (uncompressed P-256 point, base64url-encoded, no padding).
func (p *PushSender) EnsureVAPIDKeys(ctx context.Context, subject string) (publicKey, privateKey string, err error) {
	p.keyMu.Lock()
	defer p.keyMu.Unlock()

	cfg, err := p.st.GetPushConfig(ctx)
	if err != nil {
		return "", "", fmt.Errorf("load push config: %w", err)
	}
	if cfg.PublicKey != "" && cfg.PrivateKey != "" {
		return cfg.PublicKey, cfg.PrivateKey, nil
	}
	priv, pub, err := webpush.GenerateVAPIDKeys()
	if err != nil {
		return "", "", fmt.Errorf("generate VAPID keys: %w", err)
	}
	if subject == "" {
		subject = cfg.Subject
	}
	if subject == "" {
		subject = "mailto:admin@example.com"
	}
	if err := p.st.UpsertPushConfig(ctx, store.PushConfig{
		PublicKey:  pub,
		PrivateKey: priv,
		Subject:    subject,
	}); err != nil {
		return "", "", fmt.Errorf("persist VAPID keys: %w", err)
	}
	slog.Info("VAPID keys generated for the first time")
	return pub, priv, nil
}

// SendAlerts fans the given alerts out to every active push subscription. The
// function never returns an error to the caller: it logs individual failures
// (expired endpoints, network blips) and prunes endpoints that the push
// service has marked as gone (HTTP 404 / 410).
func (p *PushSender) SendAlerts(ctx context.Context, alerts []domain.Alert, linkURL string) int {
	return p.send(ctx, alerts, linkURL, false)
}

// SendResolutions sends replacement notifications for alerts that have
// cleared. Their tag matches the active alert so supporting browsers replace
// the persistent critical toast instead of leaving stale warnings behind.
func (p *PushSender) SendResolutions(ctx context.Context, alerts []domain.Alert, linkURL string) int {
	return p.send(ctx, alerts, linkURL, true)
}

func (p *PushSender) send(ctx context.Context, alerts []domain.Alert, linkURL string, resolved bool) int {
	if len(alerts) == 0 {
		return 0
	}
	cfg := p.getter(ctx)
	if cfg == nil || cfg.PublicKey == "" || cfg.PrivateKey == "" {
		return 0 // push not configured yet, nothing to do
	}
	subs, err := p.st.ListAllPushSubscriptions(ctx)
	if err != nil {
		slog.Warn("push: list subscriptions", "err", err)
		return 0
	}
	if len(subs) == 0 {
		return 0
	}

	payload := buildPushPayload(alerts, linkURL, resolved)
	var delivered atomic.Int64
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)
	for _, sub := range subs {
		select {
		case sem <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return int(delivered.Load())
		}
		wg.Add(1)
		go func(sub store.PushSubscription) {
			defer wg.Done()
			defer func() { <-sem }()
			if err := p.deliverFn(ctx, cfg, sub, payload); err != nil {
				slog.Warn("push: deliver", "endpoint", redactEndpoint(sub.Endpoint), "err", err)
				p.pruneIfGone(ctx, sub, err)
				return
			}
			delivered.Add(1)
		}(sub)
	}
	wg.Wait()
	return int(delivered.Load())
}

// SendTest sends a one-off "this is a test" notification to a single
// subscription. The handler uses it to confirm the SW registration works
// before the operator leaves the settings page.
func (p *PushSender) SendTest(ctx context.Context, sub store.PushSubscription, linkURL string) error {
	cfg := p.getter(ctx)
	if cfg == nil || cfg.PublicKey == "" || cfg.PrivateKey == "" {
		return errors.New("push no configurado en el servidor")
	}
	payload := buildPushPayload([]domain.Alert{{
		ID:         "test:push",
		Title:      "Probakgo: notificacion de prueba",
		Message:    "Si ves este mensaje, las alertas del escritorio funcionan correctamente.",
		Severity:   "info",
		DetectedAt: time.Now(),
	}}, linkURL, false)
	err := p.deliverFn(ctx, cfg, sub, payload)
	if err != nil {
		p.pruneIfGone(ctx, sub, err)
	}
	return err
}

func (p *PushSender) realDeliver(ctx context.Context, cfg *store.PushConfig, sub store.PushSubscription, payload []byte) error {
	s := &webpush.Subscription{
		Endpoint: sub.Endpoint,
		Keys: webpush.Keys{
			P256dh: sub.P256DH,
			Auth:   sub.Auth,
		},
	}
	opts := &webpush.Options{
		VAPIDPrivateKey: cfg.PrivateKey,
		VAPIDPublicKey:  cfg.PublicKey,
		Topic:           "probakgo-alerts",
		TTL:             3600,
		Urgency:         webpush.UrgencyHigh,
	}
	if cfg.Subject != "" {
		opts.Subscriber = cfg.Subject
	}
	// webpush.SendNotificationWithContext is the context-aware variant; this
	// is the only way to make a stalled push service respect the caller's
	// deadline instead of blocking the alert delivery forever.
	resp, err := webpush.SendNotificationWithContext(ctx, payload, s, opts)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4<<10))
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	// Surface 4xx/5xx so the caller can decide whether to prune.
	return pushHTTPError{status: resp.StatusCode}
}

type pushHTTPError struct{ status int }

func (e pushHTTPError) Error() string { return fmt.Sprintf("push service returned %d", e.status) }

// pruneIfGone removes the subscription when the push service tells us the
// endpoint is no longer valid (404) or the user revoked it (410). Other
// failures are left for the next attempt.
func (p *PushSender) pruneIfGone(ctx context.Context, sub store.PushSubscription, err error) {
	var httpErr pushHTTPError
	if !errors.As(err, &httpErr) || (httpErr.status != http.StatusNotFound && httpErr.status != http.StatusGone) {
		return
	}
	if delErr := p.st.RemovePushSubscription(ctx, sub.UserID, sub.Endpoint); delErr != nil {
		slog.Warn("push: prune expired subscription", "err", delErr)
		return
	}
	slog.Info("push: pruned expired subscription", "endpoint", redactEndpoint(sub.Endpoint), "user_id", sub.UserID)
}

func buildPushPayload(alerts []domain.Alert, linkURL string, resolved bool) []byte {
	title := "Probakgo"
	body := fmt.Sprintf("%d alerta(s) activa(s)", len(alerts))
	severity := "info"
	if len(alerts) == 1 {
		a := alerts[0]
		if a.ServerName != "" {
			title = a.ServerName
		} else {
			title = a.Title
		}
		body = a.Message
		if body == "" {
			body = a.Title
		}
		severity = a.Severity
	} else {
		critical := 0
		for _, a := range alerts {
			if a.Severity == domain.AlertSeverityCritical {
				critical++
			}
		}
		if critical > 0 {
			title = fmt.Sprintf("%d alerta(s) critica(s)", critical)
		} else {
			title = fmt.Sprintf("%d alerta(s) activa(s)", len(alerts))
		}
	}
	if resolved {
		severity = "info"
		if len(alerts) == 1 {
			title = alerts[0].ServerName
			if title == "" {
				title = "Probakgo"
			}
			body = "Alerta resuelta: " + alerts[0].Title
		} else {
			title = fmt.Sprintf("%d alerta(s) resuelta(s)", len(alerts))
			body = "Las condiciones criticas han vuelto a la normalidad"
		}
	}
	tag := ""
	if len(alerts) == 1 {
		tag = alerts[0].ID
	} else {
		tag = fmt.Sprintf("batch:%d", time.Now().Unix())
	}
	notif := PushNotification{
		Title:    title,
		Body:     body,
		Tag:      tag,
		Icon:     "/static/icons/icon-192.png",
		URL:      linkURL,
		Severity: severity,
	}
	buf, err := json.Marshal(notif)
	if err != nil {
		// Marshalling our own struct should never fail, but fall back to a
		// minimal payload instead of panicking.
		return []byte(`{"title":"Probakgo","body":"Nueva alerta"}`)
	}
	return bytes.TrimSpace(buf)
}

func redactEndpoint(endpoint string) string {
	if idx := strings.Index(endpoint, "?"); idx > 0 {
		return endpoint[:idx]
	}
	if len(endpoint) > 64 {
		return endpoint[:64] + "..."
	}
	return endpoint
}

// pushSender is the process-wide sender wired up by main.go. Stored as a
// package var so the existing free-function call sites (SendImmediateCriticalAlerts)
// can pick it up without breaking their public signatures.
var pushSender atomic.Pointer[PushSender]

// SetPushSender registers the sender that immediate critical alerts will be
// delivered through in parallel to the email transport. Passing nil disables
// push (the default during tests).
func SetPushSender(s *PushSender) { pushSender.Store(s) }

// GetPushSender returns the sender registered with SetPushSender, or nil.
func GetPushSender() *PushSender { return pushSender.Load() }

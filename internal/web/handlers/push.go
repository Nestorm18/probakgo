package webhandlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"probakgo/internal/service"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

// pushSubscriptionPayload is the JSON body the browser hands us when the
// service worker successfully subscribes via pushManager.subscribe().
type pushSubscriptionPayload struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256DH string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
}

// VAPIDPublicKey returns the public VAPID key the browser needs to subscribe.
// The key is public by definition, but this handler remains behind login
// because first access lazily creates and persists the server key pair.
func (h *WebH) VAPIDPublicKey(w http.ResponseWriter, r *http.Request) {
	sender := service.GetPushSender()
	if sender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "push no configurado")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	pub, _, err := sender.EnsureVAPIDKeys(ctx, "")
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "no se pudieron generar las claves VAPID")
		return
	}
	serveJSON(w, map[string]string{"vapid_public_key": pub})
}

// PushSubscribe stores the subscription that the browser just created. It
// also stamps the requesting user as the owner so the unsubscribe endpoint
// can verify ownership.
func (h *WebH) PushSubscribe(w http.ResponseWriter, r *http.Request) {
	username, _, ok := session.GetUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "usuario no encontrado")
		return
	}

	var body pushSubscriptionPayload
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	if body.Endpoint == "" || body.Keys.P256DH == "" || body.Keys.Auth == "" {
		writeJSONError(w, http.StatusBadRequest, "suscripcion incompleta")
		return
	}

	sub := store.PushSubscription{
		UserID:    user.ID,
		Endpoint:  body.Endpoint,
		P256DH:    body.Keys.P256DH,
		Auth:      body.Keys.Auth,
		UserAgent: r.UserAgent(),
	}
	if _, err := h.store.AddPushSubscription(r.Context(), sub); err != nil {
		if errors.Is(err, store.ErrInvalidPushSubscription) {
			writeJSONError(w, http.StatusBadRequest, "suscripcion invalida")
			return
		}
		writeJSONError(w, http.StatusInternalServerError, "no se pudo guardar la suscripcion")
		return
	}
	h.audit(r, "push.subscribe", "user", username, username, nil)
	serveJSON(w, map[string]string{"status": "ok"})
}

// PushUnsubscribe removes the subscription that matches the supplied
// endpoint. The endpoint must belong to the requesting user; this prevents
// a logged-out attacker from wiping someone else's subscription.
func (h *WebH) PushUnsubscribe(w http.ResponseWriter, r *http.Request) {
	username, _, ok := session.GetUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "usuario no encontrado")
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	endpoint := strings.TrimSpace(body.Endpoint)
	if endpoint == "" {
		writeJSONError(w, http.StatusBadRequest, "endpoint requerido")
		return
	}
	if err := h.store.RemovePushSubscription(r.Context(), user.ID, endpoint); err != nil {
		writeJSONError(w, http.StatusInternalServerError, "no se pudo eliminar")
		return
	}
	h.audit(r, "push.unsubscribe", "user", username, username, nil)
	serveJSON(w, map[string]string{"status": "ok"})
}

// PushSubscriptions lists the endpoints the current user has subscribed from.
// The browser uses this to render the "you are subscribed from these devices"
// panel on the profile page.
func (h *WebH) PushSubscriptions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	username, _, ok := session.GetUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "usuario no encontrado")
		return
	}
	subs, err := h.store.ListPushSubscriptionsByUser(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "no se pudo listar")
		return
	}
	out := make([]map[string]any, 0, len(subs))
	for _, s := range subs {
		out = append(out, map[string]any{
			"endpoint":     s.Endpoint,
			"user_agent":   s.UserAgent,
			"created_at":   s.CreatedAt,
			"last_seen_at": s.LastSeenAt,
		})
	}
	serveJSON(w, map[string]any{"subscriptions": out})
}

// PushTest sends a "this is a test" notification to the subscription whose
// endpoint the browser reports. The browser calls this immediately after a
// successful subscribe so the user can confirm the toast appears before
// closing the settings tab.
func (h *WebH) PushTest(w http.ResponseWriter, r *http.Request) {
	username, _, ok := session.GetUser(r)
	if !ok {
		writeJSONError(w, http.StatusUnauthorized, "no autenticado")
		return
	}
	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "usuario no encontrado")
		return
	}
	var body struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<10)).Decode(&body); err != nil {
		writeJSONError(w, http.StatusBadRequest, "JSON invalido")
		return
	}
	endpoint := strings.TrimSpace(body.Endpoint)
	if endpoint == "" {
		writeJSONError(w, http.StatusBadRequest, "endpoint requerido")
		return
	}
	sender := service.GetPushSender()
	if sender == nil {
		writeJSONError(w, http.StatusServiceUnavailable, "push no configurado")
		return
	}
	subs, err := h.store.ListPushSubscriptionsByUser(r.Context(), user.ID)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "no se pudo listar")
		return
	}
	var target *store.PushSubscription
	for i := range subs {
		if subs[i].Endpoint == endpoint {
			target = &subs[i]
			break
		}
	}
	if target == nil {
		writeJSONError(w, http.StatusNotFound, "suscripcion no encontrada")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	if err := sender.SendTest(ctx, *target, "/profile"); err != nil {
		writeJSONError(w, http.StatusBadGateway, "el servicio push rechazo el envio")
		return
	}
	serveJSON(w, map[string]string{"status": "sent"})
}

func writeJSONError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

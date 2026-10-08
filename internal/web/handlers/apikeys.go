package webhandlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"probakgo/internal/service"
	"probakgo/internal/session"
)

const apiKeysPageSize = 25

func (h *WebH) APIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil {
		slog.Error("get current user for api keys", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	keys, err := h.store.ListAPIKeysPage(ctx, apiKeysPageSize+1, (page-1)*apiKeysPageSize, query)
	if err != nil {
		slog.Error("list api keys", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	hasNext := len(keys) > apiKeysPageSize
	if hasNext {
		keys = keys[:apiKeysPageSize]
	}
	type keyRow struct {
		ID         int64
		Name       string
		KeyPreview string
		KeyType    string
		IsActive   bool
		MachineID  string
		ServerName string
		LastUsed   any
	}
	var rows []keyRow
	for _, k := range keys {
		rows = append(rows, keyRow{
			ID:         k.ID,
			Name:       k.Name,
			KeyPreview: service.KeyPreview(k.Key),
			KeyType:    k.KeyType,
			IsActive:   k.IsActive,
			MachineID:  k.MachineID,
			ServerName: k.ServerName,
			LastUsed:   k.LastUsed,
		})
	}
	h.tmpl.Render(w, r, "api_keys.html", map[string]any{
		"Username":           username,
		"Role":               role,
		"Keys":               rows,
		"Flash":              r.URL.Query().Get("flash"),
		"SearchQuery":        query,
		"SearchQueryEscaped": url.QueryEscape(query),
		"KeysPage":           page,
		"KeysPrevPage":       page - 1,
		"KeysNextPage":       page + 1,
		"KeysHasPrev":        page > 1,
		"KeysHasNext":        hasNext,
		"UserTOTPEnabled":    user.TOTPEnabled,
	})
}

func (h *WebH) NewAPIKeyPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	h.tmpl.Render(w, r, "api_key_new.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) CreateAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/api-keys")
	name := strings.TrimSpace(r.FormValue("name"))
	serverName := strings.TrimSpace(r.FormValue("server_name"))
	if serverName == "" {
		redirectWithFlash(w, r, back, "Hostname del servidor requerido", false)
		return
	}
	if name == "" {
		name = serverName
	}
	serverURL := r.FormValue("server_url")
	githubToken := strings.TrimSpace(r.FormValue("github_token"))
	configuredURL := ""
	if cfg, cfgErr := h.store.GetEmailConfig(ctx); cfgErr == nil && cfg != nil {
		configuredURL = cfg.PublicAPIURL
	}
	apiURL, err := installerAPIURL(r, configuredURL)
	if err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
		return
	}
	k, err := h.store.CreateAPIKey(ctx, name, serverName, serverURL)
	if err != nil {
		slog.Error("create api key", "err", err)
		redirectWithFlash(w, r, back, "No se pudo crear la clave", false)
		return
	}
	h.audit(r, "api_key.create", "api_key", strconv.FormatInt(k.ID, 10), k.Name, map[string]any{
		"server_name": k.ServerName,
		"server_url":  k.ServerURL,
		"key_preview": service.KeyPreview(k.Key),
	})
	username, role, _ := session.GetUser(r)
	h.tmpl.Render(w, r, "api_key_created.html", map[string]any{
		"Username":    username,
		"Role":        role,
		"Key":         k.Key,
		"Name":        k.Name,
		"APIURL":      apiURL,
		"GitHubToken": githubToken,
	})
}

func (h *WebH) ToggleAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, "/api-keys", "No se encontró la clave", false)
		return
	}
	if err := h.store.ToggleAPIKey(ctx, id); err != nil {
		slog.Error("toggle api key", "id", id, "err", err)
		redirectWithFlash(w, r, "/api-keys", "No se pudo cambiar el estado de la clave", false)
		return
	}
	h.audit(r, "api_key.toggle", "api_key", strconv.FormatInt(id, 10), k.Name, map[string]any{"was_active": k.IsActive, "new_active": !k.IsActive})
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

func (h *WebH) DeleteAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, "/api-keys", "No se encontró la clave", false)
		return
	}
	if k.ServerName != "" {
		if err := h.store.HardDeleteServerDataForAPIKey(ctx, id, k.ServerName); err != nil {
			slog.Error("hard delete server data for api key", "id", id, "err", err)
			redirectWithFlash(w, r, "/api-keys", "Error al borrar los datos del servidor", false)
			return
		}
		h.audit(r, "server.hard_delete", "server", "", k.ServerName, map[string]any{"api_key_id": id})
	}
	if err := h.store.DeleteAPIKey(ctx, id); err != nil {
		slog.Error("delete api key", "id", id, "err", err)
		redirectWithFlash(w, r, "/api-keys", "No se pudo borrar la clave", false)
		return
	}
	h.audit(r, "api_key.delete", "api_key", strconv.FormatInt(id, 10), k.Name, map[string]any{"server_name": k.ServerName})
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

func (h *WebH) UnbindAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		redirectWithFlash(w, r, "/api-keys", "ID de clave inválido", false)
		return
	}
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			redirectWithFlash(w, r, "/api-keys", "No se encontró la clave", false)
		} else {
			slog.Error("get api key for unbind", "id", id, "err", err)
			redirectWithFlash(w, r, "/api-keys", "No se pudo consultar la clave", false)
		}
		return
	}
	if err := h.store.UnbindAPIKeyServer(ctx, id); err != nil {
		slog.Error("unbind api key", "id", id, "err", err)
		redirectWithFlash(w, r, "/api-keys", "No se pudo desvincular la máquina", false)
		return
	}
	h.audit(r, "api_key.unbind", "api_key", strconv.FormatInt(id, 10), k.Name, map[string]any{
		"machine_id_was_set":  k.MachineID != "",
		"server_name_was_set": k.ServerName != "",
	})
	redirectWithFlash(w, r, "/api-keys", "Máquina desvinculada", true)
}

func (h *WebH) RevealAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, `{"error":"invalid id"}`, http.StatusBadRequest)
		return
	}
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(ctx, username)
	if err != nil || !user.IsActive {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Usuario no válido"})
		return
	}
	if ok, blocked := h.checkCurrentPassword(r, user, r.FormValue("password")); !ok {
		status := http.StatusUnauthorized
		if blocked {
			status = http.StatusTooManyRequests
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": currentPasswordFailureMessage(blocked)})
		return
	}
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Key no encontrada"})
		return
	}
	h.audit(r, "api_key.reveal", "api_key", strconv.FormatInt(id, 10), k.Name, map[string]any{"key_preview": service.KeyPreview(k.Key)})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"key": k.Key})
}

func (h *WebH) EditAPIKeyPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		h.NotFound(w, r)
		return
	}
	h.tmpl.Render(w, r, "api_key_edit.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Key":      k,
		"Flash":    r.URL.Query().Get("flash"),
	})
}

func (h *WebH) EditAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	name := r.FormValue("name")
	name = strings.TrimSpace(name)
	serverName := strings.TrimSpace(r.FormValue("server_name"))
	serverURL := strings.TrimSpace(r.FormValue("server_url"))
	if serverName == "" {
		http.Redirect(w, r, "/api-keys/"+chi.URLParam(r, "id")+"/edit?flash=El+hostname+del+servidor+es+obligatorio", http.StatusSeeOther)
		return
	}
	if name == "" {
		name = serverName
	}
	if err := h.store.UpdateAPIKey(ctx, id, name, serverName, serverURL); err != nil {
		slog.Error("update api key", "id", id, "err", err)
		redirectWithFlash(w, r, "/api-keys/"+strconv.FormatInt(id, 10)+"/edit", "No se pudo guardar la clave", false)
		return
	}
	h.audit(r, "api_key.update", "api_key", strconv.FormatInt(id, 10), name, map[string]any{"server_name": serverName, "server_url": serverURL})
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

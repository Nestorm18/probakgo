package webhandlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/service"
	"probakgo/internal/session"
)

func (h *WebH) APIKeys(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	keys, err := h.store.ListAPIKeys(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
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
		"Username": username,
		"Role":     role,
		"Keys":     rows,
		"Flash":    r.URL.Query().Get("flash"),
	})
}

func (h *WebH) CreateAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	name := r.FormValue("name")
	serverName := r.FormValue("server_name")
	if name == "" {
		http.Redirect(w, r, "/api-keys?flash=Nombre+requerido", http.StatusSeeOther)
		return
	}
	serverURL := r.FormValue("server_url")
	k, err := h.store.CreateAPIKey(ctx, name, serverName, serverURL)
	if err != nil {
		http.Redirect(w, r, "/api-keys?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	scheme := "http"
	if r.TLS != nil {
		scheme = "https"
	}
	username, _, _ := session.GetUser(r)
	h.tmpl.Render(w, r, "api_key_created.html", map[string]any{
		"Username": username,
		"Key":      k.Key,
		"Name":     k.Name,
		"APIURL":   scheme + "://" + r.Host,
	})
}

func (h *WebH) ToggleAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_ = h.store.ToggleAPIKey(ctx, id)
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

func (h *WebH) DeleteAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	k, err := h.store.GetAPIKey(ctx, id)
	if err == nil && k.ServerName != "" {
		if err := h.store.HardDeleteServerData(ctx, k.ServerName); err != nil {
			http.Redirect(w, r, "/api-keys?flash=Error+al+borrar+servidor:+"+err.Error(), http.StatusSeeOther)
			return
		}
	}
	_ = h.store.DeleteAPIKey(ctx, id)
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

func (h *WebH) UnbindAPIKeyPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_ = h.store.UnbindAPIKeyMachineID(ctx, id)
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
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
	password := r.FormValue("password")
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Contraseña incorrecta"})
		return
	}
	k, err := h.store.GetAPIKey(ctx, id)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "Key no encontrada"})
		return
	}
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
		http.NotFound(w, r)
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
	serverName := r.FormValue("server_name")
	serverURL := r.FormValue("server_url")
	if name == "" {
		http.Redirect(w, r, "/api-keys/"+chi.URLParam(r, "id")+"/edit?flash=El+nombre+es+obligatorio", http.StatusSeeOther)
		return
	}
	if err := h.store.UpdateAPIKey(ctx, id, name, serverName, serverURL); err != nil {
		http.Redirect(w, r, "/api-keys/"+chi.URLParam(r, "id")+"/edit?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/api-keys", http.StatusSeeOther)
}

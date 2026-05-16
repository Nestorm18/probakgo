package webhandlers

import (
	"log/slog"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/session"
)

func (h *WebH) Users(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	users, err := h.store.ListUsers(ctx)
	if err != nil {
		slog.Error("list users", "err", err)
		http.Error(w, "error interno del servidor", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "users.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"CurrentUsername": username,
		"Users":           users,
		"Flash":           r.URL.Query().Get("flash"),
		"FlashOK":         r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) CreateUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	uname := r.FormValue("username")
	pass := r.FormValue("password")
	role := r.FormValue("role")
	if uname == "" || pass == "" {
		http.Redirect(w, r, "/users?flash=Usuario+y+contraseña+requeridos", http.StatusSeeOther)
		return
	}
	if role != "admin" && role != "editor" && role != "reader" {
		role = "reader"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	id, err := h.store.CreateUser(ctx, uname, string(hash), role)
	if err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	h.audit(r, "user.create", "user", strconv.FormatInt(id, 10), uname, map[string]any{"role": role})
	http.Redirect(w, r, "/users?flash=Usuario+creado&ok=1", http.StatusSeeOther)
}

func (h *WebH) ChangeUsernamePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	newUsername := r.FormValue("username")
	if newUsername == "" {
		http.Redirect(w, r, "/users?flash=Nombre+de+usuario+requerido", http.StatusSeeOther)
		return
	}
	curUsername, _, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	if err := h.store.UpdateUserUsername(ctx, id, newUsername); err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	h.audit(r, "user.rename", "user", strconv.FormatInt(id, 10), newUsername, map[string]any{"old_username": u.Username, "new_username": newUsername})
	// If changing own username, logout
	if u.Username == curUsername {
		session.Clear(w, r)
		http.Redirect(w, r, "/login?flash=Nombre+de+usuario+cambió+a+"+newUsername+"+-+inicia+sesión+de+nuevo", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/users?flash=Nombre+de+usuario+cambió+a+"+newUsername+"&ok=1", http.StatusSeeOther)
}

func (h *WebH) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pass := r.FormValue("password")
	confirm := r.FormValue("password_confirm")
	if pass == "" || confirm == "" {
		http.Redirect(w, r, "/users?flash=Contraseña+y+confirmación+requeridas", http.StatusSeeOther)
		return
	}
	if pass != confirm {
		http.Redirect(w, r, "/users?flash=Las+contraseñas+no+coinciden", http.StatusSeeOther)
		return
	}
	curUsername, _, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	_ = h.store.UpdateUserPassword(ctx, id, string(hash))
	h.audit(r, "user.password_change", "user", strconv.FormatInt(id, 10), u.Username, nil)
	// If changing own password, logout
	if u.Username == curUsername {
		session.Clear(w, r)
		http.Redirect(w, r, "/login?flash=Contraseña+actualizada+-+inicia+sesión+de+nuevo", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/users?flash=Contraseña+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) ChangeRolePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	role := r.FormValue("role")
	if role != "admin" && role != "editor" && role != "reader" {
		http.Redirect(w, r, "/users?flash=Rol+no+válido", http.StatusSeeOther)
		return
	}
	curUsername, sessionRole, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	// Prevent admin from changing their own role
	if u.Username == curUsername && sessionRole == "admin" {
		http.Redirect(w, r, "/users?flash=No+puedes+cambiar+tu+propio+rol", http.StatusSeeOther)
		return
	}
	_ = h.store.UpdateUserRole(ctx, id, role)
	h.audit(r, "user.role_change", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"old_role": u.Role, "new_role": role})
	http.Redirect(w, r, "/users?flash=Rol+actualizado&ok=1", http.StatusSeeOther)
}

func (h *WebH) ToggleUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	curUsername, sessionRole, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	// Prevent admin from toggling their own status
	if u.Username == curUsername && sessionRole == "admin" {
		http.Redirect(w, r, "/users?flash=No+puedes+desactivarte+a+ti+mismo", http.StatusSeeOther)
		return
	}
	_ = h.store.ToggleUser(ctx, id)
	h.audit(r, "user.toggle", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"was_active": u.IsActive, "new_active": !u.IsActive})
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (h *WebH) DeleteUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	if u.Role == "admin" {
		http.Redirect(w, r, "/users?flash=No+se+puede+eliminar+un+usuario+admin", http.StatusSeeOther)
		return
	}
	_ = h.store.DeleteUser(ctx, id)
	h.audit(r, "user.delete", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"role": u.Role})
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

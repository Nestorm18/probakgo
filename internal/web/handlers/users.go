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

func (h *WebH) UserNewPage(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	h.tmpl.Render(w, r, "user_new.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) UserEditPage(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	username, role, _ := session.GetUser(r)
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		http.Error(w, "invalid id", http.StatusBadRequest)
		return
	}
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		http.NotFound(w, r)
		return
	}
	h.tmpl.Render(w, r, "user_edit.html", map[string]any{
		"Username":        username,
		"Role":            role,
		"CurrentUsername": username,
		"User":            u,
		"Flash":           r.URL.Query().Get("flash"),
		"FlashOK":         r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) CreateUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	uname := r.FormValue("username")
	pass := r.FormValue("password")
	role := r.FormValue("role")
	if uname == "" || pass == "" {
		redirectWithFlash(w, r, back, "Usuario y contraseña requeridos", false)
		return
	}
	if role != "admin" && role != "editor" && role != "reader" {
		role = "reader"
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
		return
	}
	id, err := h.store.CreateUser(ctx, uname, string(hash), role)
	if err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
		return
	}
	h.audit(r, "user.create", "user", strconv.FormatInt(id, 10), uname, map[string]any{"role": role})
	redirectWithFlash(w, r, "/users", "Usuario creado", true)
}

func (h *WebH) ChangeUsernamePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	newUsername := r.FormValue("username")
	if newUsername == "" {
		redirectWithFlash(w, r, back, "Nombre de usuario requerido", false)
		return
	}
	curUsername, _, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	if err := h.store.UpdateUserUsername(ctx, id, newUsername); err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
		return
	}
	h.audit(r, "user.rename", "user", strconv.FormatInt(id, 10), newUsername, map[string]any{"old_username": u.Username, "new_username": newUsername})
	// If changing own username, logout
	if u.Username == curUsername {
		session.Clear(w, r)
		http.Redirect(w, r, "/login?flash=Nombre+de+usuario+cambió+a+"+newUsername+"+-+inicia+sesión+de+nuevo", http.StatusSeeOther)
		return
	}
	redirectWithFlash(w, r, back, "Nombre de usuario cambió a "+newUsername, true)
}

func (h *WebH) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pass := r.FormValue("password")
	confirm := r.FormValue("password_confirm")
	if pass == "" || confirm == "" {
		redirectWithFlash(w, r, back, "Contraseña y confirmación requeridas", false)
		return
	}
	if pass != confirm {
		redirectWithFlash(w, r, back, "Las contraseñas no coinciden", false)
		return
	}
	curUsername, _, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
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
	redirectWithFlash(w, r, back, "Contraseña actualizada", true)
}

func (h *WebH) DisableUser2FAPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	if err := h.store.DisableUserTOTP(ctx, id); err != nil {
		redirectWithFlash(w, r, back, err.Error(), false)
		return
	}
	h.audit(r, "user.2fa_admin_disable", "user", strconv.FormatInt(id, 10), u.Username, nil)
	redirectWithFlash(w, r, back, "2FA desactivado para "+u.Username, true)
}

func (h *WebH) ChangeRolePost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	role := r.FormValue("role")
	if role != "admin" && role != "editor" && role != "reader" {
		redirectWithFlash(w, r, back, "Rol no válido", false)
		return
	}
	curUsername, sessionRole, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	// Prevent admin from changing their own role
	if u.Username == curUsername && sessionRole == "admin" {
		redirectWithFlash(w, r, back, "No puedes cambiar tu propio rol", false)
		return
	}
	_ = h.store.UpdateUserRole(ctx, id, role)
	h.audit(r, "user.role_change", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"old_role": u.Role, "new_role": role})
	redirectWithFlash(w, r, back, "Rol actualizado", true)
}

func (h *WebH) ToggleUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	curUsername, sessionRole, _ := session.GetUser(r)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	// Prevent admin from toggling their own status
	if u.Username == curUsername && sessionRole == "admin" {
		redirectWithFlash(w, r, back, "No puedes desactivarte a ti mismo", false)
		return
	}
	_ = h.store.ToggleUser(ctx, id)
	h.audit(r, "user.toggle", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"was_active": u.IsActive, "new_active": !u.IsActive})
	redirectWithFlash(w, r, back, "Estado actualizado", true)
}

func (h *WebH) DeleteUserPost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	back := formBackOrDefault(r, "/users")
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	u, err := h.store.GetUser(ctx, id)
	if err != nil {
		redirectWithFlash(w, r, back, "Usuario no encontrado", false)
		return
	}
	if u.Role == "admin" {
		redirectWithFlash(w, r, back, "No se puede eliminar un usuario admin", false)
		return
	}
	_ = h.store.DeleteUser(ctx, id)
	h.audit(r, "user.delete", "user", strconv.FormatInt(id, 10), u.Username, map[string]any{"role": u.Role})
	redirectWithFlash(w, r, "/users", "Usuario eliminado", true)
}

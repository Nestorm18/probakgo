package webhandlers

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/session"
)

func (h *WebH) Users(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	users, err := h.store.ListUsers()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, "users.html", map[string]any{
		"Username": username,
		"Role":     role,
		"Users":    users,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) CreateUserPost(w http.ResponseWriter, r *http.Request) {
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
	if _, err := h.store.CreateUser(uname, string(hash), role); err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/users?flash=Usuario+creado&ok=1", http.StatusSeeOther)
}

func (h *WebH) ChangePasswordPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	pass := r.FormValue("password")
	if pass == "" {
		http.Redirect(w, r, "/users?flash=Contraseña+requerida", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pass), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/users?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	_ = h.store.UpdateUserPassword(id, string(hash))
	http.Redirect(w, r, "/users?flash=Contraseña+actualizada&ok=1", http.StatusSeeOther)
}

func (h *WebH) ChangeRolePost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	role := r.FormValue("role")
	if role != "admin" && role != "editor" && role != "reader" {
		http.Redirect(w, r, "/users?flash=Rol+no+válido", http.StatusSeeOther)
		return
	}
	// Prevent self-demotion
	_, sessionRole, _ := session.GetUser(r)
	u, err := h.store.GetUser(id)
	if err != nil {
		http.Redirect(w, r, "/users?flash=Usuario+no+encontrado", http.StatusSeeOther)
		return
	}
	curUsername, _, _ := session.GetUser(r)
	if u.Username == curUsername && sessionRole == "admin" && role != "admin" {
		http.Redirect(w, r, "/users?flash=No+puedes+cambiar+tu+propio+rol+de+admin", http.StatusSeeOther)
		return
	}
	_ = h.store.UpdateUserRole(id, role)
	http.Redirect(w, r, "/users?flash=Rol+actualizado&ok=1", http.StatusSeeOther)
}

func (h *WebH) ToggleUserPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_ = h.store.ToggleUser(id)
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (h *WebH) DeleteUserPost(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	_ = h.store.DeleteUser(id)
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

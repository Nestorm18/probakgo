package webhandlers

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/session"
)

func (h *WebH) Profile(w http.ResponseWriter, r *http.Request) {
	username, role, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}
	h.tmpl.Render(w, r, "profile.html", map[string]any{
		"Username": username,
		"Role":     role,
		"User":     user,
		"Flash":    r.URL.Query().Get("flash"),
		"FlashOK":  r.URL.Query().Get("ok") == "1",
	})
}

func (h *WebH) ProfilePost(w http.ResponseWriter, r *http.Request) {
	username, _, _ := session.GetUser(r)
	user, err := h.store.GetUserByUsername(username)
	if err != nil {
		http.Error(w, "user not found", http.StatusInternalServerError)
		return
	}

	currentPass := r.FormValue("current_password")
	newPass := r.FormValue("new_password")
	confirm := r.FormValue("password_confirm")

	if newPass == "" {
		http.Redirect(w, r, "/profile?flash=La+nueva+contrasena+no+puede+estar+vacia", http.StatusSeeOther)
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(currentPass)) != nil {
		http.Redirect(w, r, "/profile?flash=Contrasena+actual+incorrecta", http.StatusSeeOther)
		return
	}
	if newPass != confirm {
		http.Redirect(w, r, "/profile?flash=Las+nuevas+contrasenas+no+coinciden", http.StatusSeeOther)
		return
	}
	if len(newPass) < 6 {
		http.Redirect(w, r, "/profile?flash=La+contrasena+debe+tener+al+menos+6+caracteres", http.StatusSeeOther)
		return
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(newPass), bcrypt.DefaultCost)
	if err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	if err := h.store.UpdateUserPassword(user.ID, string(hash)); err != nil {
		http.Redirect(w, r, "/profile?flash="+err.Error(), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/profile?flash=Contrasena+actualizada&ok=1", http.StatusSeeOther)
}

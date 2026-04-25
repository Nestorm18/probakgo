package webhandlers

import (
	"net/http"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/service"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

type WebH struct {
	store  *store.Store
	tmpl   *Templates
	report *service.ReportService
}

func New(st *store.Store, tmpl *Templates, rep *service.ReportService) *WebH {
	return &WebH{store: st, tmpl: tmpl, report: rep}
}

func (h *WebH) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := session.GetUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, "login.html", map[string]any{"Error": ""})
}

func (h *WebH) LoginPost(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.store.GetUserByUsername(username)
	if err != nil || !user.IsActive {
		h.tmpl.Render(w, "login.html", map[string]any{"Error": "Invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		h.tmpl.Render(w, "login.html", map[string]any{"Error": "Invalid credentials"})
		return
	}
	if err := session.SetUser(w, r, user.Username, user.Role); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	next := r.URL.Query().Get("next")
	if next == "" || next == "/login" {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *WebH) Logout(w http.ResponseWriter, r *http.Request) {
	session.Clear(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

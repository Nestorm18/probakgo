package webhandlers

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/ratelimit"
	"probakgo/internal/service"
	"probakgo/internal/session"
	"probakgo/internal/store"
)

type WebH struct {
	store  *store.Store
	tmpl   *Templates
	report *service.ReportService
	ban    *ratelimit.Banhammer
}

func New(st *store.Store, tmpl *Templates, rep *service.ReportService) *WebH {
	return &WebH{store: st, tmpl: tmpl, report: rep}
}

func (h *WebH) SetBanhammer(b *ratelimit.Banhammer) {
	h.ban = b
}

func (h *WebH) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := session.GetUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	flash := r.URL.Query().Get("flash")
	h.tmpl.Render(w, r, "login.html", map[string]any{"Error": flash})
}

func (h *WebH) LoginPost(w http.ResponseWriter, r *http.Request) {
	ip := ratelimit.ExtractIP(r)

	if h.ban != nil {
		if banned, remaining := h.ban.IsBanned(ip); banned {
			w.WriteHeader(http.StatusTooManyRequests)
			h.tmpl.Render(w, r, "login.html", map[string]any{
				"Error": fmt.Sprintf("Too many failed attempts. Try again in %s.", formatRemaining(remaining)),
			})
			return
		}
	}

	username := r.FormValue("username")
	password := r.FormValue("password")

	user, err := h.store.GetUserByUsername(username)
	if err != nil || !user.IsActive {
		if h.ban != nil {
			h.ban.RecordFailure(ip)
		}
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Invalid credentials"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		if h.ban != nil {
			h.ban.RecordFailure(ip)
		}
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Invalid credentials"})
		return
	}

	if h.ban != nil {
		h.ban.ClearFailures(ip)
	}

	if err := session.SetUser(w, r, user.Username, user.Role); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	next := r.URL.Query().Get("next")
	if len(next) == 0 || next[0] != '/' || (len(next) > 1 && next[1] == '/') || next == "/login" {
		next = "/"
	}
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *WebH) Logout(w http.ResponseWriter, r *http.Request) {
	session.Clear(w, r)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func formatRemaining(d time.Duration) string {
	if d < 0 {
		return "permanently"
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%d day(s)", int(d.Hours()/24)+1)
	case d >= time.Hour:
		return fmt.Sprintf("%d hour(s)", int(d.Hours())+1)
	default:
		return fmt.Sprintf("%d minute(s)", int(d.Minutes())+1)
	}
}

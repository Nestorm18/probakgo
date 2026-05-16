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
	store     *store.Store
	tmpl      *Templates
	report    *service.ReportService
	ban       *ratelimit.Banhammer
	startTime time.Time
}

func New(st *store.Store, tmpl *Templates, rep *service.ReportService) *WebH {
	return &WebH{store: st, tmpl: tmpl, report: rep, startTime: time.Now()}
}

func (h *WebH) SetBanhammer(b *ratelimit.Banhammer) {
	h.ban = b
}

func (h *WebH) LoginPage(w http.ResponseWriter, r *http.Request) {
	if _, _, ok := session.GetUser(r); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	if h.ban != nil {
		if banned, remaining := h.ban.IsBanned(ratelimit.ExtractIP(r)); banned {
			h.tmpl.Render(w, r, "login.html", map[string]any{
				"Error": fmt.Sprintf("Demasiados intentos fallidos. Inténtalo de nuevo en %s.", formatRemaining(remaining)),
			})
			return
		}
	}
	flash := r.URL.Query().Get("flash")
	h.tmpl.Render(w, r, "login.html", map[string]any{"Error": flash})
}

func (h *WebH) LoginPost(w http.ResponseWriter, r *http.Request) {
	ip := ratelimit.ExtractIP(r)
	username := r.FormValue("username")
	userAgent := r.UserAgent()

	if h.ban != nil {
		if banned, _ := h.ban.IsBanned(ip); banned {
			_ = h.store.InsertLoginAttempt(r.Context(), username, ip, userAgent, "blocked", "ip_banned")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}

	password := r.FormValue("password")

	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil || !user.IsActive {
		_ = h.store.InsertLoginAttempt(r.Context(), username, ip, userAgent, "failed", "invalid_credentials")
		if h.ban != nil {
			h.ban.RecordFailure(ip)
		}
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Usuario o contraseña incorrectos"})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		_ = h.store.InsertLoginAttempt(r.Context(), username, ip, userAgent, "failed", "invalid_credentials")
		if h.ban != nil {
			h.ban.RecordFailure(ip)
		}
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Usuario o contraseña incorrectos"})
		return
	}

	if h.ban != nil {
		h.ban.ClearFailures(ip)
	}

	if err := session.SetUser(w, r, user.Username, user.Role); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ip, userAgent, "success", "")
	_ = h.store.UpdateUserLastLogin(r.Context(), user.ID, ratelimit.ExtractIP(r))
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
		return "permanentemente"
	}
	switch {
	case d >= 24*time.Hour:
		days := int(d.Hours()/24) + 1
		if days == 1 {
			return "1 día"
		}
		return fmt.Sprintf("%d días", days)
	case d >= time.Hour:
		hours := int(d.Hours()) + 1
		if hours == 1 {
			return "1 hora"
		}
		return fmt.Sprintf("%d horas", hours)
	default:
		mins := int(d.Minutes()) + 1
		if mins == 1 {
			return "1 minuto"
		}
		return fmt.Sprintf("%d minutos", mins)
	}
}

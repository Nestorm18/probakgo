package webhandlers

import (
	"fmt"
	"net/http"
	"time"

	"golang.org/x/crypto/bcrypt"

	"probakgo/internal/domain"
	"probakgo/internal/ratelimit"
	"probakgo/internal/service"
	"probakgo/internal/session"
	"probakgo/internal/store"
	"probakgo/internal/totp"
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

	next := safeNext(r.URL.Query().Get("next"))
	if redirect, ok := h.handleTOTPEnforcement(w, r, user); ok {
		if redirect == "" {
			return
		}
		next = redirect
	}

	if h.ban != nil {
		if !user.TOTPEnabled {
			h.ban.ClearFailures(ip)
		}
	}

	if user.TOTPEnabled {
		if err := session.SetPending2FA(w, r, user.ID, next); err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}
		_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ip, userAgent, "pending", "totp_required")
		http.Redirect(w, r, "/login/2fa", http.StatusSeeOther)
		return
	}

	if err := session.SetUser(w, r, user.Username, user.Role); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ip, userAgent, "success", "")
	_ = h.store.UpdateUserLastLogin(r.Context(), user.ID, ratelimit.ExtractIP(r))
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *WebH) Login2FAPage(w http.ResponseWriter, r *http.Request) {
	userID, _, ok := session.GetPending2FA(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil || !user.TOTPEnabled {
		_ = session.ClearPending2FA(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	h.tmpl.Render(w, r, "login_2fa.html", map[string]any{
		"Username": user.Username,
		"Error":    r.URL.Query().Get("flash"),
	})
}

func (h *WebH) Login2FAPost(w http.ResponseWriter, r *http.Request) {
	ip := ratelimit.ExtractIP(r)
	userAgent := r.UserAgent()
	userID, next, ok := session.GetPending2FA(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil || !user.IsActive || !user.TOTPEnabled {
		_ = session.ClearPending2FA(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if h.ban != nil {
		if banned, _ := h.ban.IsBanned(ip); banned {
			_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ip, userAgent, "blocked", "ip_banned")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	if !totp.Validate(r.FormValue("code"), user.TOTPSecret, time.Now()) {
		_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ip, userAgent, "failed", "invalid_totp")
		if h.ban != nil {
			h.ban.RecordFailure(ip)
		}
		h.tmpl.Render(w, r, "login_2fa.html", map[string]any{
			"Username": user.Username,
			"Error":    "Codigo 2FA incorrecto",
		})
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
	_ = h.store.UpdateUserLastLogin(r.Context(), user.ID, ip)
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
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

func safeNext(next string) string {
	if len(next) == 0 || next[0] != '/' || (len(next) > 1 && next[1] == '/') || next == "/login" || next == "/login/2fa" {
		return "/"
	}
	return next
}

func (h *WebH) handleTOTPEnforcement(w http.ResponseWriter, r *http.Request, user *domain.User) (string, bool) {
	if user.Role == "reader" || user.TOTPEnabled {
		return "", false
	}
	cfg, err := h.store.GetEmailConfig(r.Context())
	if err != nil || cfg == nil || !cfg.EnforceTOTPNonReaders {
		return "", false
	}

	now := time.Now()
	startedAt := user.TOTPGraceStartedAt
	if startedAt == nil {
		if err := h.store.StartUserTOTPGrace(r.Context(), user.ID); err != nil {
			http.Error(w, "error interno del servidor", http.StatusInternalServerError)
			return "", true
		}
		user.TOTPGraceStartedAt = &now
		startedAt = &now
	}
	if now.Sub(*startedAt) >= 72*time.Hour {
		_ = h.store.SetUserActive(r.Context(), user.ID, false)
		_ = h.store.InsertLoginAttempt(r.Context(), user.Username, ratelimit.ExtractIP(r), r.UserAgent(), "blocked", "totp_grace_expired")
		h.tmpl.Render(w, r, "login.html", map[string]any{
			"Error": "Usuario desactivado: 2FA no se activo dentro del plazo de 3 dias.",
		})
		return "", true
	}
	return "/profile?flash=Activa+2FA+en+tu+usuario.+Tienes+3+dias+desde+el+primer+aviso.&ok=1", true
}

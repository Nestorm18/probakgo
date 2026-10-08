package webhandlers

import (
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
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
	store        *store.Store
	tmpl         *Templates
	report       *service.ReportService
	ban          *ratelimit.Banhammer
	telegram     adminSecurityNotifier
	loginNotices loginNoticeThrottle
	// passwordFailures limits current-password checks by signed-in users.
	passwordFailures *ratelimit.Limiter
	startTime        time.Time
}

func New(st *store.Store, tmpl *Templates, rep *service.ReportService) *WebH {
	h := &WebH{store: st, tmpl: tmpl, report: rep, passwordFailures: ratelimit.New(5, 15*time.Minute), startTime: time.Now()}
	if sender := service.GetTelegramSender(); sender != nil {
		h.telegram = sender
	}
	return h
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
	h.tmpl.Render(w, r, "login.html", map[string]any{"Error": flash, "Next": loginNext(r)})
}

func (h *WebH) LoginPost(w http.ResponseWriter, r *http.Request) {
	ip := ratelimit.ExtractIP(r)
	username := r.FormValue("username")
	userAgent := r.UserAgent()

	if h.ban != nil {
		if banned, _ := h.ban.IsBanned(ip); banned {
			// The ban itself was already notified; repeated attempts are only logged.
			h.recordLoginAttempt(r, username, ip, userAgent, "blocked", "ip_banned")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}

	password := r.FormValue("password")

	user, err := h.store.GetUserByUsername(r.Context(), username)
	if err != nil || !user.IsActive {
		// Spend the same bcrypt work so response time does not reveal accounts.
		_ = bcrypt.CompareHashAndPassword(dummyPasswordHash(), []byte(password))
		h.loginFailed(r, username, ip, userAgent, "invalid_credentials", "Credenciales no válidas")
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Usuario o contraseña incorrectos", "Next": loginNext(r)})
		return
	}
	if bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)) != nil {
		h.loginFailed(r, username, ip, userAgent, "invalid_credentials", "Credenciales no válidas")
		h.tmpl.Render(w, r, "login.html", map[string]any{"Error": "Usuario o contraseña incorrectos", "Next": loginNext(r)})
		return
	}

	next := safeNext(r.FormValue("next"))
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
		if err := session.SetPending2FA(w, r, user.ID, next, user.SessionVersion); err != nil {
			http.Error(w, "Session error", http.StatusInternalServerError)
			return
		}
		h.recordLoginAttempt(r, user.Username, ip, userAgent, "pending", "totp_required")
		http.Redirect(w, r, "/login/2fa", http.StatusSeeOther)
		return
	}

	if err := session.SetUserWithVersion(w, r, user.ID, user.Username, user.Role, user.SessionVersion); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	h.recordLoginAttempt(r, user.Username, ip, userAgent, "success", "")
	_ = h.store.UpdateUserLastLogin(r.Context(), user.ID, ratelimit.ExtractIP(r))
	h.notifyAdminLoginSuccess(user.Username, ip)
	http.Redirect(w, r, next, http.StatusSeeOther)
}

func (h *WebH) Login2FAPage(w http.ResponseWriter, r *http.Request) {
	userID, _, version, ok := session.GetPending2FA(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil || !user.IsActive || !user.TOTPEnabled || user.SessionVersion != version {
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
	userID, next, version, ok := session.GetPending2FA(r)
	if !ok {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	user, err := h.store.GetUser(r.Context(), userID)
	if err != nil || !user.IsActive || !user.TOTPEnabled || user.SessionVersion != version {
		_ = session.ClearPending2FA(w, r)
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if h.ban != nil {
		if banned, _ := h.ban.IsBanned(ip); banned {
			h.recordLoginAttempt(r, user.Username, ip, userAgent, "blocked", "ip_banned")
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
	}
	if !session.AllowPending2FAAttempt(r) {
		_ = session.ClearPending2FA(w, r)
		http.Error(w, "Demasiados intentos 2FA. Inicia sesion de nuevo.", http.StatusTooManyRequests)
		return
	}
	step, valid := totp.ValidateStep(r.FormValue("code"), user.TOTPSecret, time.Now())
	if valid {
		// A code works once: an observed or replayed code is rejected.
		claimed, err := h.store.ClaimUserTOTPStep(r.Context(), user.ID, step)
		if err != nil {
			http.Error(w, "error interno del servidor", http.StatusInternalServerError)
			return
		}
		valid = claimed
	}
	if !valid {
		h.loginFailed(r, user.Username, ip, userAgent, "invalid_totp", "Código 2FA incorrecto")
		h.tmpl.Render(w, r, "login_2fa.html", map[string]any{
			"Username": user.Username,
			"Error":    "Codigo 2FA incorrecto o ya utilizado",
		})
		return
	}
	if h.ban != nil {
		h.ban.ClearFailures(ip)
	}
	if !session.ConsumePending2FA(r, user.ID, user.SessionVersion) {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if err := session.SetUserWithVersion(w, r, user.ID, user.Username, user.Role, user.SessionVersion); err != nil {
		http.Error(w, "Session error", http.StatusInternalServerError)
		return
	}
	h.recordLoginAttempt(r, user.Username, ip, userAgent, "success", "")
	_ = h.store.UpdateUserLastLogin(r.Context(), user.ID, ip)
	h.notifyAdminLoginSuccess(user.Username, ip)
	http.Redirect(w, r, safeNext(next), http.StatusSeeOther)
}

// loginFailed records a failed attempt, escalates the IP ban and notifies
// administrators within the failed-login notification throttle.
func (h *WebH) loginFailed(r *http.Request, username, ip, userAgent, reason, notice string) {
	h.recordLoginAttempt(r, username, ip, userAgent, "failed", reason)
	banned := h.ban != nil && h.ban.RecordFailure(ip)
	if banned {
		notice += "; IP bloqueada"
	}
	h.notifyAdminLoginFailed(username, ip, notice, banned)
}

var dummyPasswordHash = sync.OnceValue(func() []byte {
	hash, err := bcrypt.GenerateFromPassword([]byte("probakgo-timing-equalizer"), bcrypt.DefaultCost)
	if err != nil {
		return nil
	}
	return hash
})

// Logout revokes the session ID server-side, so a copied cookie stops working
// too, and then clears the cookie.
func (h *WebH) Logout(w http.ResponseWriter, r *http.Request) {
	if sid, ok := session.ID(r); ok {
		if err := h.store.RevokeSession(r.Context(), sid, time.Now().Add(session.Lifetime)); err != nil {
			slog.Error("logout: revoke session", "err", err)
			http.Error(w, "Session store unavailable", http.StatusServiceUnavailable)
			return
		}
	}
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
	next = SafeLocalPath(next)
	if next == "" {
		return "/"
	}
	if u, err := url.Parse(next); err != nil || u.Path == "/login" || u.Path == "/login/2fa" || u.Path == "/logout" {
		return "/"
	}
	return next
}

// loginNext is the validated return path posted by the login form, or "" for
// the dashboard.
func loginNext(r *http.Request) string {
	if next := safeNext(r.FormValue("next")); next != "/" {
		return next
	}
	return ""
}

func (h *WebH) handleTOTPEnforcement(w http.ResponseWriter, r *http.Request, user *domain.User) (string, bool) {
	if user.Role == "reader" || user.TOTPEnabled {
		return "", false
	}
	cfg, err := h.store.GetEmailConfig(r.Context())
	if err != nil || cfg == nil {
		http.Error(w, "Security configuration unavailable", http.StatusServiceUnavailable)
		return "", true
	}
	if !cfg.EnforceTOTPNonReaders {
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
		// Never lock out the last active administrator: keep asking for 2FA.
		lastAdmin, err := h.store.IsLastActiveAdmin(r.Context(), user.ID)
		if err != nil {
			http.Error(w, "error interno del servidor", http.StatusInternalServerError)
			return "", true
		}
		if lastAdmin {
			return "/profile?flash=Plazo+de+2FA+vencido.+Eres+el+unico+administrador+activo:+activa+2FA+ahora.", true
		}
		_ = h.store.SetUserActive(r.Context(), user.ID, false)
		h.recordLoginAttempt(r, user.Username, ratelimit.ExtractIP(r), r.UserAgent(), "blocked", "totp_grace_expired")
		h.notifyAdminLoginFailed(user.Username, ratelimit.ExtractIP(r), "Usuario desactivado por no configurar 2FA", false)
		h.tmpl.Render(w, r, "login.html", map[string]any{
			"Error": "Usuario desactivado: 2FA no se activo dentro del plazo de 3 dias.",
		})
		return "", true
	}
	return "/profile?flash=Activa+2FA+en+tu+usuario.+Tienes+3+dias+desde+el+primer+aviso.&ok=1", true
}
